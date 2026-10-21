package handler

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/c-wind/mist-docs/internal/database"
	"github.com/gin-gonic/gin"
)

// maxClientExportBytes caps the HTML a client may post for conversion. The
// body is user content, so it needs a ceiling: a huge paste would otherwise
// be turned into a huge package server-side.
//
// 32MB, not 8: a diagram is posted as a base64 data: URI (4/3 expansion) and
// the per-image ceiling in the docx pipeline is 10MB, so several diagrams in
// one document must still fit. The frontend caps each rendered diagram at 2MB.
const maxClientExportBytes = 32 << 20 // 32 MB

// TeamExportDocumentPost POST /teams/:team_id/documents/:id/export?format=docx
//
// Converts HTML supplied by the client instead of the stored copy. This is
// what makes client-side rendering possible (#56): the browser draws Mermaid
// diagrams to PNG data: URIs and posts the patched HTML, so the diagram
// reaches the .docx as a picture. The server still owns the conversion.
//
// Semantics deliberately mirror the GET export: the same read permission is
// required (a viewer who may read the document may export it), docx stays
// free of any plan gate, and a sheet is still refused explicitly.
func TeamExportDocumentPost(c *gin.Context) {
	docID := c.Param("id")
	if !requireDoc(c, docID, "read", true) {
		return
	}
	format := strings.ToLower(strings.TrimSpace(c.DefaultQuery("format", "docx")))
	if format != "docx" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该接口仅支持 format=docx，可选: docx"})
		return
	}

	var title, docType string
	err := database.DB.QueryRow(
		`SELECT title, type FROM md_documents WHERE id=? AND team_id=? AND status=1`,
		docID, getTeamID(c)).Scan(&title, &docType)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文档不存在"})
		return
	}
	// A sheet's payload is JSON, not HTML: refuse instead of silently
	// feeding it to the converter (same rule as the GET export).
	if docType == "sheet" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sheet 暂不支持导出 docx"})
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, maxClientExportBytes))
	if err != nil {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "导出内容过大"})
		return
	}
	html := strings.TrimSpace(string(body))
	if html == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "导出内容为空"})
		return
	}

	teamID := getTeamID(c)
	// The posted HTML may reference media of the team. Re-sign our own
	// team's files and neutralise every other team's references: a client
	// must not be able to pull another team's images into an export.
	html = restrictMediaToTeam(teamID, signMediaURLs(teamID, html))

	if !serveDocumentExport(c, title, html, format) {
		return
	}
	userName, _ := c.Get("username")
	audit(c, "export", "document", docID, title, fmt.Sprintf("%s 导出文档 (%s, %s, 客户端渲染)", userName, format, docType))
}

// restrictMediaToTeam removes media references that do not belong to teamID.
// Own-team files keep their (re-signed) URL; foreign ones are replaced by
// their bare file name.
//
// The replacement matters: the docx converter falls back to the alt text
// (or, when there is none, the src) when it cannot embed an image. Leaving a
// "#" there produced a lone "#" character in the exported document, and
// leaving the full URL would keep a reference to another team's file. The
// bare file name is neither resolvable by the image fetcher (it accepts
// only data: URIs and signed /api/media paths) nor a URL — so the image
// degrades to its alt text, and the file name still shows when there is no
// alt text.
func restrictMediaToTeam(teamID, html string) string {
	if teamID == "" || html == "" {
		return html
	}
	return mediaRefRe.ReplaceAllStringFunc(html, func(m string) string {
		p := mediaRefRe.FindStringSubmatch(m)
		team := p[1]
		if team == "" {
			team = p[2]
		}
		if team == teamID {
			return m
		}
		return p[3]
	})
}
