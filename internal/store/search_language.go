package store

import (
	"context"
	"strings"
)

// SearchLanguage accepts only the languages offered by the search controls.
// Empty means all languages. It is unrelated to interface localization.
func SearchLanguage(value string) string {
	switch value = strings.ToLower(strings.TrimSpace(value)); value {
	case "ru", "uk", "en":
		return value
	default:
		return ""
	}
}

// Values interpolated into SQL come exclusively from the whitelist above.
// Books without language metadata always remain eligible.
func searchLanguageFilter(language []string) string {
	if len(language) == 0 {
		return "1"
	}
	lang := SearchLanguage(language[0])
	if lang == "" {
		return "1"
	}
	return "lower(trim(b.lang)) IN ('', '" + lang + "')"
}

// FilterSearchBooks also covers ISBN results, which bypass the FTS index.
func (s *Store) FilterSearchBooks(ctx context.Context, books []Book, language string) ([]Book, error) {
	filter := searchLanguageFilter([]string{language})
	if filter == "1" || len(books) == 0 {
		return books, nil
	}
	placeholders := make([]string, len(books))
	args := make([]any, len(books))
	for i, b := range books {
		placeholders[i] = "?"
		args[i] = b.ID
	}
	rows, err := s.db.QueryContext(ctx, `SELECT b.id FROM books b WHERE b.deleted=0 AND `+filter+` AND b.id IN (`+strings.Join(placeholders, ",")+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	allowed := make(map[int64]bool, len(books))
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		allowed[id] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	result := make([]Book, 0, len(books))
	for _, b := range books {
		if allowed[b.ID] {
			result = append(result, b)
		}
	}
	return result, nil
}
