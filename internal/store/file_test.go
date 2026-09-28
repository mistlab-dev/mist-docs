package store

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/c-wind/mist-docs/internal/config"
)

func withRoot(t *testing.T) string {
	t.Helper()
	old := config.C.Storage
	root := t.TempDir()
	config.C.Storage.Root = root
	t.Cleanup(func() { config.C.Storage = old })
	return root
}

func TestPathsStayUnderRoot(t *testing.T) {
	root := withRoot(t)
	bad := []struct{ dept, doc string }{
		{"team", "../../etc"},
		{"..", "doc"},
		{"team", ".."},
		{"a/b", "doc"},
		{"team", `..\x`},
		{"team", "doc\x00"},
		{"team", ""},
		{"team", "."},
	}
	for _, b := range bad {
		for _, p := range []string{DocPath(b.dept, b.doc), VersionPath(b.dept, b.doc, 1), CurrentPath(b.dept, b.doc)} {
			rel, err := filepath.Rel(root, p)
			if err != nil || strings.HasPrefix(rel, "..") {
				t.Errorf("dept=%q doc=%q escaped the root: %s", b.dept, b.doc, p)
			}
		}
		if _, _, err := WriteVersion(b.dept, b.doc, 1, []byte("x")); !errors.Is(err, ErrUnsafePath) {
			t.Errorf("WriteVersion(%q,%q) err=%v, want ErrUnsafePath", b.dept, b.doc, err)
		}
		if _, err := ReadCurrent(b.dept, b.doc); !errors.Is(err, ErrUnsafePath) {
			t.Errorf("ReadCurrent(%q,%q) err=%v, want ErrUnsafePath", b.dept, b.doc, err)
		}
		if _, err := ReadVersion(b.dept, b.doc, 1); !errors.Is(err, ErrUnsafePath) {
			t.Errorf("ReadVersion(%q,%q) err=%v, want ErrUnsafePath", b.dept, b.doc, err)
		}
	}
	// Nothing may have been written outside the quarantine-free layout.
	if _, err := os.Stat(filepath.Join(root, "_unsafe")); err == nil {
		t.Error("unsafe ids must not create files")
	}
}

func TestSafeIDsAccepted(t *testing.T) {
	root := withRoot(t)
	if got, want := DocPath("team-mistlab", "0b7c5f2e-1d4a-4c1b-9f55-3a0e7f1a2b3c"),
		filepath.Join(root, "team-mistlab", "0b7c5f2e-1d4a-4c1b-9f55-3a0e7f1a2b3c"); got != want {
		t.Fatalf("DocPath = %s, want %s", got, want)
	}
	// Legacy rows may have an empty bucket; that still lands under the root.
	if got := DocPath("", "doc-1"); got != filepath.Join(root, "doc-1") {
		t.Fatalf("empty bucket: %s", got)
	}
}

// Without a master key the store writes plaintext; versions and current.dat
// must round-trip and a later version must not change an earlier one.
func TestWriteReadRoundTrip(t *testing.T) {
	withRoot(t)
	p1, n, err := WriteVersion("team-a", "doc-1", 1, []byte("<p>一</p>"))
	if err != nil || n != int64(len("<p>一</p>")) || !strings.HasSuffix(p1, "v1.dat") {
		t.Fatalf("write v1: path=%s n=%d err=%v", p1, n, err)
	}
	if _, _, err := WriteVersion("team-a", "doc-1", 2, []byte("<p>二</p>")); err != nil {
		t.Fatal(err)
	}
	cur, err := ReadCurrent("team-a", "doc-1")
	if err != nil || string(cur) != "<p>二</p>" {
		t.Fatalf("current = %q err=%v", cur, err)
	}
	v1, err := ReadVersion("team-a", "doc-1", 1)
	if err != nil || string(v1) != "<p>一</p>" {
		t.Fatalf("v1 = %q err=%v", v1, err)
	}
	if _, err := ReadVersion("team-a", "doc-1", 9); err == nil {
		t.Fatal("missing version should error")
	}
}

func TestStorageDefaults(t *testing.T) {
	old := config.C.Storage
	t.Cleanup(func() { config.C.Storage = old })

	config.C.Storage = config.StorageConfig{}
	if RootPath() != "/var/lib/mist-docs/files" {
		t.Errorf("default root = %s", RootPath())
	}
	if MaxFileSize() != 50*1024*1024 {
		t.Errorf("default max file size = %d", MaxFileSize())
	}
	if VersionKeep() != 20 {
		t.Errorf("default version keep = %d", VersionKeep())
	}

	config.C.Storage = config.StorageConfig{Root: "/data/x", MaxFileSize: 1024, VersionKeep: 5}
	if RootPath() != "/data/x" || MaxFileSize() != 1024 || VersionKeep() != 5 {
		t.Errorf("configured values ignored: %s %d %d", RootPath(), MaxFileSize(), VersionKeep())
	}
}
