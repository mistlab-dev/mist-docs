package handler

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

// TestRestrictMediaToTeam: a foreign team's media reference is replaced by
// its bare file name. The docx converter cannot embed a bare file name
// (the image fetcher accepts only data: URIs and signed /api/media paths),
// so the image degrades to its alt text — and no "#" placeholder or
// cross-team URL is left in the exported document.
func TestRestrictMediaToTeam(t *testing.T) {
	html := `<p><img src="/api/media/other-team/secret.png" alt="外队图"></p>` +
		`<p><img src="/api/media/my-team/ok.png" alt="本队图"></p>`

	out := restrictMediaToTeam("my-team", html)

	// The foreign reference is reduced to the bare file name.
	if strings.Contains(out, "secret.png?sig") || strings.Contains(out, "other-team") {
		t.Errorf("foreign media reference survived: %s", out)
	}
	if !strings.Contains(out, `src="secret.png"`) {
		t.Errorf("foreign media should degrade to the bare file name: %s", out)
	}
	// The own-team reference is untouched.
	if !strings.Contains(out, "/api/media/my-team/ok.png") {
		t.Errorf("own-team media must be kept: %s", out)
	}
	// No "#" placeholder is left behind.
	if strings.Contains(out, `src="#"`) {
		t.Errorf("foreign media must not become a bare #: %s", out)
	}
}

// TestRestrictMediaToTeamLeavesOtherHTML: the rewrite only touches media
// references; ordinary markup survives unchanged.
func TestRestrictMediaToTeamLeavesOtherHTML(t *testing.T) {
	html := `<p>正文 <a href="/api/teams/x/documents/y">链接</a></p>`
	if got := restrictMediaToTeam("my-team", html); got != html {
		t.Errorf("non-media HTML changed: %s", got)
	}
}

// TestDocxImageDimensionsOK: images with an out-of-range declared canvas are
// refused, so a tiny PNG cannot claim a 1x100000 canvas and blow up the
// layout in Word.
func TestDocxImageDimensionsOK(t *testing.T) {
	// A normal small PNG is fine.
	small := makePNG(t, 40, 30)
	if !docxImageDimensionsOK(small) {
		t.Error("a 40x30 PNG should be accepted")
	}

	// A PNG whose header declares an out-of-range height is refused. The
	// bytes are crafted by hand: the IHDR chunk carries the dimensions and
	// the decoder only reads the header.
	huge := pngWithDimensions(t, 1, docxMaxImageDimension+1)
	if docxImageDimensionsOK(huge) {
		t.Errorf("a %d px tall PNG should be refused", docxMaxImageDimension+1)
	}
	wide := pngWithDimensions(t, docxMaxImageDimension+1, 1)
	if docxImageDimensionsOK(wide) {
		t.Errorf("a %d px wide PNG should be refused", docxMaxImageDimension+1)
	}

	// Garbage is refused rather than accepted.
	if docxImageDimensionsOK([]byte("not an image")) {
		t.Error("garbage should be refused")
	}
}

// pngWithDimensions builds a PNG whose header declares the given size. The
// pixel data is not filled in — only the header matters for DecodeConfig.
func pngWithDimensions(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 1, G: 2, B: 3, A: 255})
	// Encode a real 1x1 PNG, then rewrite the IHDR width/height fields.
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatalf("png encode: %v", err)
	}
	raw := b.Bytes()
	// IHDR payload starts right after the 8-byte signature + 4-byte length +
	// 4-byte "IHDR" tag.
	const ihdrPayload = 8 + 4 + 4
	raw[ihdrPayload+0] = byte(w >> 24)
	raw[ihdrPayload+1] = byte(w >> 16)
	raw[ihdrPayload+2] = byte(w >> 8)
	raw[ihdrPayload+3] = byte(w)
	raw[ihdrPayload+4] = byte(h >> 24)
	raw[ihdrPayload+5] = byte(h >> 16)
	raw[ihdrPayload+6] = byte(h >> 8)
	raw[ihdrPayload+7] = byte(h)
	return raw
}

// TestFetchDocxDataURIRejectsOversizedImage: the data: URI path applies the
// same dimension cap as the media path.
func TestFetchDocxDataURIRejectsOversizedImage(t *testing.T) {
	huge := pngWithDimensions(t, 1, docxMaxImageDimension+1)
	uri := "data:image/png;base64," + base64.StdEncoding.EncodeToString(huge)
	if _, _, ok := fetchDocxDataURI(uri); ok {
		t.Error("an oversized image must not be embedded")
	}

	small := makePNG(t, 40, 30)
	uri = "data:image/png;base64," + base64.StdEncoding.EncodeToString(small)
	if _, _, ok := fetchDocxDataURI(uri); !ok {
		t.Error("a normal image must still be embedded")
	}
}
