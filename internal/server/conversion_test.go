package server

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDeliveryConversionAndPermissions(t *testing.T) {
	f := newFullServer(t, "required")
	ctx := context.Background()
	host, port, got := fakeSMTP(t)
	resp := postJSON(t, f.admin, f.ts.URL+"/admin/settings", map[string]any{"smtp": map[string]any{"enabled": true, "host": host, "port": port, "from": "sender@polka.test", "security": "none"}})
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatal(resp.StatusCode)
	}
	// Any recipient domain is accepted; addresses remain personal.
	postJSON(t, f.reader, f.ts.URL+"/api/v1/me/reader-email", map[string]string{"email": "recipient@example.org"}).Body.Close()
	settings := getJSONWith(t, f.reader, f.ts.URL+"/api/v1/me/reader-email")
	if settings["conversionEnabled"] != false {
		t.Fatal("conversion must be opt-in")
	}
	send := func(format string) int {
		t.Helper()
		r := postJSON(t, f.reader, f.ts.URL+"/api/v1/books/"+itoa64(f.bookID)+"/send", map[string]string{"format": format})
		r.Body.Close()
		return r.StatusCode
	}
	if code := send("epub3"); code != 400 {
		t.Fatalf("disabled conversion: %d", code)
	}
	resp = postJSON(t, f.reader, f.ts.URL+"/admin/settings", map[string]bool{"conversionEnabled": true})
	resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Fatalf("reader changed admin setting: %d", resp.StatusCode)
	}
	postJSON(t, f.admin, f.ts.URL+"/admin/settings", map[string]bool{"conversionEnabled": true}).Body.Close()
	if getJSONWith(t, f.reader, f.ts.URL+"/api/v1/me/reader-email")["conversionEnabled"] != true {
		t.Fatal("admin toggle not visible")
	}
	if code := send("fb2"); code != 400 {
		t.Fatalf("invalid target: %d", code)
	}
	dir := t.TempDir()
	binary := filepath.Join(dir, "fbc")
	marker := filepath.Join(dir, "working-directory")
	// A controlled executable verifies wiring; the real converter has its own test.
	script := "#!/bin/sh\nprintf '%s' \"$PWD\" > '" + marker + "'\nwhile [ \"$#\" -gt 0 ]; do if [ \"$1\" = --output-file ]; then shift; output=$1; fi; shift; done\nprintf 'converted-epub-fixture' > \"$output\"\n"
	if err := os.WriteFile(binary, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("POLKA_FBC_BINARY", binary)
	if code := send("epub3"); code != 200 {
		t.Fatalf("converted send: %d", code)
	}
	var message string
	select {
	case message = <-got:
	case <-time.After(5 * time.Second):
		t.Fatal("no SMTP message")
	}
	parsed, err := mail.ReadMessage(strings.NewReader(message))
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Header.Get("To") != "recipient@example.org" || parsed.Header.Get("From") != "sender@polka.test" {
		t.Fatal("wrong SMTP sender or recipient")
	}
	_, params, err := mime.ParseMediaType(parsed.Header.Get("Content-Type"))
	if err != nil {
		t.Fatal(err)
	}
	parts := multipart.NewReader(parsed.Body, params["boundary"])
	var attachment []byte
	for {
		part, err := parts.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if part.FileName() == "" {
			continue
		}
		if !strings.HasSuffix(part.FileName(), ".epub") {
			t.Fatalf("attachment: %s", part.FileName())
		}
		attachment, err = io.ReadAll(base64.NewDecoder(base64.StdEncoding, part))
		if err != nil {
			t.Fatal(err)
		}
	}
	if !bytes.Equal(attachment, []byte("converted-epub-fixture")) {
		t.Fatalf("wrong attachment %q", attachment)
	}
	if id, err := f.st.BookIDByDigest(ctx, referenceDigest(attachment)); err != nil || id != f.bookID {
		t.Fatalf("converted digest: %d %v", id, err)
	}
	working, err := os.ReadFile(marker)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(string(working)); !os.IsNotExist(err) {
		t.Fatalf("temporary conversion files remain: %v", err)
	}
	// Missing converter returns an error, never falls back to mailing raw FB2.
	t.Setenv("POLKA_FBC_BINARY", filepath.Join(dir, "not-installed"))
	if code := send("epub3"); code != http.StatusUnprocessableEntity {
		t.Fatalf("missing converter: %d", code)
	}
}

func TestRealConvertedDelivery(t *testing.T) {
	binary := os.Getenv("POLKA_TEST_FBC")
	if binary == "" {
		t.Skip("set POLKA_TEST_FBC for the complete real-converter delivery test")
	}
	t.Setenv("POLKA_FBC_BINARY", binary)
	f := newFullServer(t, "required")
	host, port, got := fakeSMTP(t)
	postJSON(t, f.admin, f.ts.URL+"/admin/settings", map[string]any{"conversionEnabled": true, "smtp": map[string]any{"enabled": true, "host": host, "port": port, "from": "sender@polka.test", "security": "none"}}).Body.Close()
	postJSON(t, f.reader, f.ts.URL+"/api/v1/me/reader-email", map[string]string{"email": "reader@example.org"}).Body.Close()
	resp := postJSON(t, f.reader, f.ts.URL+"/api/v1/books/"+itoa64(f.bookID)+"/send", map[string]string{"format": "epub3"})
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		t.Fatalf("real delivery: %d %s", resp.StatusCode, data)
	}
	select {
	case raw := <-got:
		message, err := mail.ReadMessage(strings.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		_, params, err := mime.ParseMediaType(message.Header.Get("Content-Type"))
		if err != nil {
			t.Fatal(err)
		}
		parts := multipart.NewReader(message.Body, params["boundary"])
		for {
			part, err := parts.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasSuffix(part.FileName(), ".epub") {
				continue
			}
			data, err := io.ReadAll(base64.NewDecoder(base64.StdEncoding, part))
			if err != nil {
				t.Fatal(err)
			}
			z, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
			if err != nil {
				t.Fatal(err)
			}
			if len(z.File) < 3 {
				t.Fatal("empty EPUB archive")
			}
			if id, err := f.st.BookIDByDigest(context.Background(), referenceDigest(data)); err != nil || id != f.bookID {
				t.Fatalf("real EPUB digest: %d %v", id, err)
			}
			return
		}
		t.Fatal("real EPUB attachment missing")
	case <-time.After(5 * time.Second):
		t.Fatal("no test SMTP message")
	}
}
