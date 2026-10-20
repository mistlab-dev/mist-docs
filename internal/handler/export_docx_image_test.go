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

	if !strings.Contains(docXML, "<w:drawing>") {
		t.Errorf("expected w:drawing in document.xml: %s", docXML)
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
	data, mt, ok := defaultDocxImageFetcher("/api/media/team1/a.png?sig=abc")
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
