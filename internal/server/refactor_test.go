package server

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/vestigiumincaligne/polka/internal/store"
)

func TestWishlistPaginationSkipsMissingBooks(t *testing.T) {
	f := newFullServer(t, "required")
	ctx := context.Background()
	user, err := f.users.GetByLogin(ctx, "reader")
	if err != nil {
		t.Fatal(err)
	}
	list, err := f.users.Wishlist(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < opdsPageSize+1; i++ {
		id, err := f.st.AddBook(ctx, &store.BookInput{Title: fmt.Sprint(i), Folder: "extra", File: fmt.Sprint(i), Ext: "fb2"})
		if err != nil {
			t.Fatal(err)
		}
		if err := f.users.AddToList(ctx, user.ID, list.ID, id); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []int64{9999, 10000} {
		if err := f.users.AddToList(ctx, user.ID, list.ID, id); err != nil {
			t.Fatal(err)
		}
	}
	// Exercise the real handler through two OPDS pages, not just a helper.
	readPage := func(page int) opdsFeed {
		t.Helper()
		response, err := f.reader.Get(fmt.Sprintf("%s/opds/wishlist?page=%d", f.ts.URL, page))
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		var feed opdsFeed
		if err := xml.NewDecoder(response.Body).Decode(&feed); err != nil {
			t.Fatal(err)
		}
		return feed
	}
	first, second := readPage(0), readPage(1)
	if len(first.Entries) != opdsPageSize || len(second.Entries) != 1 {
		t.Fatalf("live page sizes: %d %d", len(first.Entries), len(second.Entries))
	}
	for _, entry := range first.Entries {
		if entry.ID == second.Entries[0].ID {
			t.Fatal("book repeated across pages")
		}
	}
}

func TestSanitizeLongUnicodeFilename(t *testing.T) {
	for _, text := range []string{"A" + strings.Repeat("я", 70), "A" + strings.Repeat("📚", 40)} {
		name := sanitizeFileName(text)
		if !utf8.ValidString(name) || len(name) > 120 || !strings.HasPrefix(text, name) {
			t.Fatalf("broken filename %q", name)
		}
	}
}

func TestDeliveryWithoutLibraryIsAnErrorNotAPanic(t *testing.T) {
	s := &Server{}
	_, err := s.prepareDelivery(context.Background(), &store.BookFile{ID: 1, File: "1", Ext: "fb2"}, formatOriginal)
	if !errors.Is(err, errDeliveryLibrary) {
		t.Fatalf("missing library: %v", err)
	}
}

func TestPublicCatalogStillRequiresLoginToDeleteProgress(t *testing.T) {
	f := newFullServer(t, "public")
	request, err := http.NewRequest("DELETE", f.ts.URL+"/api/v1/read/1/progress", nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("anonymous delete: %d", response.StatusCode)
	}
}
