package server

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"testing"

	"github.com/vestigiumincaligne/polka/internal/store"
)

func TestIndependentSearchLanguages(t *testing.T) {
	f := newFullServer(t, "required")
	ctx := context.Background()
	var bookIDs []int64
	for i, lang := range []string{"ru", "uk", "en", ""} {
		id, err := f.st.AddBook(ctx, &store.BookInput{Title: "LanguageTest", Lang: lang, Folder: "books", File: fmt.Sprint(i), Ext: "fb2"})
		if err != nil {
			t.Fatal(err)
		}
		bookIDs = append(bookIDs, id)
	}
	settingsURL := f.ts.URL + "/api/v1/me/search-settings"
	response := postJSON(t, f.reader, settingsURL, map[string]string{"opdsSearchLanguage": "uk"})
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatal(response.StatusCode)
	}
	if got := getJSONWith(t, f.admin, settingsURL)["opdsSearchLanguage"]; got != "" {
		t.Fatalf("preference leaked to another user: %v", got)
	}
	readFeed := func(client *http.Client, path string) opdsFeed {
		t.Helper()
		resp, err := client.Get(f.ts.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatal(resp.StatusCode)
		}
		var feed opdsFeed
		if err := xml.NewDecoder(resp.Body).Decode(&feed); err != nil {
			t.Fatal(err)
		}
		return feed
	}
	if feed := readFeed(f.reader, "/opds/search?q=LanguageTest&searchLanguage=en"); len(feed.Entries) != 2 {
		t.Fatalf("OPDS preference: %d", len(feed.Entries))
	}
	reader, _ := f.users.GetByLogin(ctx, "reader")
	feed := readFeed(f.reader, "/opds/search?q=LanguageTest")
	for _, entry := range feed.Entries {
		var lang string
		// Feed IDs end in the book's numeric ID.
		var id int64
		if _, err := fmt.Sscanf(entry.ID, "urn:polka:book:%d", &id); err != nil {
			t.Fatal(entry.ID)
		}
		if err := f.st.DB().QueryRow(`SELECT lang FROM books WHERE id=?`, id).Scan(&lang); err != nil {
			t.Fatal(err)
		}
		if lang != "uk" && lang != "" {
			t.Fatalf("unexpected language: %s", lang)
		}

	}
	list, err := f.users.Wishlist(ctx, reader.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range bookIDs {
		if err := f.users.AddToList(ctx, reader.ID, list.ID, id); err != nil {
			t.Fatal(err)
		}
	}
	if feed := readFeed(f.reader, "/opds/wishlist"); len(feed.Entries) != 4 {
		t.Fatal("OPDS preference filtered wishlist")
	}
	if feed := readFeed(f.admin, "/opds/search?q=LanguageTest"); len(feed.Entries) != 4 {
		t.Fatalf("default OPDS: %d", len(feed.Entries))
	}
	web := func(lang string) int {
		t.Helper()
		return len(getJSONWith(t, f.reader, f.ts.URL+"/main/getBooks/getSearchTitles?search=LanguageTest&searchLanguage="+lang)["titlesList"].([]any))
	}
	if web("") != 4 || web("en") != 2 || web("ru") != 2 {
		t.Fatal("web search depends on OPDS preference or ignores explicit filter")
	}
	if got := getJSONWith(t, f.reader, settingsURL)["opdsSearchLanguage"]; got != "uk" {
		t.Fatalf("web search changed OPDS preference: %v", got)
	}
	if got := getJSONWith(t, f.reader, f.ts.URL+"/main/getBooks/getSearchAuthorBooks?selectedItemID="+itoa64(f.authorID))["titlesList"].([]any); len(got) != 2 {
		t.Fatal("ordinary author page filtered")
	}
	if got := getJSONWith(t, f.reader, f.ts.URL+"/main/getBooks/getSearchAuthorBooks?selectedItemID="+itoa64(f.authorID)+"&searchLanguage=en")["titlesList"].([]any); len(got) != 0 {
		t.Fatal("filtered author search lost language")
	}
	response = postJSON(t, f.reader, settingsURL, map[string]string{"opdsSearchLanguage": "xx"})
	response.Body.Close()
	if response.StatusCode != 400 {
		t.Fatal("invalid preference accepted")
	}
	if got := getJSONWith(t, f.reader, settingsURL)["opdsSearchLanguage"]; got != "uk" {
		t.Fatal("invalid input overwrote preference")
	}
}
