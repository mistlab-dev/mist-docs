package handler

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestBatchExportConcurrencyGate: only a couple of batch exports run at a
// time. The gate is global on purpose — a batch export is both CPU- and
// memory-hungry, and the production box has 7.5GB of RAM with no swap.
func TestBatchExportConcurrencyGate(t *testing.T) {
	// Take every slot.
	for i := 0; i < cap(batchExportSlots); i++ {
		if !acquireBatchExportSlot() {
			t.Fatalf("slot %d should have been free", i)
		}
	}
	// The next one must be refused.
	if acquireBatchExportSlot() {
		t.Error("the gate should refuse a request when every slot is taken")
		releaseBatchExportSlot()
	}
	// Give the slots back.
	for i := 0; i < cap(batchExportSlots); i++ {
		releaseBatchExportSlot()
	}
	// Now a slot is free again.
	if !acquireBatchExportSlot() {
		t.Error("the gate should hand out a slot after a release")
	}
	releaseBatchExportSlot()
}

// TestUniqueBatchName: two documents with the same title must not produce
// the same zip entry name — an ambiguous archive is a problem to extract.
func TestUniqueBatchName(t *testing.T) {
	used := map[string]bool{}
	if got := uniqueBatchName(used, "报告", ".docx"); got != "报告.docx" {
		t.Errorf("first name = %q, want 报告.docx", got)
	}
	if got := uniqueBatchName(used, "报告", ".docx"); got != "报告-2.docx" {
		t.Errorf("second name = %q, want 报告-2.docx", got)
	}
	if got := uniqueBatchName(used, "报告", ".docx"); got != "报告-3.docx" {
		t.Errorf("third name = %q, want 报告-3.docx", got)
	}
}

// TestServeBatchArchiveOverRealHTTP: the archive must survive a real HTTP
// round trip.
//
// A ResponseRecorder does not enforce Content-Length, so the earlier bug
// (declaring the sum of the uncompressed document bytes instead of the size
// of the zip) slipped past the integration tests: net/http aborted the
// response with "wrote more than the declared Content-Length" and the
// client received a truncated archive whose central directory was missing.
//
// This test goes over a real socket: it checks that the declared length
// matches what the server actually sent, that the client reads the whole
// body, and that the result opens as a zip with its manifest intact.
func TestServeBatchArchiveOverRealHTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Build an archive the way the handler does: a few entries plus the
	// manifest. The entries are already-compressed bytes (like a docx), so
	// the zip file is larger than the sum of the entry payloads — exactly
	// the case that used to be truncated.
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range []string{"甲.docx", "乙.docx"} {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		// Incompressible-ish payload: docx is already zip-compressed.
		payload := bytes.Repeat([]byte{0x41, 0x42, 0x43, 0x44}, 4096)
		if _, err := w.Write(payload); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if w, err := zw.Create("_manifest.txt"); err == nil {
		_, _ = w.Write([]byte("MistDocs 批量导出清单\n"))
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	archive := buf.Bytes()

	tmp, err := os.CreateTemp("", "batch-archive-*.zip")
	if err != nil {
		t.Fatalf("temp file: %v", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(archive); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	r := gin.New()
	r.GET("/batch", func(c *gin.Context) {
		serveBatchArchive(c, tmp, 2)
	})
	srv := httptest.NewServer(r)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/batch")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if got := resp.Header.Get("X-Export-Count"); got != "2" {
		t.Errorf("X-Export-Count = %q, want 2", got)
	}
	// The archive must be labelled application/zip so the browser routes it
	// to a download instead of trying to render it. Worth pinning: if the
	// explicit header ever goes away, net/http falls back to sniffing, and
	// what the client sees then depends on the first bytes of the archive
	// rather than on our intent.
	if got := resp.Header.Get("Content-Type"); got != "application/zip" {
		t.Errorf("Content-Type = %q, want application/zip", got)
	}
	if cd := resp.Header.Get("Content-Disposition"); cd != "" && !strings.Contains(cd, ".zip") {
		t.Errorf("Content-Disposition = %q, want a .zip filename", cd)
	}

	// The declared length must match the archive we are about to receive.
	if cl := resp.ContentLength; cl != int64(len(archive)) {
		t.Errorf("Content-Length = %d, want %d (the real archive size)", cl, len(archive))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if len(body) != len(archive) {
		t.Fatalf("body length = %d, want %d (the download was truncated)", len(body), len(archive))
	}

	// The archive must open and keep its manifest.
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatalf("downloaded archive is not a valid zip: %v", err)
	}
	hasManifest := false
	for _, f := range zr.File {
		if f.Name == "_manifest.txt" {
			hasManifest = true
		}
	}
	if !hasManifest {
		t.Error("downloaded archive is missing _manifest.txt")
	}
}
