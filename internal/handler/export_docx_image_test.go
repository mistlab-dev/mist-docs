package handler

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"os"
	"strings"
	"testing"

	"github.com/c-wind/mist-docs/internal/config"
	"github.com/c-wind/mist-docs/internal/store"
)

// makePNG returns a PNG of the given size.
func makePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	// Solid color so the file is non-trivial but small.
	for x := 0; x < w; x++ {
		img.Set(x, 0, color.RGBA{R: 255, G: 0, B: 0, A: 255})
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png encode: %v", err)
	}
	return buf.Bytes()
}

// TestHtmlToDocx_EmbedImage verifies a fetchable image is embedded as a
// media part with a real relationship (issue #39 acceptance: success path).
func TestHtmlToDocx_EmbedImage(t *testing.T) {
	pngBytes := makePNG(t, 40, 30)
	dataURI := "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes)

	input := `<p><img src="` + dataURI + `" alt="红色条"></p>`
	data, err := htmlToDocx(input)
	if err != nil {
		t.Fatalf("htmlToDocx embed: %v", err)
	}
	docXML, relsXML := inspectDocx(t, data)

	if !strings.Contains(docXML, "<w:r><w:drawing>") {
		t.Errorf("B2: expected <w:r><w:drawing> in document.xml: %s", docXML)
	}
	if !strings.Contains(relsXML, "relationships/image") {
		t.Errorf("expected image relationship: %s", relsXML)
	}
	if strings.Contains(relsXML, "TargetMode") {
		t.Errorf("image rel must be internal, not external: %s", relsXML)
	}
	// The media part must be present in the zip with the right name.
	if !zipHasPartWithPrefix(data, "word/media/image1.") {
		t.Errorf("expected word/media/image1.* part")
	}
	// Extent must be non-zero and within the text column (EMU).
	if strings.Contains(docXML, `cx="0"`) || strings.Contains(docXML, `cy="0"`) {
		t.Errorf("zero extent in drawing: %s", docXML)
	}
}

// TestHtmlToDocx_ImageFallback verifies a non-embeddable image degrades to
// alt text (and a link when the src is a safe URL) while the docx still builds
// (issue #39 acceptance: failure path must not fail the export).
func TestHtmlToDocx_ImageFallback(t *testing.T) {
	cases := []struct {
		name string
		html string
		want string
	}{
		{
			name: "external http image -> alt text kept",
			html: `<p><img src="https://example.com/missing.png" alt="外部图"></p>`,
			want: "外部图",
		},
		{
			name: "external image without alt -> src kept",
			html: `<p><img src="https://example.com/missing.png"></p>`,
			want: "https://example.com/missing.png",
		},
		{
			name: "javascript: image -> no link, text kept",
			html: `<p><img src="javascript:alert(1)" alt="坏图"></p>`,
			want: "坏图",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := htmlToDocx(tc.html)
			if err != nil {
				t.Fatalf("export must succeed on fallback: %v", err)
			}
			docXML, relsXML := inspectDocx(t, data)
			if !strings.Contains(docXML, tc.want) {
				t.Errorf("expected %q in document.xml: %s", tc.want, docXML)
			}
			if strings.Contains(docXML, "<w:drawing>") {
				t.Errorf("fallback must not produce a drawing: %s", docXML)
			}
			if strings.Contains(relsXML, "javascript:") {
				t.Errorf("javascript: must not become a relationship")
			}
		})
	}
}

// TestDefaultDocxImageFetcher_SameOriginMedia verifies a signed same-origin
// media path is read from disk (no HTTP hop), and that traversal/foreign
// schemes are refused.
func TestDefaultDocxImageFetcher_SameOriginMedia(t *testing.T) {
	root := t.TempDir()
	oldRoot := config.C.Storage.Root
	config.C.Storage.Root = root
	defer func() { config.C.Storage.Root = oldRoot }()

	pngBytes := makePNG(t, 8, 8)
	// Write to <root>/team1/media/a.png (the fetcher path shape).
	mediaDir := root + "/team1/media"
	if err := mkdirAll(mediaDir); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := writeFile(mediaDir+"/a.png", pngBytes); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Same-origin signed path -> embedded.
	validSig := mediaSig("team1", "a.png")
	data, mt, ok := defaultDocxImageFetcher("/api/media/team1/a.png?sig=" + validSig)
	if !ok || len(data) == 0 || mt != "image/png" {
		t.Fatalf("expected same-origin media to load, ok=%v mt=%q", ok, mt)
	}
	// store.RootPath must be honoured (sanity that we used the temp root).
	if store.RootPath() != root {
		t.Errorf("RootPath=%q want %q", store.RootPath(), root)
	}

	// Traversal must be refused.
	if _, _, ok := defaultDocxImageFetcher("/api/media/../../etc/passwd"); ok {
		t.Error("traversal must be refused")
	}
	// Foreign scheme / host must be refused.
	if _, _, ok := defaultDocxImageFetcher("https://evil.example/x.png"); ok {
		t.Error("external URL must be refused (SSRF guard)")
	}
	if _, _, ok := defaultDocxImageFetcher("file:///etc/passwd"); ok {
		t.Error("file: scheme must be refused")
	}
}

// TestDocxImageExtent covers scale-down to the text column.
func TestDocxImageExtent(t *testing.T) {
	// Wide image: width capped at column, height proportional.
	w, h, ok := docxImageExtent(makePNG(t, 2000, 100))
	if !ok {
		t.Fatal("extent not computed")
	}
	const columnEMU = int64(9026) * 9525 // 9026 twips at 96dpi/9525emu
	if w > columnEMU {
		t.Errorf("width %d exceeds column %d", w, columnEMU)
	}
	if h <= 0 || h >= w {
		t.Errorf("height %d should be positive and < width %d", h, w)
	}

	// Tiny image: untouched.
	w2, h2, ok := docxImageExtent(makePNG(t, 10, 20))
	if !ok || w2 != 10*9525 || h2 != 20*9525 {
		t.Errorf("tiny extent = %d,%d,%v", w2, h2, ok)
	}
}

func mkdirAll(path string) error            { return os.MkdirAll(path, 0o755) }
func writeFile(path string, b []byte) error { return os.WriteFile(path, b, 0o644) }

// zipHasPartWithPrefix reports whether the docx zip contains an entry whose
// name starts with prefix.
func zipHasPartWithPrefix(data []byte, prefix string) bool {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return false
	}
	for _, f := range r.File {
		if strings.HasPrefix(f.Name, prefix) {
			return true
		}
	}
	return false
}

// TestHtmlToDocx_TopLevelBlockImage verifies that an <img> element that is a direct
// block sibling of <p> (TipTap output with inline: false) is rendered in its own paragraph
// and not silently dropped (fixes PR #50 review B1).
func TestHtmlToDocx_TopLevelBlockImage(t *testing.T) {
	pngBytes := makePNG(t, 20, 20)
	dataURI := "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes)

	// Block-level <img> before and between <p>
	input := `<img src="` + dataURI + `" alt="顶层图片"><p>段落内容</p>`
	data, err := htmlToDocx(input)
	if err != nil {
		t.Fatalf("htmlToDocx block image: %v", err)
	}
	docXML, relsXML := inspectDocx(t, data)

	if !strings.Contains(docXML, "<w:r><w:drawing>") {
		t.Errorf("B1: top-level img must produce a drawing in its own paragraph: %s", docXML)
	}
	if !strings.Contains(docXML, "段落内容") {
		t.Errorf("sibling paragraph content missing: %s", docXML)
	}
	if !strings.Contains(relsXML, "relationships/image") {
		t.Errorf("missing image relationship: %s", relsXML)
	}
}

// TestDefaultDocxImageFetcher_CrossTeamOrInvalidSig verifies that foreign-team or
// unsigned media references degrade and are refused by the fetcher (fixes PR #50 review B3).
func TestDefaultDocxImageFetcher_CrossTeamOrInvalidSig(t *testing.T) {
	root := t.TempDir()
	oldRoot := config.C.Storage.Root
	oldSecret := config.C.JWT.Secret
	config.C.Storage.Root = root
	config.C.JWT.Secret = "test-secret-media-sig"
	defer func() {
		config.C.Storage.Root = oldRoot
		config.C.JWT.Secret = oldSecret
	}()

	pngBytes := makePNG(t, 8, 8)
	mediaDir := root + "/teamB/media"
	if err := mkdirAll(mediaDir); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := writeFile(mediaDir+"/secret.png", pngBytes); err != nil {
		t.Fatalf("write: %v", err)
	}

	// 1. Missing signature -> refused
	if _, _, ok := defaultDocxImageFetcher("/api/media/teamB/secret.png"); ok {
		t.Error("B3: unsigned media path must be refused")
	}

	// 2. Forged / invalid signature -> refused
	if _, _, ok := defaultDocxImageFetcher("/api/media/teamB/secret.png?sig=invalidhexsig1234"); ok {
		t.Error("B3: invalid signature must be refused")
	}

	// 3. Genuine signature -> accepted
	validSig := mediaSig("teamB", "secret.png")
	data, mt, ok := defaultDocxImageFetcher("/api/media/teamB/secret.png?sig=" + validSig)
	if !ok || len(data) == 0 || mt != "image/png" {
		t.Errorf("B3: genuine signature should be accepted, ok=%v mt=%q", ok, mt)
	}
}

// TestHtmlToDocx_BlockquoteImage verifies an <img> inside a <blockquote> is
// rendered as a quoted drawing instead of being silently dropped
// (PR #50 review B1 prime; TipTap blockquote content is block+).
func TestHtmlToDocx_BlockquoteImage(t *testing.T) {
	pngBytes := makePNG(t, 24, 16)
	dataURI := "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes)

	input := `<blockquote><img src="` + dataURI + `" alt="引用图"><p>引用正文</p></blockquote>`
	data, err := htmlToDocx(input)
	if err != nil {
		t.Fatalf("htmlToDocx blockquote image: %v", err)
	}
	docXML, relsXML := inspectDocx(t, data)

	if !strings.Contains(docXML, "<w:r><w:drawing>") {
		t.Errorf("blockquote img must produce a drawing: %s", docXML)
	}
	if !strings.Contains(docXML, "引用正文") {
		t.Errorf("blockquote paragraph content missing: %s", docXML)
	}
	if !strings.Contains(relsXML, "relationships/image") {
		t.Errorf("missing image relationship: %s", relsXML)
	}
	// The quoted drawing paragraph must carry the quote indent + shading.
	if !strings.Contains(docXML, "w:ind") || !strings.Contains(docXML, "F0F4F8") {
		t.Errorf("quoted image paragraph should keep indent/shading: %s", docXML)
	}
}
