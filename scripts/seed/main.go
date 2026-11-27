package main

import (
	"github.com/jackc/pgx/v5"

	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/fiveace-Merill/photobooth/internal/auth"
	authstore "github.com/fiveace-Merill/photobooth/internal/auth/store"
	"github.com/fiveace-Merill/photobooth/internal/clients"
	"github.com/fiveace-Merill/photobooth/internal/platform/database"
	"github.com/fiveace-Merill/photobooth/internal/shoots"
	"github.com/jackc/pgx/v5/pgtype"
)

func main() {
	if os.Getenv("APP_ENV") == "production" {
		log.Fatal("refusing to seed when APP_ENV=production")
	}
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	ctx := context.Background()
	pool, err := database.Open(ctx, dbURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	email := envDefault("SEED_PHOTOGRAPHER_EMAIL", "demo@photobooth.example")
	password := envDefault("SEED_PHOTOGRAPHER_PASSWORD", "password1234")
	biz := envDefault("SEED_PHOTOGRAPHER_BUSINESS_NAME", "Demo Studio")
	slug := envDefault("SEED_PHOTOGRAPHER_SLUG", "demo-studio")

	authSvc := auth.NewService(authstore.New(pool), pool, "open")
	photog, _, err := authSvc.Signup(ctx, auth.SignupInput{Email: email, Password: password, BusinessName: biz, Slug: slug})
	if err != nil {
		p, _, lerr := authSvc.Login(ctx, email, password)
		if lerr != nil {
			log.Fatalf("signup failed: %v; login failed: %v", err, lerr)
		}
		photog = p
	}
	pid := photog.ID
	fmt.Println("photographer:", uuidString(pid), photog.Email)

	clientsSvc := clients.NewService(pool)
	shootsSvc, err := shoots.NewService(pool, clientsSvc)
	if err != nil {
		log.Fatal(err)
	}

	clientIDs := make([]pgtype.UUID, 0, 15)
	for i := 1; i <= 15; i++ {
		phone := fmt.Sprintf("0700 000 %03d", i)
		email := fmt.Sprintf("client%02d@example.com", i)
		name := fakeNames[(i-1)%len(fakeNames)]
		c, err := clientsSvc.Create(ctx, pid, clients.CreateInput{Name: name, Phone: &phone, Email: &email})
		if err != nil {
			var existing pgtype.UUID
			qerr := database.WithTenant(ctx, pool, pid, func(tx pgx.Tx) error {
				return tx.QueryRow(ctx, `SELECT id FROM clients WHERE photographer_id=$1 AND phone=$2`, pid, fmt.Sprintf("254700000%03d", i)).Scan(&existing)
			})
			if qerr == nil {
				clientIDs = append(clientIDs, existing)
				continue
			}
			log.Fatalf("create client %d: %v", i, err)
		}
		clientIDs = append(clientIDs, c.ID)
	}
	fmt.Println("clients:", len(clientIDs))

	var shootCount int
	_ = database.WithTenant(ctx, pool, pid, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT COUNT(*) FROM shoots WHERE photographer_id=$1`, pid).Scan(&shootCount)
	})
	if shootCount >= 30 {
		fmt.Println("shoots: already seeded")
		return
	}

	today := time.Now()
	statuses := []string{"shot", "editing", "ready", "delivered", "expired"}
	for i := 0; i < 30; i++ {
		cid := clientIDs[i%len(clientIDs)]
		title := fmt.Sprintf("Shoot %02d", i+1)
		sd := today.AddDate(0, 0, -i)
		var dd *time.Time
		if i%3 == 0 {
			d := sd.AddDate(0, 0, 3)
			dd = &d
		}
		if i%5 == 0 {
			d := sd.AddDate(0, 0, 14)
			dd = &d
		}
		price := int64((i % 5) * 100000)
		sh, err := shootsSvc.Create(ctx, pid, shoots.CreateInput{ClientID: cid, Title: title, ShootDate: &sd, DueDate: dd, PriceMinor: price, Currency: "KES"})
		if err != nil {
			log.Fatalf("create shoot %d: %v", i, err)
		}
		status := statuses[i%len(statuses)]
		if status == "delivered" || status == "expired" {
			_, err = pool.Exec(ctx, `UPDATE shoots SET status=$1, updated_at=now() WHERE photographer_id=$2 AND id=$3`, status, pid, sh.ID)
			if err != nil {
				log.Fatalf("set status %d: %v", i, err)
			}
		} else if status != "shot" {
			if _, err := shootsSvc.SetStatus(ctx, pid, sh.ID, status); err != nil {
				log.Fatalf("status %d: %v", i, err)
			}
		}
	}
	fmt.Println("shoots: 30")
}

var fakeNames = []string{
	"Wanjiku Njeri", "Kamau Otieno", "Achieng Onyango", "Muthoni Kariuki", "Odhiambo Ouma",
	"Chebet Kiptoo", "Naserian Sankale", "Amina Hassan", "Fatuma Ali", "Nyambura Wambui",
	"Kiplangat Korir", "Wekesa Wafula", "Makena Mwende", "Atieno Adhiambo", "Mutiso Musyoka",
}

func envDefault(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
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
		out[pos+1] = hex[by&0x0f]
		pos += 2
	}
	return string(out)
}
