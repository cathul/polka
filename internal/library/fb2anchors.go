package library

// The supported location syntax follows CREngine's normalized named XPointers:
// https://github.com/koreader/crengine/blob/master/crengine/src/lvtinydom.cpp
// This deliberately does not emulate CREngine's layout or repair malformed XML.
import (
	"encoding/xml"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	nethtml "golang.org/x/net/html"
)

type FB2Anchor struct {
	Chapter int    `json:"chapter"`
	Start   int    `json:"-"`
	Text    string `json:"-"`
	First   bool   `json:"-"`
}

type FB2Anchors struct {
	Nodes     map[string]FB2Anchor
	Lengths   []int
	Total     int
	Ambiguous map[string]bool
}

var namedStep = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9_-]*|text\(\))(?:\[([1-9][0-9]*)\])?$`)

func NormalizeFB2Path(path string) (string, bool) {
	if len(path) > 4096 || !strings.HasPrefix(path, "/FictionBook") {
		return "", false
	}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) > 128 {
		return "", false
	}
	for i, part := range parts {
		match := namedStep.FindStringSubmatch(part)
		if match == nil {
			return "", false
		}
		index := match[2]
		if index == "" {
			index = "1"
		}
		if _, err := strconv.ParseUint(index, 10, 32); err != nil {
			return "", false
		}
		parts[i] = match[1] + "[" + index + "]"
	}
	if parts[0] != "FictionBook[1]" {
		return "", false
	}
	return "/" + strings.Join(parts, "/"), true
}

// Resolve accepts only text positions with a checked character offset. An
// element's .0 is accepted only if its first direct text node is mapped.
func (a *FB2Anchors) Resolve(pointer string) (string, int, FB2Anchor, bool) {
	if a == nil {
		return "", 0, FB2Anchor{}, false
	}
	point := strings.LastIndex(pointer, ".")
	if point < 0 {
		return "", 0, FB2Anchor{}, false
	}
	offset, err := strconv.Atoi(pointer[point+1:])
	if err != nil || offset < 0 {
		return "", 0, FB2Anchor{}, false
	}
	path, ok := NormalizeFB2Path(pointer[:point])
	if !ok {
		return "", 0, FB2Anchor{}, false
	}
	rawParts := strings.Split(strings.TrimPrefix(pointer[:point], "/"), "/")
	normalizedParts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	for i, part := range rawParts {
		if !strings.Contains(part, "[") && a.Ambiguous["/"+strings.Join(normalizedParts[:i], "/")+"/"+part] {
			return "", 0, FB2Anchor{}, false
		}
	}
	element := !strings.Contains(path, "/text()[")
	if element {
		if offset != 0 {
			return "", 0, FB2Anchor{}, false
		}
		path += "/text()[1]"
	}
	node, ok := a.Nodes[path]
	if !ok || element && !node.First || offset > utf8.RuneCountInString(node.Text) {
		return "", 0, FB2Anchor{}, false
	}
	return path, offset, node, true
}

func (a *FB2Anchors) Fraction(node FB2Anchor, offset int) float64 {
	if a == nil || a.Total == 0 {
		return 0
	}
	position := offset + node.Start
	for _, length := range a.Lengths[:node.Chapter] {
		position += length
	}
	return float64(position) / float64(a.Total)
}

type fb2Frame struct {
	name, path   string
	counts       map[string]int
	texts        int
	previousText bool
	unsafe       bool
}
type fb2TextDecoder struct {
	*xml.Decoder
	frames     []fb2Frame
	enabled    bool
	rootSeen   bool
	textPath   string
	candidates map[string]string
	invalid    map[string]bool
	first      map[string]bool
	ambiguous  map[string]bool
}

func (d *fb2TextDecoder) Token() (xml.Token, error) {
	if !d.enabled {
		return d.Decoder.Token()
	}
	token, err := d.Decoder.Token()
	if err != nil {
		return nil, err
	}
	d.textPath = ""
	switch t := token.(type) {
	case xml.StartElement:
		if len(d.frames) == 0 {
			if d.rootSeen || t.Name.Local != "FictionBook" {
				return nil, fmt.Errorf("unsupported FB2 root")
			}
			d.rootSeen = true
		}
		if len(d.frames) >= 128 {
			return nil, fmt.Errorf("FB2 anchor nesting limit exceeded")
		}
		path := "/" + t.Name.Local + "[1]"
		unsafe := len(d.frames) >= 128
		if len(d.frames) > 0 {
			p := &d.frames[len(d.frames)-1]
			p.previousText = false
			p.counts[t.Name.Local]++
			if p.counts[t.Name.Local] > 1 {
				d.ambiguous[p.path+"/"+t.Name.Local] = true
			}
			path = p.path + "/" + t.Name.Local + "[" + strconv.Itoa(p.counts[t.Name.Local]) + "]"
			unsafe = unsafe || p.unsafe
		}
		switch t.Name.Local {
		case "FictionBook", "body", "section", "p", "title", "epigraph", "cite", "poem", "stanza", "v", "subtitle", "text-author", "emphasis", "strong", "strikethrough", "sub", "sup":
		default:
			unsafe = true
		}
		for _, attr := range t.Attr {
			if attr.Name.Local == "style" {
				unsafe = true
			}
		}
		d.frames = append(d.frames, fb2Frame{name: t.Name.Local, path: path, counts: map[string]int{}, unsafe: unsafe})
	case xml.EndElement:
		if len(d.frames) > 0 {
			d.frames = d.frames[:len(d.frames)-1]
		}
		if len(d.frames) > 0 {
			d.frames[len(d.frames)-1].previousText = false
		}
	case xml.CharData:
		if !d.enabled || len(d.frames) == 0 {
			return token, nil
		}
		f := &d.frames[len(d.frames)-1]
		value := string(t)
		// Whitespace-only nodes and adjacent CDATA/comment chunks are engine-
		// dependent. Do not publish ambiguous text indexes back to the device.
		if strings.TrimSpace(value) == "" {
			d.invalid[f.path] = true
			return token, nil
		}
		if f.previousText {
			d.invalid[f.path] = true
		}
		f.previousText = true
		f.texts++
		if f.texts > 1 {
			d.ambiguous[f.path+"/text()"] = true
		}
		if f.unsafe || len(d.candidates) >= 100000 {
			return token, nil
		}
		if strings.ContainsAny(value, "\r\n\t") || strings.Contains(value, "  ") || strings.TrimSpace(value) != value || utf8.RuneCountInString(value) > 4096 {
			d.invalid[f.path] = true
			return token, nil
		}
		d.textPath = f.path + "/text()[" + strconv.Itoa(f.texts) + "]"
		if len(d.textPath) > 2048 {
			d.textPath = ""
			return token, nil
		}
		d.candidates[d.textPath] = value
		d.first[d.textPath] = f.texts == 1 && len(f.counts) == 0
	}
	return token, nil
}

func (d *fb2TextDecoder) Skip() error {
	depth := 1
	for depth > 0 {
		token, err := d.Token()
		if err != nil {
			return err
		}
		switch token.(type) {
		case xml.StartElement:
			depth++
		case xml.EndElement:
			depth--
		}
	}
	return nil
}

func (d *fb2TextDecoder) writeText(out *strings.Builder, value string) {
	if d.textPath == "" {
		out.WriteString(escape(value))
		return
	}
	fmt.Fprintf(out, `<span data-fb2-node="%s">%s</span>`, d.textPath, escape(value))
}

// Build the chapter index only after splitting/merging sections is complete.
func (d *fb2TextDecoder) index(text *FB2Text) {
	if !d.enabled {
		return
	}
	a := &FB2Anchors{Nodes: map[string]FB2Anchor{}, Lengths: make([]int, len(text.Chapters)), Ambiguous: d.ambiguous}
	for chapter, ch := range text.Chapters {
		document, err := nethtml.Parse(strings.NewReader("<html><body>" + ch.HTML + "</body></html>"))
		if err != nil {
			continue
		}
		size := 0
		var visit func(*nethtml.Node)
		visit = func(node *nethtml.Node) {
			if node.Type == nethtml.ElementNode {
				for _, attr := range node.Attr {
					if attr.Key == "data-fb2-node" {
						if value, ok := d.candidates[attr.Val]; ok {
							parent := attr.Val[:strings.LastIndex(attr.Val, "/text()")]
							if !d.invalid[parent] {
								a.Nodes[attr.Val] = FB2Anchor{Chapter: chapter, Start: size, Text: value, First: d.first[attr.Val]}
							}
						}
					}
				}
			}
			if node.Type == nethtml.TextNode {
				size += utf8.RuneCountInString(node.Data)
			}
			for child := node.FirstChild; child != nil; child = child.NextSibling {
				visit(child)
			}
		}
		visit(document)
		a.Lengths[chapter] = size
		a.Total += size
	}
	text.Anchors = a
}
