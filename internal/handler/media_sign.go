package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"path/filepath"
	"regexp"

	"github.com/c-wind/mist-docs/internal/config"
	"github.com/c-wind/mist-docs/internal/store"
	"github.com/gin-gonic/gin"
)

// Images in a document are <img src> tags, which cannot send the
// Authorization header, and share pages have no login at all. So media
// referenced from document HTML is served through a signed URL:
//
//	/api/media/{team}/{file}?sig=HMAC(secret, team/file)
//
// The signature is a capability for that one file (like the random file
// name itself), it does not expire, and it is only handed out to someone who
// can already read a document of that team (content, version, export, share).

func mediaSig(teamID, filename string) string {
	m := hmac.New(sha256.New, []byte("mist-docs/media-url/v1\x00"+config.C.JWT.Secret))
	m.Write([]byte(teamID + "/" + filename))
	return hex.EncodeToString(m.Sum(nil)[:16])
}

// signedMediaURL is the public, signed address of a team media file.
func signedMediaURL(teamID, filename string) string {
	return "/api/media/" + teamID + "/" + filename + "?sig=" + mediaSig(teamID, filename)
}

// Either form, with or without an old signature: both are (re)signed.
var mediaRefRe = regexp.MustCompile(`/api/(?:teams/([A-Za-z0-9_.-]+)/media|media/([A-Za-z0-9_.-]+))/([A-Za-z0-9_.-]+)(\?sig=[0-9a-f]*)?`)

// signMediaURLs rewrites references to teamID's media in document HTML into
// signed URLs. Other teams' media is left alone, so a document cannot be used
// to obtain signatures for files of a team its reader does not belong to.
func signMediaURLs(teamID, html string) string {
	if teamID == "" || html == "" {
		return html
	}
	return mediaRefRe.ReplaceAllStringFunc(html, func(m string) string {
		p := mediaRefRe.FindStringSubmatch(m)
		team := p[1]
		if team == "" {
			team = p[2]
		}
		name, ok := safeBaseName(p[3])
		if team != teamID || !ok {
			return m
		}
		return signedMediaURL(team, name)
	})
}

// setMediaHeaders keeps an uploaded HTML/SVG file from running script in our
// origin when opened directly.
func setMediaHeaders(c *gin.Context) {
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Security-Policy", "default-src 'none'; img-src 'self' data:; style-src 'unsafe-inline'; sandbox")
}

// PublicGetMedia GET /api/media/:team_id/:filename?sig=
func PublicGetMedia(c *gin.Context) {
	teamID, ok1 := safeBaseName(c.Param("team_id"))
	filename, ok2 := safeBaseName(c.Param("filename"))
	if !ok1 || !ok2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文件名无效"})
		return
	}
	if !hmac.Equal([]byte(c.Query("sig")), []byte(mediaSig(teamID, filename))) {
		c.JSON(http.StatusForbidden, gin.H{"error": "链接无效"})
		return
	}
	path := filepath.Join(store.RootPath(), teamID, "media", filename)
	setMediaHeaders(c)
	c.Header("Cache-Control", "private, max-age=86400")
	c.File(path)
}
