package tests

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func uploadImage(t *testing.T) (filename, url string) {
	t.Helper()
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	part, _ := mw.CreateFormFile("file", "pic.png")
	part.Write([]byte("\x89PNG fake image"))
	mw.Close()
	req := httptest.NewRequest("POST", teamPath("/upload"), body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+adminToken)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("upload: %d %s", w.Code, w.Body.String())
	}
	d := parseJSON(t, w)["data"].(map[string]interface{})
	return getString(d["filename"]), getString(d["url"])
}

func anonGet(path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
	return w
}

// Images in documents are <img src> without a login header, and share pages
// have no login at all: media referenced from a document is served through a
// signed URL that only the server hands out.
func TestMediaSignedURLs(t *testing.T) {
	fn, url := uploadImage(t)
	defer request("DELETE", teamPath("/media/"+fn), nil, adminToken)
	if !strings.HasPrefix(url, "/api/media/"+teamID+"/"+fn+"?sig=") {
		t.Fatalf("upload url = %q, want a signed /api/media URL", url)
	}

	w := anonGet(url)
	if w.Code != 200 || w.Body.String() != "\x89PNG fake image" {
		t.Fatalf("signed url without login: %d %q", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Header().Get("Content-Security-Policy"), "sandbox") || w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("media must be served sandboxed, headers %v", w.Header())
	}
	for _, bad := range []string{
		"/api/media/" + teamID + "/" + fn,                             // no signature
		"/api/media/" + teamID + "/" + fn + "?sig=00",                 // wrong
		"/api/media/other-team/" + fn + url[strings.Index(url, "?"):], // signature of another file
		"/api/media/" + teamID + "/..%2Fetc" + url[strings.Index(url, "?"):],
	} {
		if w := anonGet(bad); w.Code != http.StatusForbidden && w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
			t.Errorf("%s: %d, want refused", bad, w.Code)
		}
	}

	// Content stored with the plain (authed) form is signed on the way out,
	// for the doc's own team only.
	doc := createTestDoc(t, adminToken, "signed-media")
	defer request("DELETE", teamPath("/documents/"+doc), nil, adminToken)
	html := `<p><img src="/api/teams/` + teamID + `/media/` + fn + `"></p><p><img src="/api/teams/someone-else/media/x.png"></p>`
	saveContent(t, doc, html)

	w = request("GET", teamPath("/documents/"+doc+"/content"), nil, adminToken)
	content := getString(parseJSON(t, w)["data"].(map[string]interface{})["content"])
	if !strings.Contains(content, `src="`+url+`"`) {
		t.Fatalf("content not signed: %s", content)
	}
	if !strings.Contains(content, `/api/teams/someone-else/media/x.png`) || strings.Contains(content, "/api/media/someone-else") {
		t.Fatalf("another team's media must not be signed: %s", content)
	}

	// Share page (no login) gets the same signed URLs.
	w = request("POST", teamPath("/documents/"+doc+"/share"), map[string]interface{}{}, adminToken)
	token := getString(parseJSON(t, w)["data"].(map[string]interface{})["token"])
	w = anonGet("/api/s/" + token)
	if w.Code != 200 || !strings.Contains(getString(parseJSON(t, w)["content"]), url) {
		t.Fatalf("share content: %d %s", w.Code, w.Body.String())
	}
}

// The old uploads/ endpoint used to be public.
func TestLegacyFilesRequireLogin(t *testing.T) {
	if w := anonGet("/api/files/anything.png"); w.Code != http.StatusUnauthorized {
		t.Fatalf("/api/files without login: %d, want 401", w.Code)
	}
}
