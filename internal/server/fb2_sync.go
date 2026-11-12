package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/vestigiumincaligne/polka/internal/auth"
)

type fb2Location struct {
	Digest  string `json:"digest"`
	Path    string `json:"path"`
	Offset  int    `json:"offset"`
	Chapter int    `json:"chapter"`
}

type deviceLocation struct {
	Document string `json:"document"`
	Pointer  string `json:"pointer"`
}

func deviceLocator(document, pointer string) string {
	data, _ := json.Marshal(deviceLocation{Document: document, Pointer: pointer})
	return "kosync:position:" + string(data)
}

func parseFB2Location(locator string) (fb2Location, bool) {
	var location fb2Location
	if !strings.HasPrefix(locator, "fb2:") {
		return location, false
	}
	err := json.Unmarshal([]byte(strings.TrimPrefix(locator, "fb2:")), &location)
	return location, err == nil && location.Digest != "" && location.Offset >= 0
}

// Called only by the reading screen's explicit resolve request; card/home
// progress requests never parse or open a library file.
func (s *Server) resolveFB2Progress(r *http.Request, userID, bookID int64, p auth.Progress) (*fb2Location, float64) {
	if s.lib == nil || !(strings.HasPrefix(p.Locator, "kosync:") || strings.HasPrefix(p.Locator, "fb2:")) {
		return nil, p.Position
	}
	text, err := s.bookText(r, bookID)
	if err != nil || text.Anchors == nil || text.Digest == "" {
		return nil, p.Position
	}
	pointer := ""
	if web, ok := parseFB2Location(p.Locator); ok {
		if web.Digest != text.Digest {
			return nil, p.Position
		}
		pointer = web.Path + "." + strconv.Itoa(web.Offset)
	} else if strings.HasPrefix(p.Locator, "kosync:position:") {
		var device deviceLocation
		if json.Unmarshal([]byte(strings.TrimPrefix(p.Locator, "kosync:position:")), &device) != nil || device.Document != text.Digest {
			return nil, p.Position
		}
		pointer = device.Pointer
	} else if p.Locator == "kosync:overall" {
		// Pre-bridge positions can be resolved if their last device record still
		// agrees with the displayed percentage and the original file identity.
		id, err := s.st.BookIDByDigest(r.Context(), text.Digest)
		if err != nil || id != bookID {
			return nil, p.Position
		}
		previous, err := s.users.KosyncGet(r.Context(), userID, text.Digest)
		if err != nil || previous.Percentage != p.Overall {
			return nil, p.Position
		}
		pointer = previous.Progress
	}
	path, offset, node, ok := text.Anchors.Resolve(pointer)
	if !ok {
		return nil, p.Position
	}
	ratio := float64(node.Start+offset) / float64(max(1, text.Anchors.Lengths[node.Chapter]))
	return &fb2Location{Digest: text.Digest, Path: path, Offset: offset, Chapter: node.Chapter}, ratio
}

// No path, offset or digest supplied by a browser is trusted on its own.
func (s *Server) validateFB2Progress(r *http.Request, bookID int64, p *auth.Progress) (string, string) {
	location, ok := parseFB2Location(p.Locator)
	if !ok || s.lib == nil {
		return "", ""
	}
	text, err := s.bookText(r, bookID)
	if err != nil || text.Digest != location.Digest || text.Anchors == nil {
		return "", ""
	}
	pointer := location.Path + "." + strconv.Itoa(location.Offset)
	path, offset, node, ok := text.Anchors.Resolve(pointer)
	if !ok || node.Chapter != p.Chapter || offset > utf8.RuneCountInString(node.Text) {
		return "", ""
	}
	f, err := s.st.BookFile(r.Context(), bookID)
	if err != nil {
		return "", ""
	}
	if err := s.st.SetBookDigestForFile(r.Context(), text.Digest, f); err != nil {
		return "", ""
	}
	if id, err := s.st.BookIDByDigest(r.Context(), text.Digest); err != nil || id != bookID {
		return "", ""
	}
	location.Path = path
	location.Chapter = node.Chapter
	raw, _ := json.Marshal(location)
	p.Locator = "fb2:" + string(raw)
	p.Overall = text.Anchors.Fraction(node, offset)
	return text.Digest, path + "." + strconv.Itoa(offset)
}
