package server

import (
	"context"
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vestigiumincaligne/polka/internal/auth"
	"github.com/vestigiumincaligne/polka/internal/store"
)

func TestWishlistOPDSProgressLifecycle(t *testing.T) {
	f := newFullServer(t, "required")
	ctx := context.Background()
	reader, err := f.users.GetByLogin(ctx, "reader")
	if err != nil {
		t.Fatal(err)
	}
	admin, err := f.users.GetByLogin(ctx, "admin")
	if err != nil {
		t.Fatal(err)
	}
	postJSON(t, f.reader, f.ts.URL+"/api/v1/books/"+itoa64(f.bookID)+"/wishlist", map[string]bool{"add": true}).Body.Close()
	postJSON(t, f.admin, f.ts.URL+"/api/v1/books/"+itoa64(f.bookID2)+"/wishlist", map[string]bool{"add": true}).Body.Close()
	// An anonymous caller cannot see either user's list.
	resp, err := http.Get(f.ts.URL + "/opds/wishlist")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("anonymous wishlist: %d", resp.StatusCode)
	}
	checkList := func() {
		t.Helper()
		req, _ := http.NewRequest("GET", f.ts.URL+"/opds/wishlist", nil)
		req.SetBasicAuth("reader", "reader123")
		req.Header.Set("Accept-Language", "ru")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var feed opdsFeed
		if err := xml.NewDecoder(resp.Body).Decode(&feed); err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != 200 || feed.Title != "Want to read" || len(feed.Entries) != 1 {
			t.Fatalf("wishlist feed: %d %+v", resp.StatusCode, feed)
		}
		var link string
		for _, l := range feed.Entries[0].Links {
			if l.Rel == relAcquisition {
				link = l.Href
				break
			}
		}
		if !strings.HasSuffix(link, "/"+itoa64(f.bookID)) {
			t.Fatalf("wrong personal acquisition: %q", link)
		}
	}
	checkList()
	// Download the exact acquisition link used in this personal feed.
	req, _ := http.NewRequest("GET", f.ts.URL+"/Images/fb2/"+itoa64(f.bookID), nil)
	req.SetBasicAuth("reader", "reader123")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	doc := referenceDigest(data)
	if id, err := f.st.BookIDByDigest(ctx, doc); err != nil || id != f.bookID {
		t.Fatalf("download bridge: %d %v", id, err)
	}
	postJSON(t, f.reader, f.ts.URL+"/api/v1/me/kosync", map[string]string{"password": "device-pass-123"}).Body.Close()
	push := func() {
		t.Helper()
		code, _ := kosyncDo(t, f.ts.URL, "PUT", "/syncs/progress", "reader", md5hex("device-pass-123"), `{"document":"`+doc+`","progress":"x","percentage":0.37,"device":"KOReader"}`)
		if code != 200 {
			t.Fatalf("push: %d", code)
		}
	}
	push()
	if err := f.users.SaveProgress(ctx, admin.ID, f.bookID, auth.Progress{Overall: 0.8}); err != nil {
		t.Fatal(err)
	}
	req, _ = http.NewRequest("DELETE", f.ts.URL+"/api/v1/read/"+itoa64(f.bookID)+"/progress", nil)
	resp, err = f.reader.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("delete: %d", resp.StatusCode)
	}
	if _, err := f.users.GetProgress(ctx, reader.ID, f.bookID); err != auth.ErrNotFound {
		t.Fatalf("progress not removed: %v", err)
	}
	if p, err := f.users.GetProgress(ctx, admin.ID, f.bookID); err != nil || p.Overall != 0.8 {
		t.Fatalf("other user changed: %+v %v", p, err)
	}
	checkList() // Reading progress deletion must not change the wishlist.
	push()      // No redownload or permanent suppression is required.
	if p, err := f.users.GetProgress(ctx, reader.ID, f.bookID); err != nil || p.Overall != 0.37 {
		t.Fatalf("not resumed: %+v %v", p, err)
	}
	snapshot, err := f.st.BookFile(ctx, f.bookID)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.st.Clear(ctx); err != nil {
		t.Fatal(err)
	}
	im, err := f.st.NewImport(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer im.Abort()
	// An unrelated arrival must not inherit the original book's user state.
	if err := im.Add(&store.BookInput{Title: "New", Folder: "arch-001.zip", File: "300", Ext: "fb2"}); err != nil {
		t.Fatal(err)
	}
	if err := im.Add(&store.BookInput{Title: "Same book", Folder: snapshot.Folder, File: snapshot.File, Ext: snapshot.Ext, Size: snapshot.Size, LibID: snapshot.LibID}); err != nil {
		t.Fatal(err)
	}
	if _, err := im.Finish(); err != nil {
		t.Fatal(err)
	}
	checkList()
	push()
	if p, err := f.users.GetProgress(ctx, reader.ID, f.bookID); err != nil || p.Overall != 0.37 {
		t.Fatalf("reimport progress: %+v %v", p, err)
	}
}

func TestOPDSLocalAndProxyOrigins(t *testing.T) {
	for _, test := range []struct{ name, host, proto, want string }{
		{"local", "127.0.0.1:12791", "", "http://127.0.0.1:12791/opds/search"},
		{"proxy", "books.example.org", "https", "https://books.example.org/opds/search"},
	} {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "http://"+test.host+"/opds/opensearch", nil)
			req.Header.Set("X-Forwarded-Proto", test.proto)
			response := httptest.NewRecorder()
			(&Server{}).handleOpdsOpenSearch(response, req)
			if !strings.Contains(response.Body.String(), test.want) {
				t.Fatalf("wrong origin: %s", response.Body.String())
			}
		})
	}
}
