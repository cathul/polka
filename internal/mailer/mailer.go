// Package mailer sends e-mails with a single file attachment over SMTP —
// used to deliver books to e-readers (send-to-Kindle and friends).
package mailer

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"mime"
	"net"
	"net/smtp"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config describes the sending SMTP account.
type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	From     string // envelope/From address; falls back to User
	Security string // "tls" (implicit), "starttls", or "none"
}

func (c Config) from() string {
	if c.From != "" {
		return c.From
	}
	return c.User
}

// Valid reports whether the config has the minimum to attempt sending.
func (c Config) Valid() bool {
	return c.Host != "" && c.Port != 0 && c.from() != ""
}

// Send delivers a message with one attachment to addr.
func (c Config) Send(to, subject, body, attachName string, attach []byte) error {
	if !c.Valid() {
		return fmt.Errorf("smtp is not configured")
	}
	msg := c.build(to, subject, body, attachName, attach)
	return c.deliver(to, msg)
}

// Verify opens an authenticated SMTP session without sending — used by
// the "test connection" button in the admin panel.
func (c Config) Verify() error {
	if !c.Valid() {
		return fmt.Errorf("smtp is not configured")
	}
	client, err := c.dial()
	if err != nil {
		return err
	}
	defer client.Close()
	if err := c.auth(client); err != nil {
		return err
	}
	return client.Quit()
}

func (c Config) deliver(to string, msg []byte) error {
	client, err := c.dial()
	if err != nil {
		return err
	}
	defer client.Close()
	if err := c.auth(client); err != nil {
		return err
	}
	if err := client.Mail(c.from()); err != nil {
		return fmt.Errorf("MAIL FROM: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("RCPT TO: %w", err)
	}
	wc, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := wc.Write(msg); err != nil {
		wc.Close()
		return err
	}
	if err := wc.Close(); err != nil {
		return err
	}
	return client.Quit()
}

func (c Config) dial() (*smtp.Client, error) {
	addr := net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
	conn, err := net.DialTimeout("tcp", addr, 30*time.Second)
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}
	if err := conn.SetDeadline(time.Now().Add(30 * time.Second)); err != nil {
		conn.Close()
		return nil, err
	}
	if c.Security == "tls" {
		tlsConn := tls.Client(conn, &tls.Config{ServerName: c.Host, MinVersion: tls.VersionTLS12})
		if err := tlsConn.Handshake(); err != nil {
			conn.Close()
			return nil, fmt.Errorf("tls handshake: %w", err)
		}
		conn = tlsConn
	}
	client, err := smtp.NewClient(conn, c.Host)
	if err != nil {
		conn.Close()
		return nil, err
	}

	if c.Security == "starttls" {
		if err := client.StartTLS(&tls.Config{ServerName: c.Host}); err != nil {
			client.Close()
			return nil, fmt.Errorf("starttls: %w", err)
		}
	}
	return client, nil
}

func (c Config) auth(client *smtp.Client) error {
	if c.User == "" {
		return nil
	}
	if err := client.Auth(smtp.PlainAuth("", c.User, c.Password, c.Host)); err != nil {
		return fmt.Errorf("auth: %w", err)
	}
	return nil
}

func (c Config) build(to, subject, body, attachName string, attach []byte) []byte {
	boundary := "polka-" + fmt.Sprint(time.Now().UnixNano())
	var b strings.Builder
	enc := mime.QEncoding.Encode
	fmt.Fprintf(&b, "From: %s\r\n", c.from())
	fmt.Fprintf(&b, "To: %s\r\n", to)
	fmt.Fprintf(&b, "Subject: %s\r\n", enc("utf-8", subject))
	b.WriteString("MIME-Version: 1.0\r\n")
	fmt.Fprintf(&b, "Content-Type: multipart/mixed; boundary=%q\r\n\r\n", boundary)

	fmt.Fprintf(&b, "--%s\r\n", boundary)
	b.WriteString("Content-Type: text/plain; charset=utf-8\r\n\r\n")
	b.WriteString(body + "\r\n\r\n")

	fmt.Fprintf(&b, "--%s\r\n", boundary)
	attachmentType := "application/octet-stream"
	switch strings.ToLower(filepath.Ext(attachName)) {
	case ".epub":
		attachmentType = "application/epub+zip"
	case ".fb2":
		attachmentType = "application/x-fictionbook+xml"
	case ".pdf":
		attachmentType = "application/pdf"
	case ".txt":
		attachmentType = "text/plain"
	}
	fmt.Fprintf(&b, "Content-Type: %s\r\n", mime.FormatMediaType(attachmentType, map[string]string{"name": attachName}))
	b.WriteString("Content-Transfer-Encoding: base64\r\n")
	fmt.Fprintf(&b, "Content-Disposition: %s\r\n\r\n", mime.FormatMediaType("attachment", map[string]string{"filename": attachName}))
	enc64 := base64.StdEncoding.EncodeToString(attach)
	for i := 0; i < len(enc64); i += 76 {
		end := i + 76
		if end > len(enc64) {
			end = len(enc64)
		}
		b.WriteString(enc64[i:end] + "\r\n")
	}
	fmt.Fprintf(&b, "\r\n--%s--\r\n", boundary)
	return []byte(b.String())
}
