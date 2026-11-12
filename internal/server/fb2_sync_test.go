package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFB2DeviceWebRoundTrip(t *testing.T) {
	f := newFullServer(t, "required")
	ctx := context.Background()
	response, err := f.reader.Get(f.ts.URL + "/Images/fb2/" + itoa64(f.bookID))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	digest := referenceDigest(raw)
	postJSON(t, f.reader, f.ts.URL+"/api/v1/me/kosync", map[string]string{"password": "device-pass-123"}).Body.Close()
	key := md5hex("device-pass-123")
	const pointer = "/FictionBook/body/section/p/text().2"
	// Ordinary sync push and card progress must work with the library offline:
	// neither request is allowed to build the text index.
	archive := filepath.Join(f.dir, "lib", "arch-001.zip")
	if err := os.Rename(archive, archive+".offline"); err != nil {
		t.Fatal(err)
	}
	input, _ := json.Marshal(map[string]any{"document": digest, "progress": pointer, "percentage": 0.03, "device": "KOReader", "device_id": "device-1"})
	if status, body := kosyncDo(t, f.ts.URL, http.MethodPut, "/syncs/progress", "reader", key, string(input)); status != 200 {
		t.Fatalf("push: %d %v", status, body)
	}
	ordinary := getJSONWith(t, f.reader, f.ts.URL+"/api/v1/read/"+itoa64(f.bookID)+"/progress")
	if ordinary["progress"] != 0.03 || ordinary["anchor"] != nil {
		t.Fatalf("card progress: %v", ordinary)
	}
	if err := os.Rename(archive+".offline", archive); err != nil {
		t.Fatal(err)
	}
	meta := getJSONWith(t, f.reader, f.ts.URL+"/api/v1/read/"+itoa64(f.bookID))
	if meta["fb2Digest"] != digest {
		t.Fatal("original file digest mismatch")
	}
	resolved := getJSONWith(t, f.reader, f.ts.URL+"/api/v1/read/"+itoa64(f.bookID)+"/progress?resolve=1")
	anchor, ok := resolved["anchor"].(map[string]any)
	if !ok {
		t.Fatalf("no resolved position: %v", resolved)
	}
	if anchor["offset"] != float64(2) || anchor["chapter"] != float64(0) {
		t.Fatalf("position: %v", anchor)
	}
	chapter := getJSONWith(t, f.reader, f.ts.URL+"/api/v1/read/"+itoa64(f.bookID)+"/chapter/0")
	if !strings.Contains(chapter["html"].(string), anchor["path"].(string)) {
		t.Fatal("anchor is not in rendered HTML")
	}
	anchor["offset"] = 4
	location, _ := json.Marshal(anchor)
	saved := postJSON(t, f.reader, f.ts.URL+"/api/v1/read/"+itoa64(f.bookID)+"/progress", map[string]any{"chapter": 0, "position": 0.6, "progress": 0.6, "locator": "fb2:" + string(location)})
	saved.Body.Close()
	if saved.StatusCode != 200 {
		t.Fatal(saved.StatusCode)
	}
	status, device := kosyncDo(t, f.ts.URL, http.MethodGet, "/syncs/progress/"+digest, "reader", key, "")
	if status != 200 || device["device"] != "Polka Web" || device["progress"] != anchor["path"].(string)+".4" {
		t.Fatalf("device pull: %v", device)
	}
	user, err := f.users.GetByLogin(ctx, "reader")
	if err != nil {
		t.Fatal(err)
	}
	web, err := f.users.GetProgress(ctx, user.ID, f.bookID)
	if err != nil {
		t.Fatal(err)
	}
	if web.Overall != device["percentage"] {
		t.Fatal("half-written web/device update")
	}
	// A locator from a different byte stream must never change the device record.
	anchor["digest"] = strings.Repeat("f", 32)
	location, _ = json.Marshal(anchor)
	saved = postJSON(t, f.reader, f.ts.URL+"/api/v1/read/"+itoa64(f.bookID)+"/progress", map[string]any{"chapter": 0, "position": 0.1, "progress": 0.1, "locator": "fb2:" + string(location)})
	saved.Body.Close()
	if saved.StatusCode != 200 {
		t.Fatal(saved.StatusCode)
	}
	_, unchanged := kosyncDo(t, f.ts.URL, http.MethodGet, "/syncs/progress/"+digest, "reader", key, "")
	if unchanged["timestamp"] != device["timestamp"] || unchanged["progress"] != device["progress"] {
		t.Fatal("invalid locator overwrote device position")
	}
	// A converted format may map to the book, but its XPointer is not FB2's.
	converted := strings.Repeat("a", 32)
	if err := f.st.SetBookDigest(ctx, converted, f.bookID); err != nil {
		t.Fatal(err)
	}
	input, _ = json.Marshal(map[string]any{"document": converted, "progress": pointer, "percentage": 0.2, "device": "KOReader", "device_id": "device-1"})
	if status, _ := kosyncDo(t, f.ts.URL, http.MethodPut, "/syncs/progress", "reader", key, string(input)); status != 200 {
		t.Fatal(status)
	}
	mismatch := getJSONWith(t, f.reader, f.ts.URL+"/api/v1/read/"+itoa64(f.bookID)+"/progress?resolve=1")
	if mismatch["anchor"] != nil {
		t.Fatal("converted-file pointer treated as original FB2")
	}
}

func TestFB2ChangedArchiveDoesNotUseStaleReaderCache(t *testing.T) {
	f := newFullServer(t, "required")
	url := f.ts.URL + "/api/v1/read/" + itoa64(f.bookID)
	first := getJSONWith(t, f.reader, url)
	if first["fb2Digest"] == "" {
		t.Fatal("missing digest")
	}
	archive := filepath.Join(f.dir, "lib", "arch-001.zip")
	if err := os.WriteFile(archive, []byte("invalid replacement archive"), 0600); err != nil {
		t.Fatal(err)
	}
	response, err := f.reader.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode == 200 {
		t.Fatal("stale text served after source replacement")
	}
}

func TestFB2ConcurrentOpening(t *testing.T) {
	f := newFullServer(t, "required")
	results := make(chan error, 8)
	for i := 0; i < 8; i++ {
		go func() {
			response, err := f.reader.Get(f.ts.URL + "/api/v1/read/" + itoa64(f.bookID))
			if err != nil {
				results <- err
				return
			}
			defer response.Body.Close()
			var meta map[string]any
			if err := json.NewDecoder(response.Body).Decode(&meta); err != nil {
				results <- err
				return
			}
			if response.StatusCode != 200 || meta["fb2Digest"] == "" {
				results <- fmt.Errorf("invalid metadata: %d", response.StatusCode)
				return
			}
			results <- nil
		}()
	}
	for i := 0; i < 8; i++ {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
}
