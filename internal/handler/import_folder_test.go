package handler

import (
	"archive/zip"
	"bytes"
	"testing"

	"golang.org/x/text/encoding/simplifiedchinese"
)

func TestCleanImportPath(t *testing.T) {
	cases := map[string]string{
		"手册/部署/a.md":        "手册/部署/a.md",
		`手册\部署\a.md`:        "手册/部署/a.md",
		"/手册/./a.md":        "手册/a.md",
		"../a.md":           "",
		"手册/../../a.md":     "",
		"手册/.git/config":    "",
		"__MACOSX/手册/a.md":  "",
		"手册/.DS_Store":      "",
		"手册/node_modules/x": "",
	}
	for in, want := range cases {
		if got := cleanImportPath(in); got != want {
			t.Errorf("cleanImportPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSplitImportRoot(t *testing.T) {
	root, out := splitImportRoot([]importEntry{{path: "手册/a.md"}, {path: "手册/b/c.md"}}, "zip名")
	if root != "手册" || out[0].path != "a.md" || out[1].path != "b/c.md" {
		t.Fatalf("got %q %v", root, out)
	}
	root, out = splitImportRoot([]importEntry{{path: "a.md"}, {path: "b/c.md"}}, "zip名")
	if root != "zip名" || out[1].path != "b/c.md" {
		t.Fatalf("got %q %v", root, out)
	}
}

func TestReadImportZipGBKNames(t *testing.T) {
	gbk, _ := simplifiedchinese.GBK.NewEncoder().String("运维手册/上线.md")
	buf := &bytes.Buffer{}
	zw := zip.NewWriter(buf)
	fw, _ := zw.CreateHeader(&zip.FileHeader{Name: gbk, Method: zip.Deflate, NonUTF8: true})
	fw.Write([]byte("# 上线"))
	zw.Close()
	entries, err := readImportZip(buf.Bytes())
	if err != nil || len(entries) != 1 || entries[0].path != "运维手册/上线.md" {
		t.Fatalf("entries = %v, err = %v", entries, err)
	}
}
