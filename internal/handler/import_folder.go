package handler

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/c-wind/mist-docs/internal/database"
	"github.com/c-wind/mist-docs/internal/service"
	"github.com/c-wind/mist-docs/internal/store"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/text/encoding/simplifiedchinese"
)

// Folder import: a whole Markdown folder (picked in the browser, or a .zip)
// becomes a folder tree with one document per file. Images referenced from
// the documents are stored in the team's media library, and links between
// imported documents point at the new documents.

const (
	maxFolderImportBytes   = 100 << 20 // whole request / unpacked zip
	maxFolderImportEntries = 3000      // files considered (after skipping hidden ones)
	maxFolderImportDocs    = 500
	maxFolderImportImage   = 20 << 20
)

var importImageExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".svg": true, ".bmp": true,
}

var importDocExts = map[string]bool{
	".md": true, ".markdown": true, ".txt": true, ".html": true, ".htm": true, ".docx": true, ".xlsx": true,
}

type importEntry struct {
	path string // cleaned, slash-separated, relative to the imported folder
	data []byte
}

type FolderImportWarning struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

// cleanImportPath normalises an uploaded relative path. It returns "" for
// anything outside the folder, and for hidden or system files.
func cleanImportPath(p string) string {
	p = strings.ReplaceAll(p, `\`, "/")
	p = strings.TrimLeft(p, "/")
	if p == "" {
		return ""
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == ".." {
			return ""
		}
	}
	p = path.Clean(p)
	if p == "." || strings.HasPrefix(p, "../") {
		return ""
	}
	for _, seg := range strings.Split(p, "/") {
		if strings.HasPrefix(seg, ".") || seg == "__MACOSX" || seg == "node_modules" ||
			strings.EqualFold(seg, "Thumbs.db") || strings.EqualFold(seg, "desktop.ini") {
			return ""
		}
	}
	return p
}

// zipEntryName decodes an entry name. Archives made by Windows' built-in
// "send to compressed folder" on Chinese systems store names in GBK without
// the UTF-8 flag.
func zipEntryName(f *zip.File) string {
	if !f.NonUTF8 || utf8.ValidString(f.Name) {
		return f.Name
	}
	if s, err := simplifiedchinese.GBK.NewDecoder().String(f.Name); err == nil {
		return s
	}
	return f.Name
}

func readImportZip(data []byte) ([]importEntry, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("压缩包打不开，请确认是 .zip 文件")
	}
	var out []importEntry
	var total int64
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		p := cleanImportPath(zipEntryName(f))
		if p == "" {
			continue
		}
		if len(out) >= maxFolderImportEntries {
			return nil, fmt.Errorf("文件太多了，一次最多 %d 个", maxFolderImportEntries)
		}
		total += int64(f.UncompressedSize64)
		if total > maxFolderImportBytes {
			return nil, fmt.Errorf("解压后超过 %dMB，请分几次导入", maxFolderImportBytes>>20)
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("读取 %s 失败", p)
		}
		b, err := io.ReadAll(io.LimitReader(rc, maxFolderImportBytes+1))
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("读取 %s 失败", p)
		}
		if int64(len(b)) > maxFolderImportBytes {
			return nil, fmt.Errorf("解压后超过 %dMB，请分几次导入", maxFolderImportBytes>>20)
		}
		out = append(out, importEntry{path: p, data: b})
	}
	return out, nil
}

// splitImportRoot finds the folder name to create. A browser folder pick
// sends "Folder/a.md"; a zip usually has one top folder too. Otherwise the
// fallback name (the zip's name) is used and paths stay as they are.
func splitImportRoot(entries []importEntry, fallback string) (string, []importEntry) {
	root := ""
	for i, e := range entries {
		slash := strings.IndexByte(e.path, '/')
		if slash < 0 {
			return fallback, entries
		}
		if i == 0 {
			root = e.path[:slash]
		} else if e.path[:slash] != root {
			return fallback, entries
		}
	}
	if root == "" {
		return fallback, entries
	}
	out := make([]importEntry, len(entries))
	for i, e := range entries {
		out[i] = importEntry{path: e.path[len(root)+1:], data: e.data}
	}
	return root, out
}

// resolveImportRef turns a reference written in the file at fromPath into a
// path inside the import, trying it as written, then URL-decoded.
func resolveImportRef(fromPath, ref string) []string {
	ref = strings.TrimSpace(ref)
	if i := strings.IndexAny(ref, "?#"); i >= 0 {
		ref = ref[:i]
	}
	ref = strings.Trim(ref, "<>")
	if ref == "" {
		return nil
	}
	cands := []string{ref}
	if dec, err := url.PathUnescape(ref); err == nil && dec != ref {
		cands = append(cands, dec)
	}
	var out []string
	for _, c := range cands {
		c = strings.ReplaceAll(c, `\`, "/")
		var p string
		if strings.HasPrefix(c, "/") {
			p = path.Clean(strings.TrimLeft(c, "/"))
		} else {
			p = path.Clean(path.Join(path.Dir(fromPath), c))
		}
		if p != "." && !strings.HasPrefix(p, "../") && p != ".." {
			out = append(out, p)
		}
	}
	return out
}

// TeamImportFolder POST /teams/:team_id/import/folder
//
// multipart: either files (repeated) with matching paths (relative path of
// each file, e.g. "手册/部署/上线.md"), or archive (one .zip). folder_id puts
// the new folder inside an existing one. Creating folders is an admin action,
// so this is admin-only like POST /folders.
func TeamImportFolder(c *gin.Context) {
	if !requireTeamAdmin(c) {
		return
	}
	teamID := getTeamID(c)
	userID := c.GetString("user_id")

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxFolderImportBytes+(1<<20))
	form, err := c.MultipartForm()
	if err != nil {
		if strings.Contains(err.Error(), "too large") {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": fmt.Sprintf("一次最多导入 %dMB，请分几次导入", maxFolderImportBytes>>20)})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择文件夹或 .zip 压缩包"})
		return
	}
	parentID := c.PostForm("folder_id")
	if parentID != "" && !folderInTeam(teamID, parentID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文件夹不存在"})
		return
	}

	var entries []importEntry
	fallbackName := strings.TrimSpace(c.PostForm("name"))
	if archives := form.File["archive"]; len(archives) > 0 {
		fh := archives[0]
		if fallbackName == "" {
			fallbackName = strings.TrimSuffix(filepath.Base(fh.Filename), filepath.Ext(fh.Filename))
		}
		src, err := fh.Open()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "读取压缩包失败"})
			return
		}
		data, _ := io.ReadAll(io.LimitReader(src, maxFolderImportBytes+1))
		src.Close()
		if entries, err = readImportZip(data); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	} else {
		files := form.File["files"]
		paths := form.Value["paths"]
		if len(files) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "请选择文件夹或 .zip 压缩包"})
			return
		}
		if len(paths) != len(files) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "文件和路径数量对不上"})
			return
		}
		if len(files) > maxFolderImportEntries {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("文件太多了，一次最多 %d 个", maxFolderImportEntries)})
			return
		}
		for i, fh := range files {
			p := cleanImportPath(paths[i])
			if p == "" {
				continue
			}
			src, err := fh.Open()
			if err != nil {
				continue
			}
			data, _ := io.ReadAll(io.LimitReader(src, maxFolderImportBytes+1))
			src.Close()
			entries = append(entries, importEntry{path: p, data: data})
		}
	}
	if fallbackName == "" {
		fallbackName = "导入的文件夹"
	}
	rootName, entries := splitImportRoot(entries, fallbackName)

	var docs []importEntry
	images := map[string]importEntry{}
	imagesByBase := map[string][]string{}
	var warnings []FolderImportWarning
	for _, e := range entries {
		ext := strings.ToLower(path.Ext(e.path))
		switch {
		case importDocExts[ext]:
			docs = append(docs, e)
		case importImageExts[ext]:
			images[e.path] = e
			b := strings.ToLower(path.Base(e.path))
			imagesByBase[b] = append(imagesByBase[b], e.path)
		default:
			warnings = append(warnings, FolderImportWarning{Path: e.path, Message: "不支持的格式，已跳过"})
		}
	}
	if len(docs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "没有找到能导入的文档（支持 .md、.txt、.html、.docx、.xlsx）"})
		return
	}
	if len(docs) > maxFolderImportDocs {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("文档太多了，一次最多 %d 篇，请分几次导入", maxFolderImportDocs)})
		return
	}
	sort.Slice(docs, func(i, j int) bool { return docs[i].path < docs[j].path })

	var docCount int
	database.DB.QueryRow("SELECT COUNT(*) FROM md_documents WHERE status = 1 AND team_id = ?", teamID).Scan(&docCount)
	if !service.CheckDocumentLimit(c, docCount+len(docs)-1) {
		return
	}
	var incoming int64
	for _, d := range docs {
		incoming += int64(len(d.data))
	}
	for _, im := range images {
		incoming += int64(len(im.data))
	}
	if !service.CheckStorageLimit(c, teamStorageBytes(teamID), incoming) {
		return
	}

	// Folders: the root plus every directory that holds a document
	// (image-only directories such as images/ or assets/ are not created).
	rootName = uniqueFolderName(teamID, parentID, rootName)
	rootID, err := createImportFolder(c, teamID, parentID, rootName, userID, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建文件夹失败"})
		return
	}
	folderIDs := map[string]string{"": rootID}
	var dirs []string
	seen := map[string]bool{}
	for _, d := range docs {
		for dir := path.Dir(d.path); dir != "."; dir = path.Dir(dir) {
			if !seen[dir] {
				seen[dir] = true
				dirs = append(dirs, dir)
			}
		}
	}
	sort.Slice(dirs, func(i, j int) bool {
		di, dj := strings.Count(dirs[i], "/"), strings.Count(dirs[j], "/")
		if di != dj {
			return di < dj
		}
		return dirs[i] < dirs[j]
	})
	order := map[string]int{}
	for _, dir := range dirs {
		parent := path.Dir(dir)
		if parent == "." {
			parent = ""
		}
		order[parent]++
		id, err := createImportFolder(c, teamID, folderIDs[parent], path.Base(dir), userID, order[parent])
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "创建文件夹失败：" + dir})
			return
		}
		folderIDs[dir] = id
	}

	// Document ids up front so documents can link to each other.
	docIDs := map[string]string{}
	for _, d := range docs {
		docIDs[d.path] = uuid.New().String()
	}

	stored := map[string]string{} // image path -> signed URL
	imageCount := 0
	storeImage := func(p string) (string, bool) {
		if u, ok := stored[p]; ok {
			return u, u != ""
		}
		im := images[p]
		if len(im.data) > maxFolderImportImage {
			stored[p] = ""
			warnings = append(warnings, FolderImportWarning{Path: p, Message: fmt.Sprintf("图片超过 %dMB，没有导入", maxFolderImportImage>>20)})
			return "", false
		}
		u, err := saveImportImage(teamID, userID, p, im.data)
		if err != nil {
			stored[p] = ""
			log.Printf("import folder image %s: %v", p, err)
			warnings = append(warnings, FolderImportWarning{Path: p, Message: "图片保存失败"})
			return "", false
		}
		stored[p] = u
		imageCount++
		return u, true
	}

	results := make([]BatchImportResult, 0, len(docs))
	for _, d := range docs {
		ext := strings.ToLower(path.Ext(d.path))
		title := strings.TrimSpace(strings.TrimSuffix(path.Base(d.path), path.Ext(d.path)))
		if title == "" {
			title = path.Base(d.path)
		}
		if len(d.data) > maxImportFileSize {
			results = append(results, BatchImportResult{Title: d.path, Status: "skipped", Error: "文件超过10MB"})
			continue
		}
		var docType string
		var content []byte
		if ext == ".md" || ext == ".markdown" {
			from := d.path
			refs := &mdRefs{
				image: func(ref string) (string, bool) {
					cands := resolveImportRef(from, ref)
					for _, p := range cands {
						if _, ok := images[p]; ok {
							return storeImage(p)
						}
					}
					// Note apps often write just the file name and keep
					// pictures in a shared attachments folder.
					for _, p := range cands {
						if hits := imagesByBase[strings.ToLower(path.Base(p))]; len(hits) == 1 {
							return storeImage(hits[0])
						}
					}
					return "", false
				},
				link: func(ref string) (string, bool) {
					for _, p := range resolveImportRef(from, ref) {
						if id, ok := docIDs[p]; ok {
							return "/docs/" + id, true
						}
					}
					return "", false
				},
			}
			html, fmTitle := renderMarkdown(string(d.data), refs)
			if fmTitle != "" {
				title = fmTitle
			}
			for _, m := range refs.missing {
				warnings = append(warnings, FolderImportWarning{Path: d.path, Message: "找不到图片：" + m})
			}
			docType, content = "doc", []byte(html)
		} else {
			var convErr error
			docType, content, convErr = convertImport(ext, d.data)
			if convErr != nil {
				results = append(results, BatchImportResult{Title: d.path, Status: "error", Error: convErr.Error()})
				continue
			}
		}
		if utf8.RuneCountInString(title) > 200 {
			title = string([]rune(title)[:200])
		}

		dir := path.Dir(d.path)
		if dir == "." {
			dir = ""
		}
		docID := docIDs[d.path]
		if _, err := database.DB.Exec(
			`INSERT INTO md_documents (id, team_id, folder_id, department_id, title, type, status, created_by, updated_by)
			 VALUES (?, ?, ?, '', ?, ?, 1, ?, ?)`,
			docID, teamID, folderIDs[dir], title, docType, userID, userID); err != nil {
			results = append(results, BatchImportResult{Title: d.path, Status: "error", Error: "保存失败"})
			log.Printf("import folder %s: %v", d.path, err)
			continue
		}
		if err := writeInitialVersion(docID, userID, content); err != nil {
			log.Printf("import %s: %v", docID, err)
		}
		audit(c, "import_doc", "document", docID, title, fmt.Sprintf(`{"file":%q,"folder":%q}`, d.path, rootName))
		results = append(results, BatchImportResult{Title: d.path, ID: docID, Type: docType, Status: "created"})
	}

	created := 0
	for _, r := range results {
		if r.Status == "created" {
			created++
		}
	}
	if warnings == nil {
		warnings = []FolderImportWarning{}
	}
	msg := fmt.Sprintf("已导入「%s」：%d 篇文档、%d 张图片", rootName, created, imageCount)
	resp := gin.H{
		"data": gin.H{
			"folder_id": rootID, "folder_name": rootName,
			"documents": created, "images": imageCount, "folders": len(folderIDs),
		},
		"results": results, "warnings": warnings, "message": msg,
	}
	if created == 0 {
		for _, id := range folderIDs {
			database.DB.Exec(`DELETE FROM md_team_folders WHERE id=? AND team_id=?`, id, teamID)
		}
		resp["data"] = nil
		resp["error"] = "一篇文档都没有导入成功"
		c.JSON(http.StatusBadRequest, resp)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// uniqueFolderName avoids two sibling folders with the same name when the
// same folder is imported twice: 手册, 手册 (2), 手册 (3)…
func uniqueFolderName(teamID, parentID, name string) string {
	name = strings.TrimSpace(name)
	if utf8.RuneCountInString(name) > 100 {
		name = string([]rune(name)[:100])
	}
	exists := func(n string) bool {
		var cnt int
		database.DB.QueryRow(`SELECT COUNT(*) FROM md_team_folders WHERE team_id=? AND parent_id=? AND name=?`, teamID, parentID, n).Scan(&cnt)
		return cnt > 0
	}
	if !exists(name) {
		return name
	}
	for i := 2; i < 1000; i++ {
		n := fmt.Sprintf("%s (%d)", name, i)
		if !exists(n) {
			return n
		}
	}
	return name + " " + uuid.New().String()[:8]
}

func createImportFolder(c *gin.Context, teamID, parentID, name, userID string, order int) (string, error) {
	if utf8.RuneCountInString(name) > 100 {
		name = string([]rune(name)[:100])
	}
	id := uuid.New().String()
	if _, err := database.DB.Exec(
		`INSERT INTO md_team_folders (id, team_id, parent_id, name, sort_order, created_by) VALUES (?, ?, ?, ?, ?, ?)`,
		id, teamID, parentID, name, order, userID); err != nil {
		log.Printf("import folder mkdir %s: %v", name, err)
		return "", err
	}
	audit(c, "create_folder", "folder", id, name, `{"via":"import"}`)
	return id, nil
}

// saveImportImage stores one image like /upload does and returns its signed URL.
func saveImportImage(teamID, userID, p string, data []byte) (string, error) {
	ext := strings.ToLower(path.Ext(p))
	filename := uuid.New().String() + ext
	dir := fmt.Sprintf("%s/%s/media", store.RootPath(), teamID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, filename), data, 0644); err != nil {
		return "", err
	}
	database.DB.Exec(
		`INSERT INTO md_media (filename, team_id, original_name, uploaded_by, size) VALUES (?, ?, ?, ?, ?)`,
		filename, teamID, path.Base(p), userID, len(data))
	return signedMediaURL(teamID, filename), nil
}
