package notify

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

func TestLogSenderRefusedInProduction(t *testing.T) {
	_, err := NewLogSender(nil, "production")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSMTPSenderSuccess(t *testing.T) {
	addr, done := startFakeSMTP(t, map[string][]string{})
	host, port := splitHostPort(t, addr)
	s := NewSMTPSender(SMTPConfig{Host: host, Port: port, TLS: "none"})
	id, err := s.Send(context.Background(), Message{FromName: "Studio", FromAddr: "noreply@test.local", To: "client@example.com", Subject: "Hi", Text: "hello https://app.test/g/tok", HTML: "<p>hello</p>"})
	if err != nil {
		t.Fatalf("send: %v (%T)", err, err)
	}
	if strings.TrimSpace(id) == "" {
		t.Fatal("expected message id")
	}
	<-done
}

func TestSMTPSenderRecipientRejected(t *testing.T) {
	addr, done := startFakeSMTP(t, map[string][]string{"RCPT": {"550 5.1.1 user unknown"}})
	host, port := splitHostPort(t, addr)
	s := NewSMTPSender(SMTPConfig{Host: host, Port: port, TLS: "none"})
	_, err := s.Send(context.Background(), Message{FromName: "Studio", FromAddr: "noreply@test.local", To: "bad@example.com", Subject: "Hi", Text: "hello", HTML: "<p>hello</p>"})
	if !errors.Is(err, ErrRecipientRejected) {
		t.Fatalf("err=%v", err)
	}
	<-done
}

func TestSMTPSenderTemporaryFailure(t *testing.T) {
	addr, done := startFakeSMTP(t, map[string][]string{"DATA_END": {"451 temp fail"}})
	host, port := splitHostPort(t, addr)
	s := NewSMTPSender(SMTPConfig{Host: host, Port: port, TLS: "none"})
	_, err := s.Send(context.Background(), Message{FromName: "Studio", FromAddr: "noreply@test.local", To: "client@example.com", Subject: "Hi", Text: "hello", HTML: "<p>hello</p>"})
	if !errors.Is(err, ErrSendFailed) {
		t.Fatalf("err=%v", err)
	}
	<-done
}

func TestSMTPSenderTimeout(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		fmt.Fprintf(conn, "220 ready\r\n")
		time.Sleep(2 * time.Second)
	}()
	host, port := splitHostPort(t, ln.Addr().String())
	s := NewSMTPSender(SMTPConfig{Host: host, Port: port, TLS: "none"})
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_, err = s.Send(ctx, Message{FromName: "Studio", FromAddr: "noreply@test.local", To: "client@example.com", Subject: "Hi", Text: "hello", HTML: "<p>hello</p>"})
	if !errors.Is(err, ErrSendTimeout) {
		t.Fatalf("err=%v", err)
	}
	<-done
}

func startFakeSMTP(t *testing.T, responses map[string][]string) (addr string, done chan struct{}) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done = make(chan struct{})
	go func() {
		defer close(done)
		defer ln.Close()
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		r := bufio.NewReader(conn)
		write := func(line string) { fmt.Fprintf(conn, "%s\r\n", line) }
		write("220 ready")
		dataMode := false
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			if dataMode {
				if strings.TrimSpace(line) == "." {
					dataMode = false
					for _, l := range responses["DATA_END"] {
						write(l)
					}
					if len(responses["DATA_END"]) == 0 {
						write("250 queued")
					}
				}
				continue
			}
			cmd := strings.ToUpper(strings.TrimSpace(line))
			switch {
			case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
				for _, l := range responses["EHLO"] {
					write(l)
				}
				if len(responses["EHLO"]) == 0 {
					write("250 ok")
				}
			case strings.HasPrefix(cmd, "MAIL FROM:"):
				for _, l := range responses["MAIL"] {
					write(l)
				}
				if len(responses["MAIL"]) == 0 {
					write("250 ok")
				}
			case strings.HasPrefix(cmd, "RCPT TO:"):
				for _, l := range responses["RCPT"] {
					write(l)
				}
				if len(responses["RCPT"]) == 0 {
					write("250 ok")
				}
			case strings.HasPrefix(cmd, "DATA"):
				for _, l := range responses["DATA"] {
					write(l)
				}
				if len(responses["DATA"]) == 0 {
					write("354 go")
				}
				dataMode = true
			case strings.HasPrefix(cmd, "QUIT"):
				write("221 bye")
				return
			default:
				write("250 ok")
			}
		}
	}()
	return ln.Addr().String(), done
}

func splitHostPort(t *testing.T, addr string) (string, int) {
	t.Helper()
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	var port int
	fmt.Sscanf(portStr, "%d", &port)
	return host, port
}
