package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"

	"github.com/fiveace-Merill/photobooth/internal/admin"
	adminstore "github.com/fiveace-Merill/photobooth/internal/admin/store"
	"github.com/fiveace-Merill/photobooth/internal/auth"
	"github.com/fiveace-Merill/photobooth/internal/platform/config"
	"github.com/fiveace-Merill/photobooth/internal/platform/database"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func main() {
	if len(os.Args) < 2 {
		fatal("usage: admin <create|disable|reset-2fa> --email ...")
	}
	// Accept both "admin create ..." and "create ..." for Makefile flexibility.
	args := os.Args[1:]
	if args[0] == "admin" {
		args = args[1:]
	}
	if len(args) < 1 {
		fatal("usage: admin <create|disable|reset-2fa> --email ...")
	}
	switch args[0] {
	case "create":
		runCreate(args[1:])
	case "disable":
		runDisable(args[1:])
	case "reset-2fa":
		runReset2FA(args[1:])
	default:
		fatal("unknown command: " + args[0])
	}
}

func runCreate(args []string) {
	fs := flag.NewFlagSet("admin create", flag.ExitOnError)
	email := fs.String("email", "", "admin email")
	_ = fs.Parse(args)
	if strings.TrimSpace(*email) == "" {
		fatal("--email required")
	}
	cfg := mustLoadConfig()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		fatalErr(err)
	}
	defer pool.Close()

	pw1, err := readPassword("Password: ")
	if err != nil {
		fatalErr(err)
	}
	pw2, err := readPassword("Confirm password: ")
	if err != nil {
		fatalErr(err)
	}
	if pw1 != pw2 {
		fatal("passwords do not match")
	}
	if len(pw1) < 12 {
		fatal("password must be at least 12 characters")
	}
	hash, err := auth.HashPassword(pw1)
	if err != nil {
		fatalErr(err)
	}

	q := adminstore.New(pool)
	tx, err := pool.Begin(ctx)
	if err != nil {
		fatalErr(err)
	}
	defer tx.Rollback(ctx)
	qtx := q.WithTx(tx)
	row, err := qtx.CreateAdmin(ctx, adminstore.CreateAdminParams{Email: strings.ToLower(strings.TrimSpace(*email)), PasswordHash: hash})
	if err != nil {
		fatalErr(err)
	}
	meta := []byte(`{"actor":"cli","source":"cmd/admin"}`)
	if err := qtx.InsertAuditLog(ctx, adminstore.InsertAuditLogParams{AdminID: row.ID, Actor: "cli", Action: "admin.created", TargetType: "admin", TargetID: pgText(uuidString(row.ID)), Meta: meta, Ip: pgText("cli")}); err != nil {
		fatalErr(err)
	}
	if err := tx.Commit(ctx); err != nil {
		fatalErr(err)
	}
	fmt.Println("created admin", row.Email, uuidString(row.ID))
}

func runDisable(args []string) {
	fs := flag.NewFlagSet("admin disable", flag.ExitOnError)
	email := fs.String("email", "", "admin email")
	_ = fs.Parse(args)
	if strings.TrimSpace(*email) == "" {
		fatal("--email required")
	}
	cfg := mustLoadConfig()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		fatalErr(err)
	}
	defer pool.Close()
	q := adminstore.New(pool)
	tx, err := pool.Begin(ctx)
	if err != nil {
		fatalErr(err)
	}
	defer tx.Rollback(ctx)
	qtx := q.WithTx(tx)
	n, err := qtx.SetAdminDisabled(ctx, strings.ToLower(strings.TrimSpace(*email)))
	if err != nil {
		fatalErr(err)
	}
	if n == 0 {
		fatal("admin not found or already disabled")
	}
	a, err := qtx.GetAdminByEmail(ctx, strings.ToLower(strings.TrimSpace(*email)))
	if err != nil {
		fatalErr(err)
	}
	if err := qtx.InsertAuditLog(ctx, adminstore.InsertAuditLogParams{AdminID: a.ID, Actor: "cli", Action: "admin.disabled", TargetType: "admin", TargetID: pgText(uuidString(a.ID)), Meta: []byte(`{"actor":"cli"}`), Ip: pgText("cli")}); err != nil {
		fatalErr(err)
	}
	if err := tx.Commit(ctx); err != nil {
		fatalErr(err)
	}
	fmt.Println("disabled admin", *email)
}

func runReset2FA(args []string) {
	fs := flag.NewFlagSet("admin reset-2fa", flag.ExitOnError)
	email := fs.String("email", "", "admin email")
	_ = fs.Parse(args)
	if strings.TrimSpace(*email) == "" {
		fatal("--email required")
	}
	cfg := mustLoadConfig()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		fatalErr(err)
	}
	defer pool.Close()
	q := adminstore.New(pool)
	tx, err := pool.Begin(ctx)
	if err != nil {
		fatalErr(err)
	}
	defer tx.Rollback(ctx)
	qtx := q.WithTx(tx)
	norm := strings.ToLower(strings.TrimSpace(*email))
	a, err := qtx.GetAdminByEmail(ctx, norm)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			fatal("admin not found")
		}
		fatalErr(err)
	}
	if _, err := qtx.ClearAdminTOTP(ctx, norm); err != nil {
		fatalErr(err)
	}
	if err := qtx.DeleteRecoveryCodesByAdmin(ctx, a.ID); err != nil {
		fatalErr(err)
	}
	if err := qtx.InsertAuditLog(ctx, adminstore.InsertAuditLogParams{AdminID: a.ID, Actor: "cli", Action: "admin.2fa_reset", TargetType: "admin", TargetID: pgText(uuidString(a.ID)), Meta: []byte(`{"actor":"cli"}`), Ip: pgText("cli")}); err != nil {
		fatalErr(err)
	}
	if err := tx.Commit(ctx); err != nil {
		fatalErr(err)
	}
	fmt.Println("reset 2fa for", norm)
}

func mustLoadConfig() config.Config {
	cfg, err := config.Load()
	if err != nil {
		fatalErr(err)
	}
	return cfg
}

func readPassword(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	b, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func fatal(msg string)   { fmt.Fprintln(os.Stderr, msg); os.Exit(1) }
func fatalErr(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
func pgText(s string) pgtype.Text {
	if strings.TrimSpace(s) == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}
func uuidString(id pgtype.UUID) string {
	if !id.Valid {
		return ""
	}
	b := id.Bytes
	hex := "0123456789abcdef"
	out := make([]byte, 36)
	pos := 0
	for i, by := range b {
		if i == 4 || i == 6 || i == 8 || i == 10 {
			out[pos] = '-'
			pos++
		}
		out[pos] = hex[by>>4]
		pos++
		out[pos] = hex[by&0x0f]
		pos++
	}
	return string(out)
}

var _ = bufio.NewReader
var _ = admin.Service{}
