package handler

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/c-wind/mist-docs/internal/store"
	golanghtml "golang.org/x/net/html"
)

// docxImageFetcher turns an <img src> into embeddable image bytes. Returning
// ok=false makes the converter degrade gracefully (alt text / link) instead of
// failing the whole export.
type docxImageFetcher func(src string) (data []byte, contentType string, ok bool)

// docxMaxImageBytes caps one embedded image so a huge upload cannot blow up
// the generated document.
const docxMaxImageBytes = 10 << 20 // 10 MB

// docxImage is one embedded picture inside the package (word/media/imageN.ext).
type docxImage struct {
	data      []byte
	mediaType string
	name      string // e.g. image1.png
	rID       string
}

// defaultDocxImageFetcher embeds only first-party sources: data: URIs and our
// own signed /api/media/... paths (read straight from disk, no HTTP hop). Any
// other URL — external http(s), protocol-relative, javascript: — is refused so
// exporting a document can never become an SSRF primitive. External images
// degrade to alt text instead of blocking the export.
func defaultDocxImageFetcher(src string) ([]byte, string, bool) {
	src = strings.TrimSpace(src)
	if src == "" {
		return nil, "", false
	}

	if strings.HasPrefix(strings.ToLower(src), "data:") {
		return fetchDocxDataURI(src)
	}

	return fetchDocxLocalMedia(src)
}

// fetchDocxDataURI decodes a base64 data: URI. Anything not base64, not an
// image media type, or oversized is refused.
func fetchDocxDataURI(src string) ([]byte, string, bool) {
	comma := strings.Index(src, ",")
	if comma < 0 {
		return nil, "", false
	}
	meta := src[len("data:"):comma]
	if !strings.HasSuffix(strings.ToLower(meta), ";base64") {
		return nil, "", false
	}
	mediaType := strings.ToLower(strings.Split(meta, ";")[0])
	if !docxImageContentTypeOK(mediaType) {
		return nil, "", false
	}
	payload := src[comma+1:]
	// Size-check the encoded form before decoding so a huge URI cannot balloon.
	if len(payload) > docxMaxImageBytes*4/3+4 {
		return nil, "", false
	}
	data, err := base64.StdEncoding.DecodeString(payload)
	if err != nil || len(data) == 0 || len(data) > docxMaxImageBytes {
		return nil, "", false
	}
	return data, mediaType, true
}

// fetchDocxLocalMedia resolves a signed same-origin media path to the file on
// disk. The path shape is /api/media/{team}/{filename}?sig=...; PublicGetMedia
// verifies that signature at request time, so we do not re-check the HMAC here.
// What matters is that we never escape the storage root: safeBaseName rejects
// traversal and the join stays under store.RootPath()/team/media/.
func fetchDocxLocalMedia(src string) ([]byte, string, bool) {
	u, err := url.Parse(src)
	if err != nil {
		return nil, "", false
	}
	if u.Scheme != "" || u.Host != "" {
		// Absolute http(s) or any other scheme: not first-party, refuse.
		return nil, "", false
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	// Accept both /api/media/{team}/{file} and /api/teams/{team}/media/{file}.
	var team string
	switch {
	case len(parts) == 4 && parts[0] == "api" && parts[1] == "media":
		team = parts[2]
	case len(parts) == 5 && parts[0] == "api" && parts[1] == "teams" && parts[3] == "media":
		team = parts[2]
	default:
		return nil, "", false
	}
	// The filename is always the last path element in both shapes.
	file := parts[len(parts)-1]
	if team == "" || file == "" {
		return nil, "", false
	}
	team, ok1 := safeBaseName(team)
	file, ok2 := safeBaseName(file)
	if !ok1 || !ok2 {
		return nil, "", false
	}
	path := filepath.Join(store.RootPath(), team, "media", file)
	info, err := os.Stat(path)
	if err != nil || info.IsDir() || info.Size() == 0 || info.Size() > docxMaxImageBytes {
		return nil, "", false
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return nil, "", false
	}
	mediaType := docxSniffImageType(data)
	if mediaType == "" {
		return nil, "", false
	}
	return data, mediaType, true
}

// docxImageContentTypeOK reports whether an image media type has a matching
// [Content_Types].xml default in the generated package.
func docxImageContentTypeOK(mediaType string) bool {
	switch mediaType {
	case "image/png", "image/jpeg", "image/gif":
		return true
	}
	return false
}

// docxSniffImageType identifies a PNG/JPEG/GIF by magic bytes.
func docxSniffImageType(data []byte) string {
	switch {
	case len(data) >= 8 && string(data[:8]) == "\x89PNG\r\n\x1a\n":
		return "image/png"
	case len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF:
		return "image/jpeg"
	case len(data) >= 6 && (string(data[:6]) == "GIF87a" || string(data[:6]) == "GIF89a"):
		return "image/gif"
	}
	return ""
}

// docxImageExtent sizes a drawing so it fits the text column. Word measures
// drawings in EMU (9525 per pixel at 96 DPI). Content width is the page minus
// both margins: (11906 - 1440*2) twips at 1440 twips/inch. Wider images scale
// down proportionally; a square cap keeps very tall images sane too.
func docxImageExtent(data []byte) (cx, cy int64, ok bool) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return 0, 0, false
	}
	const emuPerPx = 9525.0
	const contentWidthTwips = 9026.0
	const twipsPerInch = 1440.0
	const pxPerInch = 96.0
	max := int64(contentWidthTwips / twipsPerInch * pxPerInch * emuPerPx)

	w := int64(float64(cfg.Width) * emuPerPx)
	h := int64(float64(cfg.Height) * emuPerPx)
	if w > max {
		h = h * max / w
		w = max
	}
	if h > max {
		w = w * max / h
		h = max
	}
	if w <= 0 || h <= 0 {
		return 0, 0, false
	}
	return w, h, true
}

// docxImageExtension maps a media type to the extension used inside the
// package (word/media/image1.png).
func docxImageExtension(mediaType string) string {
	switch mediaType {
	case "image/png":
		return "png"
	case "image/jpeg":
		return "jpg"
	case "image/gif":
		return "gif"
	}
	return "png"
}

// processImage renders an <img> as a Word drawing, or degrades to its alt text
// (linked to src when the src is a safe URL) when the image cannot be embedded.
// Either way the document stays valid and htmlToDocx still succeeds.
func (dc *docxConverter) processImage(n *golanghtml.Node, st docxInlineState) {
	var src, alt string
	for _, a := range n.Attr {
		switch a.Key {
		case "src":
			src = strings.TrimSpace(a.Val)
		case "alt":
			alt = a.Val
		}
	}

	data, mediaType, got := defaultDocxImageFetcher(src)
	var cx, cy int64
	if got {
		cx, cy, got = docxImageExtent(data)
	}
	if !got {
		dc.writeImageFallback(src, alt, st)
		return
	}

	dc.drawingIdx++
	name := fmt.Sprintf("image%d.%s", dc.drawingIdx, docxImageExtension(mediaType))
	relID := dc.nextRelID()
	dc.images = append(dc.images, docxImage{data: data, mediaType: mediaType, name: name, rID: relID})
	dc.rels = append(dc.rels, docxRel{id: relID, target: "media/" + name})

	dc.body.WriteString(fmt.Sprintf(
		`<w:drawing><wp:inline distT="0" distB="0" distL="0" distR="0" xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"><wp:extent cx="%d" cy="%d"/><wp:docPr id="%d" name="%s"/><a:graphic xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture"><pic:pic xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture"><pic:nvPicPr><pic:cNvPr id="%d" name="%s"/><pic:cNvPicPr/></pic:nvPicPr><pic:blipFill><a:blip r:embed="%s"/><a:stretch><a:fillRect/></a:stretch></pic:blipFill><pic:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></pic:spPr></pic:pic></a:graphicData></a:graphic></wp:inline></w:drawing>`,
		cx, cy, dc.drawingIdx, name, dc.drawingIdx, name, relID, cx, cy))
}

// writeImageFallback renders an image we could not embed: the alt text, linked
// to src when the src is a safe URL, otherwise plain text (the raw src when
// there is no alt). Nothing is silently dropped.
func (dc *docxConverter) writeImageFallback(src, alt string, st docxInlineState) {
	text := strings.TrimSpace(alt)
	if text == "" {
		text = strings.TrimSpace(src)
	}
	if text == "" {
		return
	}
	fb := docxInlineState{bold: st.bold, italic: st.italic, underline: st.underline, strike: st.strike}
	if safeHref(src) {
		fb.href = src
		fb.relID = dc.nextRelID()
		dc.rels = append(dc.rels, docxRel{id: fb.relID, target: src, external: true})
	}
	dc.writeRun(text, fb)
}
