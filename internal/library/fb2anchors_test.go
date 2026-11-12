package library

import (
	"strings"
	"testing"
)

const anchorFB2 = `<FictionBook><body>
<section><title><p>One</p></title><p>Первый 😀 абзац.</p><p>Text <emphasis>inside</emphasis> after.</p></section>
<section><title><p>Two</p></title><p>Second chapter.</p></section>
</body></FictionBook>`

func TestFB2SourceAnchors(t *testing.T) {
	text, err := ParseFB2TextAnchored(strings.NewReader(anchorFB2), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(text.Chapters) != 2 || text.Anchors == nil {
		t.Fatalf("chapters: %+v", text)
	}
	pointer := "/FictionBook/body/section[1]/p[1]/text().8"
	path, offset, node, ok := text.Anchors.Resolve(pointer)
	if !ok || offset != 8 || node.Chapter != 0 || node.Text != "Первый 😀 абзац." {
		t.Fatalf("resolve: %s %d %+v %v", path, offset, node, ok)
	}
	if !strings.Contains(text.Chapters[0].HTML, `data-fb2-node="`+path+`"`) {
		t.Fatal("source anchor not rendered")
	}
	if _, _, _, ok := text.Anchors.Resolve("/FictionBook/body/section/p[1]/text().0"); ok {
		t.Fatal("ambiguous unindexed section accepted")
	}
	if _, _, _, ok := text.Anchors.Resolve("/FictionBook/body/section[1]/p[1]/text().999"); ok {
		t.Fatal("invalid offset accepted")
	}
	if _, _, _, ok := text.Anchors.Resolve("/FictionBook/body/section[1]/p[1].0"); !ok {
		t.Fatal("unambiguous element start not resolved")
	}
	if fraction := text.Anchors.Fraction(node, offset); fraction <= 0 || fraction >= 1 {
		t.Fatalf("fraction: %f", fraction)
	}
}

func TestFB2UnsafeCoordinatesDoNotGetExported(t *testing.T) {
	for _, paragraph := range []string{"<p>line\nwrap</p>", "<p>double  space</p>", "<p>one<!--comment-->two</p>", "<p><code>code text</code></p>", "<p>leading </p>"} {
		text, err := ParseFB2TextAnchored(strings.NewReader("<FictionBook><body><section>"+paragraph+"</section></body></FictionBook>"), nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(text.Anchors.Nodes) != 0 {
			t.Fatalf("unsafe text mapped: %s", paragraph)
		}
	}
}
