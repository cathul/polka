package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"strconv"
	"strings"

	"github.com/vestigiumincaligne/polka/internal/mailer"
)

// Sending a book to an e-reader by e-mail (send-to-Kindle and friends):
// the admin configures SMTP, each user has their own reader address.

const maxSendBytes = 18 << 20 // Before MIME/base64 expansion.
const maxSendSourceBytes = 64 << 20

func (s *Server) conversionEnabled(r *http.Request) bool {
	return s.users.GetSetting(r.Context(), "delivery.convert_fb2", "0") == "1"
}

// smtpConfig reads the SMTP settings (the password is decrypted).
func (s *Server) smtpConfig(r *http.Request) mailer.Config {
	ctx := r.Context()
	get := func(k string) string { return s.users.GetSetting(ctx, "smtp."+k, "") }
	port, _ := strconv.Atoi(get("port"))
	sec := get("security")
	if sec == "" {
		sec = "starttls"
	}
	return mailer.Config{
		Host:     get("host"),
		Port:     port,
		User:     get("user"),
		Password: s.decryptSecret(get("password")),
		From:     get("from"),
		Security: sec,
	}
}

func (s *Server) smtpEnabled(r *http.Request) bool {
	return s.users.GetSetting(r.Context(), "smtp.enabled", "0") == "1" && s.smtpConfig(r).Valid()
}

// readerEmail — the user's personal e-reader address.
func (s *Server) readerEmail(r *http.Request, userID int64) string {
	return s.users.GetSetting(r.Context(), fmt.Sprintf("reader_email:%d", userID), "")
}

// GET/POST /api/v1/me/reader-email — the current user's reader address.
func (s *Server) handleReaderEmail(w http.ResponseWriter, r *http.Request) {
	u := s.currentUser(r)
	if u == nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	if r.Method == http.MethodPost {
		var req struct {
			Email string `json:"email"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		email := strings.TrimSpace(req.Email)
		if email != "" {
			address, err := mail.ParseAddress(email)
			if err != nil || strings.ContainsAny(email, "\r\n") {
				http.Error(w, "invalid email", http.StatusBadRequest)
				return
			}
			email = address.Address
		}
		if err := s.users.SetSetting(r.Context(), fmt.Sprintf("reader_email:%d", u.ID), email); err != nil {
			s.apiError(w, err)
			return
		}
	}
	writeJSON(w, map[string]any{
		"email":             s.readerEmail(r, u.ID),
		"smtpReady":         s.smtpEnabled(r),
		"conversionEnabled": s.conversionEnabled(r),
	})
}

// POST /api/v1/books/{id}/send — e-mail the book to the user's reader.
func (s *Server) handleSendBook(w http.ResponseWriter, r *http.Request) {
	u := s.currentUser(r)
	if u == nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	if !s.smtpEnabled(r) {
		http.Error(w, "email delivery is not configured", http.StatusServiceUnavailable)
		return
	}
	to := s.readerEmail(r, u.ID)
	if to == "" {
		http.Error(w, "set your reader e-mail first", http.StatusBadRequest)
		return
	}
	bookID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	var req struct {
		Format string `json:"format"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		http.Error(w, tr(reqLang(r), "send.invalid"), http.StatusBadRequest)
		return
	}
	if req.Format == "" {
		req.Format = formatOriginal
	}
	if req.Format != formatOriginal && req.Format != formatEPUB3 {
		http.Error(w, tr(reqLang(r), "send.invalid"), http.StatusBadRequest)
		return
	}
	select {
	case s.deliverySlots <- struct{}{}:
		defer func() { <-s.deliverySlots }()
	default:
		http.Error(w, tr(reqLang(r), "send.busy"), http.StatusTooManyRequests)
		return
	}
	f, err := s.st.BookFile(r.Context(), bookID)
	if err != nil {
		s.apiError(w, err)
		return
	}
	if req.Format == formatEPUB3 && (!s.conversionEnabled(r) || !isFB2(f.Ext)) {
		http.Error(w, tr(reqLang(r), "send.unavailable"), http.StatusBadRequest)
		return
	}
	attachment, err := s.prepareDelivery(r.Context(), f, req.Format)
	if err != nil {
		s.deliveryError(w, r, err)
		return
	}
	cfg := s.smtpConfig(r)
	if err := cfg.Send(to, attachment.title, tr(reqLang(r), "send.body"), attachment.name, attachment.data); err != nil {
		s.log.Warn("send book", "book", bookID, "error", err)
		http.Error(w, tr(reqLang(r), "send.smtpFail"), http.StatusBadGateway)
		return
	}
	tee := &digestTee{}
	tee.Write(attachment.data)
	if err := s.st.SetBookDigestForFile(r.Context(), tee.digest(), f); err != nil {
		s.log.Warn("sent book digest", "book", bookID, "error", err)
	}
	s.log.Info("book sent", "book", bookID)
	writeJSON(w, map[string]any{"sent": true})
}

// POST /admin/smtp/test — verify the SMTP connection (admin only).
func (s *Server) handleSmtpTest(w http.ResponseWriter, r *http.Request) {
	if err := s.smtpConfig(r).Verify(); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

type smtpInput struct {
	Enabled  *bool   `json:"enabled"`
	Host     *string `json:"host"`
	Port     *int    `json:"port"`
	User     *string `json:"user"`
	Password *string `json:"password"` // empty string = leave unchanged
	From     *string `json:"from"`
	Security *string `json:"security"`
}

// smtpSettings — config for the admin panel (no password, just a "set" flag).
func (s *Server) smtpSettings(r *http.Request) map[string]any {
	ctx := r.Context()
	get := func(k string) string { return s.users.GetSetting(ctx, "smtp."+k, "") }
	return map[string]any{
		"enabled":     get("enabled") == "1",
		"host":        get("host"),
		"port":        get("port"),
		"user":        get("user"),
		"from":        get("from"),
		"security":    get("security"),
		"hasPassword": get("password") != "",
	}
}

func (s *Server) saveSmtpSettings(r *http.Request, in smtpInput) error {
	ctx := r.Context()
	set := func(k, v string) error { return s.users.SetSetting(ctx, "smtp."+k, v) }
	if in.Enabled != nil {
		v := "0"
		if *in.Enabled {
			v = "1"
		}
		if err := set("enabled", v); err != nil {
			return err
		}
	}
	if in.Host != nil {
		if err := set("host", strings.TrimSpace(*in.Host)); err != nil {
			return err
		}
	}
	if in.Port != nil {
		if err := set("port", strconv.Itoa(*in.Port)); err != nil {
			return err
		}
	}
	if in.User != nil {
		if err := set("user", strings.TrimSpace(*in.User)); err != nil {
			return err
		}
	}
	if in.From != nil {
		if err := set("from", strings.TrimSpace(*in.From)); err != nil {
			return err
		}
	}
	if in.Security != nil {
		if err := set("security", *in.Security); err != nil {
			return err
		}
	}
	if in.Password != nil && *in.Password != "" {
		if err := set("password", s.encryptSecret(*in.Password)); err != nil {
			return err
		}
	}
	return nil
}
