package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/vestigiumincaligne/polka/internal/auth"
	"github.com/vestigiumincaligne/polka/internal/library"
)

// readerCache keeps the last few parsed books in memory:
// a reader navigates between chapters of one book, so there is no need to re-parse the file.
type readerCache struct {
	loadMu  sync.Mutex
	mu      sync.Mutex
	entries map[int64]*library.FB2Text
	order   []int64
}

const readerCacheSize = 8

func (c *readerCache) get(id int64) *library.FB2Text {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.entries[id]
}

func (c *readerCache) put(id int64, text *library.FB2Text) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = make(map[int64]*library.FB2Text, readerCacheSize)
	}
	if _, exists := c.entries[id]; exists {
		c.entries[id] = text
		return
	}
	for len(c.order) >= readerCacheSize {
		delete(c.entries, c.order[0])
		c.order = c.order[1:]
	}
	c.entries[id] = text
	c.order = append(c.order, id)
}

// bookText returns the parsed book (from the cache or from disk).
func (s *Server) bookText(r *http.Request, bookID int64) (*library.FB2Text, error) {
	if s.lib == nil {
		return nil, library.ErrNoFile
	}
	f, err := s.st.BookFile(r.Context(), bookID)
	if err != nil {
		return nil, err
	}
	version, err := s.lib.SourceVersion(f.Folder, f.File, f.Ext)
	if err != nil {
		return nil, err
	}
	key := fmt.Sprintf("%q:%q:%q:%d:%q:%s", f.Folder, f.File, f.Ext, f.Size, f.LibID, version)
	if text := s.reader.get(bookID); text != nil && text.SourceKey == key {
		return text, nil
	}
	// Avoid parsing twice for the metadata and position requests of one opening.
	s.reader.loadMu.Lock()
	defer s.reader.loadMu.Unlock()
	if text := s.reader.get(bookID); text != nil && text.SourceKey == key {
		return text, nil
	}
	imgURL := func(id string) string { return fmt.Sprintf("/api/v1/read/%d/img/%s", bookID, id) }
	var text *library.FB2Text
	if strings.EqualFold(f.Ext, "fb2") {
		reader, size, openErr := s.lib.Open(f.Folder, f.File, f.Ext)
		if openErr != nil {
			return nil, openErr
		}
		defer reader.Close()
		tee := &digestTee{}
		text, err = library.ParseFB2TextAnchored(io.TeeReader(reader, tee), imgURL)
		if err == nil {
			_, err = io.Copy(tee, reader)
		}
		if err == nil && tee.pos != size {
			err = fmt.Errorf("incomplete FB2 read")
		}
		if err == nil {
			text.Digest = tee.digest()
		} else {
			// Keep the existing lenient web reader for malformed/non-standard
			// FB2 files, but never export their guessed XML coordinates.
			text, err = s.lib.Text(f.Folder, f.File, f.Ext, imgURL)
		}
	} else {
		text, err = s.lib.Text(f.Folder, f.File, f.Ext, imgURL)
	}
	if err != nil {
		return nil, err
	}
	after, err := s.lib.SourceVersion(f.Folder, f.File, f.Ext)
	if err != nil {
		return nil, err
	}
	if version != after {
		return nil, fmt.Errorf("book changed while reading; retry")
	}
	text.SourceKey = key
	s.reader.put(bookID, text)
	return text, nil
}

func (s *Server) readerBookID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	if s.lib == nil {
		http.Error(w, "library dir is not configured", http.StatusNotFound)
		return 0, false
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return 0, false
	}
	return id, true
}

// GET /api/v1/read/{id} — table of contents and footnotes.
func (s *Server) handleReadMeta(w http.ResponseWriter, r *http.Request) {
	bookID, ok := s.readerBookID(w, r)
	if !ok {
		return
	}
	d, err := s.st.BookDetails(r.Context(), bookID)
	if err != nil {
		s.apiError(w, err)
		return
	}

	format := strings.ToLower(d.Ext)
	switch format {
	case "fb2", "txt", "epub":
		// chapters are rendered by our reader — continue below
	case "pdf":
		// rendered by a browser engine (pdf.js) from the file
		writeJSON(w, map[string]any{
			"bookId":  bookID,
			"format":  format,
			"title":   d.Title,
			"authors": d.AuthorNames,
			"fileUrl": fmt.Sprintf("/Images/fb2/%d?inline=1", bookID),
		})
		return
	default:
		http.Error(w, "online reading is not available for this format", http.StatusUnsupportedMediaType)
		return
	}

	text, err := s.bookText(r, bookID)
	if err != nil {
		s.log.Warn("read book failed", "book", bookID, "error", err)
		s.apiError(w, err)
		return
	}

	lang := reqLang(r)
	chapters := make([]map[string]any, 0, len(text.Chapters))
	for i, ch := range text.Chapters {
		title := ch.Title
		if title == "" {
			if i == 0 {
				title = tr(lang, "reader.start")
			} else {
				title = tr(lang, "reader.chapter", i+1)
			}
		}
		entry := map[string]any{"index": i, "title": title}
		if text.Anchors != nil {
			entry["textLength"] = text.Anchors.Lengths[i]
		}
		chapters = append(chapters, entry)
	}
	writeJSON(w, map[string]any{
		"bookId":    bookID,
		"format":    format,
		"title":     d.Title,
		"authors":   d.AuthorNames,
		"chapters":  chapters,
		"notes":     text.Notes,
		"fb2Digest": text.Digest,
	})
}

// GET /api/v1/read/{id}/chapter/{n} — HTML of a single chapter.
func (s *Server) handleReadChapter(w http.ResponseWriter, r *http.Request) {
	bookID, ok := s.readerBookID(w, r)
	if !ok {
		return
	}
	n, err := strconv.Atoi(r.PathValue("n"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	text, err := s.bookText(r, bookID)
	if err != nil {
		s.log.Warn("read book failed", "book", bookID, "error", err)
		s.apiError(w, err)
		return
	}
	if n < 0 || n >= len(text.Chapters) {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, map[string]any{
		"index": n,
		"total": len(text.Chapters),
		"title": text.Chapters[n].Title,
		"html":  text.Chapters[n].HTML,
	})
}

// GET /api/v1/read/{id}/img/{imgId} — an illustration from the book.
func (s *Server) handleReadImage(w http.ResponseWriter, r *http.Request) {
	bookID, ok := s.readerBookID(w, r)
	if !ok {
		return
	}
	imgID := r.PathValue("imgId")
	f, err := s.st.BookFile(r.Context(), bookID)
	if err != nil {
		s.apiError(w, err)
		return
	}
	data, mime, err := s.lib.Binary(f.Folder, f.File, f.Ext, imgID)
	if err != nil {
		s.apiError(w, err)
		return
	}
	if mime == "" {
		mime = "image/jpeg"
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Write(data)
}

// GET/POST /api/v1/read/{id}/progress — the user's reading position.
func (s *Server) handleReadProgress(w http.ResponseWriter, r *http.Request) {
	// Progress is user data: it does not need the library (sync-mode
	// clients read proxied books without one).
	bookID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	u := s.currentUser(r)
	if u == nil {
		if r.Method == http.MethodDelete {
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}
		// Public mode without login: the position is stored in the browser.
		writeJSON(w, map[string]any{"stored": false})
		return
	}

	if r.Method == http.MethodDelete {
		if err := s.users.DeleteProgress(r.Context(), u.ID, bookID); err != nil {
			s.apiError(w, err)
			return
		}
		writeJSON(w, map[string]any{"removed": true})
		return
	}
	if r.Method == http.MethodPost {
		var p auth.Progress
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&struct {
			Chapter  *int     `json:"chapter"`
			Position *float64 `json:"position"`
			Overall  *float64 `json:"progress"`
			Locator  *string  `json:"locator"`
		}{&p.Chapter, &p.Position, &p.Overall, &p.Locator}); err != nil {
			http.Error(w, "invalid body", http.StatusBadRequest)
			return
		}
		if p.Position < 0 || p.Position > 1 || p.Overall < 0 || p.Overall > 1 || p.Chapter < 0 {
			http.Error(w, "invalid progress", http.StatusBadRequest)
			return
		}
		var err error
		if document, pointer := s.validateFB2Progress(r, bookID, &p); document != "" {
			err = s.users.SaveFB2Progress(r.Context(), u.ID, bookID, p, document, pointer)
		} else {
			if strings.HasPrefix(p.Locator, "fb2:") {
				p.Locator = "web:chapters"
			}
			err = s.users.SaveProgress(r.Context(), u.ID, bookID, p)
		}
		if err != nil {
			s.apiError(w, err)
			return
		}
		writeJSON(w, map[string]any{"stored": true})
		return
	}

	p, err := s.users.GetProgress(r.Context(), u.ID, bookID)
	if errors.Is(err, auth.ErrNotFound) {
		writeJSON(w, map[string]any{"stored": true, "exists": false, "chapter": 0, "position": 0, "progress": 0})
		return
	}
	if err != nil {
		s.apiError(w, err)
		return
	}
	result := map[string]any{"stored": true, "exists": true, "chapter": p.Chapter, "position": p.Position, "progress": p.Overall, "locator": p.Locator}
	if r.URL.Query().Get("resolve") == "1" {
		if location, ratio := s.resolveFB2Progress(r, u.ID, bookID, p); location != nil {
			result["anchor"] = location
			result["chapter"] = location.Chapter
			result["position"] = ratio
		}
	}
	writeJSON(w, result)
}
