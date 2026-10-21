package handler

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/c-wind/mist-docs/internal/database"
	"github.com/c-wind/mist-docs/internal/service"
	"github.com/gin-gonic/gin"
)

// maxBatchExportDocs caps how many documents one batch request may carry.
const maxBatchExportDocs = 200

// maxBatchExportBody caps the JSON request body of a batch export. The body
// holds one HTML blob per document (a client-rendered diagram is a base64
// data: URI, so it is large), so the ceiling is generous but finite.
const maxBatchExportBody = 64 << 20 // 64 MB

// maxBatchExportOutput caps the total size of the produced zip. The
// production box has 7.5GB of RAM and no swap: a viewer selecting 200
// image-heavy documents would otherwise build a multi-GB archive in one
// request and starve every other team. Documents that do not fit are
// skipped and listed in the manifest.
const maxBatchExportOutput = 200 << 20 // 200 MB

// maxBatchExportDocBytes caps one document's HTML inside a batch. It bounds
// the per-document working set: a single huge document is converted
// in memory, and the zip is written to a file.
const maxBatchExportDocBytes = 32 << 20 // 32 MB

// batchExportSlots limits how many batch exports run at the same time
// across the whole service. Each one is CPU- and memory-hungry (it
// converts documents), so a global gate keeps a burst of large batches
// from taking the box down. Requests that do not get a slot are told to
// retry instead of queueing without bound.
var batchExportSlots = make(chan struct{}, 2)

// acquireBatchExportSlot takes one of the global batch-export slots. It
// reports false when all slots are busy.
func acquireBatchExportSlot() bool {
	select {
	case batchExportSlots <- struct{}{}:
		return true
	default:
		return false
	}
}

func releaseBatchExportSlot() {
	select {
	case <-batchExportSlots:
	default:
	}
}

// batchExportItem is one document in a batch request.
//
// HTML is optional: the client sends the rendered HTML when it has one
// (Mermaid diagrams already drawn to PNG, #56). When it is empty the server
// falls back to the stored copy, which exports Mermaid as a code block —
// the v1 behaviour.
type batchExportItem struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	HTML  string `json:"html"`
}

// batchExportRequest is the POST body of the batch export endpoint.
type batchExportRequest struct {
	Format    string            `json:"format"`
	Documents []batchExportItem `json:"documents"`
}

// batchManifestEntry records what happened to one requested document.
type batchManifestEntry struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"` // ok | skipped | error
	Reason string `json:"reason,omitempty"`
	File   string `json:"file,omitempty"`
}

// TeamBatchExportDocuments POST /teams/:team_id/documents/batch-export
//
// Exports several documents as one zip. The rules mirror the single-document
// export:
//
//   - Every document is checked for read permission on its own. A document
//     the caller cannot read is skipped and recorded in the manifest — it
//     never lands in the zip.
//   - If not a single document can be read, the request fails with 403
//     instead of returning an empty zip.
//   - One failing conversion does not abort the batch; the manifest records
//     the error and the other documents still ship.
//   - No plan gate: batch export is free, like the single-document docx
//     export.
func TeamBatchExportDocuments(c *gin.Context) {
	teamID := getTeamID(c)
	userID := c.GetString("user_id")
	teamRole := getTeamRole(c)

	// Global gate: a batch export converts documents, so it is both CPU- and
	// memory-hungry. Only a couple may run at a time; the rest are told to
	// retry rather than queued without bound.
	if !acquireBatchExportSlot() {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "批量导出请求过多，请稍后重试"})
		return
	}
	defer releaseBatchExportSlot()

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBatchExportBody)
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "批量导出内容过大，请分批导出"})
		return
	}

	var req batchExportRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误，需要 JSON：{format, documents:[{id, title, html}]}"})
		return
	}

	format := strings.ToLower(strings.TrimSpace(req.Format))
	if format == "" {
		format = strings.ToLower(strings.TrimSpace(c.DefaultQuery("format", "docx")))
	}
	if format != "docx" && format != "markdown" && format != "md" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的格式，可选: docx, markdown"})
		return
	}
	if format == "md" {
		format = "markdown"
	}

	if len(req.Documents) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "没有要导出的文档"})
		return
	}
	if len(req.Documents) > maxBatchExportDocs {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("一次最多导出 %d 篇，请分批导出", maxBatchExportDocs)})
		return
	}

	// Per-document permission check first. Reading is checked before any
	// content is touched, so a document the caller cannot read never leaks
	// its title into the manifest either.
	type permitted struct {
		item batchExportItem
	}
	var allowed []permitted
	manifest := make([]batchManifestEntry, 0, len(req.Documents))
	readable := 0

	for _, item := range req.Documents {
		docID := strings.TrimSpace(item.ID)
		entry := batchManifestEntry{ID: docID, Title: item.Title}
		if docID == "" {
			entry.Status = "skipped"
			entry.Reason = "缺少文档 id"
			manifest = append(manifest, entry)
			continue
		}
		// The document must belong to this team and still be active.
		if !documentInTeam(teamID, docID, true) {
			entry.Status = "skipped"
			entry.Reason = "文档不存在或不属于当前团队"
			manifest = append(manifest, entry)
			continue
		}
		if !service.HasTeamPermission(c.Request.Context(), userID, teamID, teamRole, "document", docID, "read") {
			entry.Status = "skipped"
			entry.Reason = "没有读取权限"
			manifest = append(manifest, entry)
			continue
		}
		readable++
		allowed = append(allowed, permitted{item: item})
	}

	if readable == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "没有可导出的文档（全部缺少读取权限或不存在）"})
		return
	}

	// Convert each permitted document. A failure is recorded and the batch
	// continues — one broken document must not cost the caller the rest.
	//
	// The archive is written to a temp file, not to memory: the production
	// box has 7.5GB of RAM and no swap, and a 200-document batch of
	// image-heavy documents would otherwise build a multi-GB buffer.
	tmp, err := os.CreateTemp("", "mistdocs-batch-*.zip")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建临时文件失败"})
		return
	}
	tmpName := tmp.Name()
	defer func() {
		tmp.Close()
		os.Remove(tmpName)
	}()

	zw := zip.NewWriter(tmp)
	used := map[string]bool{}
	exported := 0
	written := int64(0)

	for _, p := range allowed {
		item := p.item
		docID := strings.TrimSpace(item.ID)
		entry := batchManifestEntry{ID: docID, Title: item.Title}

		title, docType, err := batchDocMeta(docID, teamID)
		if err != nil {
			entry.Status = "error"
			entry.Reason = "读取文档信息失败"
			manifest = append(manifest, entry)
			continue
		}
		if title != "" {
			entry.Title = title
		}
		if docType == "sheet" && format == "docx" {
			// Sheets have no HTML, and sheet→docx stays out of scope (#40).
			// Markdown is fine: the old per-document GET export produced it.
			entry.Status = "skipped"
			entry.Reason = "sheet 暂不支持批量导出 docx"
			manifest = append(manifest, entry)
			continue
		}

		html := strings.TrimSpace(item.HTML)
		if format != "docx" {
			// Markdown always uses the stored copy. The client-rendered HTML
			// exists for the docx image pipeline; accepting it for markdown
			// would widen the attack surface for no benefit (the markdown
			// output is the same either way, minus the images).
			html = ""
		}
		if html == "" {
			// No client-rendered HTML: use the stored copy. Mermaid stays a
			// code block here — the client-rendered path is optional.
			html = string(readDocContent(docID))
		}
		if strings.TrimSpace(html) == "" {
			entry.Status = "error"
			entry.Reason = "文档内容为空"
			manifest = append(manifest, entry)
			continue
		}
		if len(html) > maxBatchExportDocBytes {
			entry.Status = "skipped"
			entry.Reason = "文档过大，单篇超过 32MB，请单独导出"
			manifest = append(manifest, entry)
			continue
		}
		html = restrictMediaToTeam(teamID, signMediaURLs(teamID, html))

		var data []byte
		var ext string
		switch format {
		case "docx":
			data, err = htmlToDocx(html)
			ext = ".docx"
		default:
			data = []byte(htmlToMarkdown(html))
			ext = ".md"
		}
		if err != nil {
			entry.Status = "error"
			entry.Reason = "转换失败：" + err.Error()
			manifest = append(manifest, entry)
			continue
		}

		// Total-output guard: stop adding documents once the archive
		// exceeds the cap. The remaining documents are reported as skipped
		// so the caller knows to export them in a second batch.
		if written+int64(len(data)) > maxBatchExportOutput {
			entry.Status = "skipped"
			entry.Reason = "超出单次导出体积上限（200MB），请分批导出"
			manifest = append(manifest, entry)
			continue
		}

		name := uniqueBatchName(used, exportBaseName(entry.Title), ext)
		f, err := zw.Create(name)
		if err != nil {
			entry.Status = "error"
			entry.Reason = "写入压缩包失败"
			manifest = append(manifest, entry)
			continue
		}
		if _, err := f.Write(data); err != nil {
			entry.Status = "error"
			entry.Reason = "写入压缩包失败"
			manifest = append(manifest, entry)
			continue
		}
		written += int64(len(data))
		entry.Status = "ok"
		entry.File = name
		exported++
		manifest = append(manifest, entry)
	}

	// The manifest explains every requested document: which ones shipped,
	// which were skipped and why. It is plain text so it opens anywhere.
	manifestText := buildBatchManifest(manifest, exported, format)
	if f, err := zw.Create("_manifest.txt"); err == nil {
		_, _ = f.Write([]byte(manifestText))
	}

	if err := zw.Close(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成压缩包失败"})
		return
	}
	if exported == 0 {
		// Every permitted document failed to convert. Refuse instead of
		// shipping a zip that contains nothing but a manifest.
		c.JSON(http.StatusInternalServerError, gin.H{"error": "所有文档都导出失败，请查看单篇导出结果"})
		return
	}

	stamp := time.Now().Format("20060102-150405")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="mistdocs-export-%s.zip"`, stamp))
	// The number of documents that actually made it into the zip. The
	// frontend shows this instead of the requested count, which would
	// include skipped and failed documents.
	c.Header("X-Export-Count", strconv.Itoa(exported))
	c.Header("Content-Length", strconv.FormatInt(written, 10))

	// Serve the archive from the temp file instead of a memory buffer.
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取导出文件失败"})
		return
	}
	c.Status(http.StatusOK)
	c.Header("Content-Type", "application/zip")
	if _, err := io.Copy(c.Writer, tmp); err != nil {
		log.Printf("batch export: streaming zip failed: %v", err)
	}

	userName, _ := c.Get("username")
	audit(c, "export", "document", "", fmt.Sprintf("%d 篇", exported),
		fmt.Sprintf("%s 批量导出文档 (%s, %d/%d 篇成功)", userName, format, exported, readable))
}

// batchDocMeta reads a document's title and type, scoped to the team.
func batchDocMeta(docID, teamID string) (title, docType string, err error) {
	err = database.DB.QueryRow(
		`SELECT title, type FROM md_documents WHERE id=? AND team_id=? AND status=1`,
		docID, teamID).Scan(&title, &docType)
	return
}

// uniqueBatchName makes the archive entry name unique: two documents can
// share a title, and a zip with duplicate names is ambiguous to extract.
func uniqueBatchName(used map[string]bool, base, ext string) string {
	name := base + ext
	if !used[name] {
		used[name] = true
		return name
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d%s", base, i, ext)
		if !used[candidate] {
			used[candidate] = true
			return candidate
		}
	}
}

// buildBatchManifest renders the human-readable manifest that ships inside
// the zip. It lists every requested document and what happened to it.
func buildBatchManifest(entries []batchManifestEntry, exported int, format string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "MistDocs 批量导出清单\n")
	fmt.Fprintf(&b, "格式: %s\n", format)
	fmt.Fprintf(&b, "成功: %d 篇\n", exported)
	fmt.Fprintf(&b, "生成时间: %s\n\n", time.Now().Format("2006-01-02 15:04:05"))

	for _, e := range entries {
		title := e.Title
		if title == "" {
			title = "(无标题)"
		}
		switch e.Status {
		case "ok":
			fmt.Fprintf(&b, "[成功] %s -> %s\n", title, e.File)
		case "skipped":
			fmt.Fprintf(&b, "[跳过] %s：%s\n", title, e.Reason)
		default:
			fmt.Fprintf(&b, "[失败] %s：%s\n", title, e.Reason)
		}
	}
	return b.String()
}
