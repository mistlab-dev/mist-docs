package handler

import (
	"html"
	"net/url"
	"regexp"
	"strings"
	"unicode"

	"github.com/c-wind/mist-docs/internal/config"
)

// defaultDocsWebOrigin is the public MistDocs site. MistTerm opens this when
// a search hit has no absolute url of its own. server.base_url overrides it
// for a self-hosted site; anything that is not an http(s) origin is ignored
// so a bad config cannot emit a link the client would refuse.
const defaultDocsWebOrigin = "https://docs.mistlab.dev"

// paragraphHeadingRe captures the inner HTML of h1–h6. The hit's slug is the
// nearest heading that starts at or before the keyword.
var paragraphHeadingRe = regexp.MustCompile(`(?is)<h[1-6]\b[^>]*>(.*?)</h\s*[1-6]\s*>`)

// docsWebOrigin is the absolute site root used in paragraph urls.
func docsWebOrigin() string {
	if origin, ok := sanitizeDocsOrigin(config.C.Server.BaseURL); ok {
		return origin
	}
	return defaultDocsWebOrigin
}

// sanitizeDocsOrigin accepts an absolute http(s) origin with no userinfo,
// path, query, or fragment. A trailing slash is allowed and dropped.
func sanitizeDocsOrigin(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil {
		return "", false
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return "", false
	}
	if (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return "", false
	}
	return u.Scheme + "://" + u.Host, true
}

// paragraphAnchor is the fragment for the heading that contains or precedes
// the first case-insensitive keyword hit. Empty when the keyword is missing
// or that heading has no usable text. Script and style blocks are ignored.
func paragraphAnchor(body, keyword string) string {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" || body == "" {
		return ""
	}
	masked := maskScriptStyle(body)
	idx := indexFoldRune(masked, keyword)
	if idx < 0 {
		return ""
	}
	slug := ""
	for _, m := range paragraphHeadingRe.FindAllStringSubmatchIndex(masked, -1) {
		if len(m) < 4 || m[0] > idx {
			break
		}
		slug = paragraphSlug(masked[m[2]:m[3]])
	}
	return slug
}

// paragraphSlug turns heading text into a fragment MistTerm will append.
// Letters (including CJK), digits, and underscores are kept; everything else
// collapses to a single hyphen. The result is at most 128 runes and contains
// none of the characters the client rejects (# ? & / \ and whitespace).
func paragraphSlug(text string) string {
	text = htmlUnescapeHeading(text)
	text = strings.ToLower(text)
	var b strings.Builder
	prevHyphen := false
	for _, r := range text {
		if isSlugRune(r) {
			b.WriteRune(r)
			prevHyphen = false
			continue
		}
		if !prevHyphen && b.Len() > 0 {
			b.WriteByte('-')
			prevHyphen = true
		}
	}
	s := strings.Trim(b.String(), "-")
	runes := []rune(s)
	if len(runes) > 128 {
		s = strings.Trim(string(runes[:128]), "-")
	}
	return s
}

func htmlUnescapeHeading(text string) string {
	text = htmlTagRe.ReplaceAllString(stripScriptStyle(text), " ")
	return html.UnescapeString(text)
}

func isSlugRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsNumber(r)
}

// paragraphDocURL is the absolute page MistTerm opens. slug, when set, is
// already fragment-safe and is appended as #slug.
func paragraphDocURL(docID, slug string) string {
	docID = strings.TrimSpace(docID)
	if !safeDocPathID(docID) {
		return ""
	}
	u := docsWebOrigin() + "/docs/" + url.PathEscape(docID)
	if slug != "" {
		u += "#" + slug
	}
	return u
}

func safeDocPathID(id string) bool {
	if id == "" || len(id) > 128 {
		return false
	}
	for _, r := range id {
		if r > unicode.MaxASCII || (!unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' && r != '_') {
			return false
		}
	}
	return true
}

// maskScriptStyle replaces script/style blocks with spaces of the same byte
// length so later indexes still line up with the original document.
func maskScriptStyle(s string) string {
	s = scriptRe.ReplaceAllStringFunc(s, blankSameLen)
	return styleRe.ReplaceAllStringFunc(s, blankSameLen)
}

func blankSameLen(m string) string {
	return strings.Repeat(" ", len(m))
}
