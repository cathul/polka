package store

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
)

func TestSearchLanguage(t *testing.T) {
	ctx := context.Background()
	st, err := Open(filepath.Join(t.TempDir(), "catalog.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	session, err := st.NewImport(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for i, lang := range []string{"ru", " UK ", "en", "", "  ", "de"} {
		if err := session.Add(&BookInput{Title: fmt.Sprintf("Общий роман %d", i), Authors: []AuthorName{{Last: "Общий", First: fmt.Sprint(i)}}, Series: fmt.Sprintf("Общая серия %d", i), Lang: lang, Folder: "books", File: fmt.Sprint(i), Ext: "fb2"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := session.Finish(); err != nil {
		t.Fatal(err)
	}
	for _, lang := range []string{"ru", "uk", "en", ""} {
		want := 3
		if lang == "" {
			want = 6
		}
		books, err := st.SearchTitles(ctx, "Общий", 20, lang)
		if err != nil || len(books) != want {
			t.Fatalf("titles %q: %d %v", lang, len(books), err)
		}
		authors, err := st.SearchAuthors(ctx, "Общий", 20, lang)
		if err != nil || len(authors) != want {
			t.Fatalf("authors %q: %d %v", lang, len(authors), err)
		}
		series, err := st.SearchSeries(ctx, "Общая", 20, lang)
		if err != nil || len(series) != want {
			t.Fatalf("series %q: %d %v", lang, len(series), err)
		}
		stats, err := st.SearchStats(ctx, "Общий", lang)
		if err != nil || stats.BookTitles != want || stats.Authors != want {
			t.Fatalf("stats %q: %+v %v", lang, stats, err)
		}
		seriesStats, err := st.SearchStats(ctx, "Общая", lang)
		if err != nil || seriesStats.BookSeries != want {
			t.Fatalf("series stats: %+v %v", seriesStats, err)
		}
		seen := map[int64]bool{}
		for offset := 0; offset < want; offset++ {
			page, err := st.SearchBooks(ctx, "Общий", 1, offset, lang)
			if err != nil || len(page) != 1 {
				t.Fatalf("page %d: %v %v", offset, page, err)
			}
			if seen[page[0].ID] {
				t.Fatal("duplicate book across pages")
			}
			seen[page[0].ID] = true
		}
	}
	all, err := st.SearchTitles(ctx, "Общий", 20)
	if err != nil {
		t.Fatal(err)
	}
	filtered, err := st.FilterSearchBooks(ctx, all, "en")
	if err != nil || len(filtered) != 3 {
		t.Fatalf("ISBN filtering: %v %v", filtered, err)
	}
}
