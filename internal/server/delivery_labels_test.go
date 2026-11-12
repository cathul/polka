package server

import (
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/mail"
	"strings"
	"testing"
	"time"
)

func TestDeliveryLabelsAndLocalizedBody(t *testing.T) {
	for _, language := range []string{"ru", "en"} {
		t.Run(language, func(t *testing.T) {
			f := newFullServer(t, "required")
			host, port, messages := fakeSMTP(t)
			postJSON(t, f.admin, f.ts.URL+"/admin/settings", map[string]any{"smtp": map[string]any{"enabled": true, "host": host, "port": port, "from": "sender@example.org", "security": "none"}}).Body.Close()
			postJSON(t, f.reader, f.ts.URL+"/api/v1/me/reader-email", map[string]string{"email": "recipient@example.org"}).Body.Close()
			request, err := http.NewRequest(http.MethodPost, f.ts.URL+"/api/v1/books/"+itoa64(f.bookID)+"/send", strings.NewReader(`{"format":"original"}`))
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Polka-Lang", language)
			response, err := f.reader.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != 200 {
				body, _ := io.ReadAll(response.Body)
				t.Fatalf("delivery: %d %s", response.StatusCode, body)
			}
			var raw string
			select {
			case raw = <-messages:
			case <-time.After(5 * time.Second):
				t.Fatal("fake SMTP received nothing")
			}
			message, err := mail.ReadMessage(strings.NewReader(raw))
			if err != nil {
				t.Fatal(err)
			}
			subject, err := new(mime.WordDecoder).DecodeHeader(message.Header.Get("Subject"))
			if err != nil {
				t.Fatal(err)
			}
			const label = "Толстой Лев - Война и мир"
			if subject != label {
				t.Fatalf("subject: %s", subject)
			}
			_, params, err := mime.ParseMediaType(message.Header.Get("Content-Type"))
			if err != nil {
				t.Fatal(err)
			}
			parts := multipart.NewReader(message.Body, params["boundary"])
			text, err := parts.NextPart()
			if err != nil {
				t.Fatal(err)
			}
			body, err := io.ReadAll(text)
			if err != nil {
				t.Fatal(err)
			}
			wantBody := "Book sent by the Polka server"
			if language == "ru" {
				wantBody = "Книга отправлена сервером Polka"
			}
			if strings.TrimSpace(string(body)) != wantBody {
				t.Fatalf("body: %q", body)
			}
			attachment, err := parts.NextPart()
			if err != nil {
				t.Fatal(err)
			}
			if attachment.FileName() != label+".fb2" {
				t.Fatalf("filename: %s", attachment.FileName())
			}
			// Personal recipient remains the previously saved setting.
			var settings map[string]any
			result, err := f.reader.Get(f.ts.URL + "/api/v1/me/reader-email")
			if err != nil {
				t.Fatal(err)
			}
			defer result.Body.Close()
			if err := json.NewDecoder(result.Body).Decode(&settings); err != nil {
				t.Fatal(err)
			}
			if settings["email"] != "recipient@example.org" {
				t.Fatal("recipient setting changed")
			}
		})
	}
}
