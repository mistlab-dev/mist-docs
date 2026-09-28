package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/c-wind/mist-docs/internal/config"
	"github.com/c-wind/mist-docs/internal/crypto"
)

// Init ensures storage root exists
func Init() error {
	root := config.C.Storage.Root
	if root == "" {
		root = "/var/lib/mist-docs/files"
	}
	return os.MkdirAll(root, 0755)
}

// InitCrypto initializes encryption (must be called before any file operations)
func InitCrypto() error {
	if err := crypto.InitKeyTables(context.Background()); err != nil {
		return fmt.Errorf("init key tables: %w", err)
	}

	if err := crypto.InitMasterKey(); err != nil {
		// Allow running without encryption (dev mode)
		fmt.Println("⚠️  Running without encryption (master key not configured)")
		return nil
	}

	if err := crypto.EnsureDEK(context.Background()); err != nil {
		return fmt.Errorf("ensure DEK: %w", err)
	}

	return nil
}

// RootPath returns the storage root
func RootPath() string {
	r := config.C.Storage.Root
	if r == "" {
		r = "/var/lib/mist-docs/files"
	}
	return r
}

// ErrUnsafePath is returned when a bucket or document id could leave the
// storage root (path separators, "..", NUL) or the document id is empty.
var ErrUnsafePath = errors.New("store: unsafe path component")

// safeSegment reports whether s can be used as one path element under the
// root. Ids are UUIDs or team ids today; this is defence in depth in case a
// caller ever passes a request value straight through.
func safeSegment(s string) bool {
	return s != "." && s != ".." && !strings.ContainsAny(s, "/\\\x00") && !strings.Contains(s, "..")
}

func checkIDs(deptID, docID string) error {
	if docID == "" || !safeSegment(docID) || !safeSegment(deptID) {
		return ErrUnsafePath
	}
	return nil
}

// DocPath returns the directory for a document's files. Unsafe ids are
// mapped to a quarantine directory inside the root instead of escaping it;
// the read/write helpers below refuse them outright.
func DocPath(deptID, docID string) string {
	if checkIDs(deptID, docID) != nil {
		return filepath.Join(RootPath(), "_unsafe")
	}
	return filepath.Join(RootPath(), deptID, docID)
}

// VersionPath returns the file path for a specific version
func VersionPath(deptID, docID string, version int) string {
	return filepath.Join(DocPath(deptID, docID), fmt.Sprintf("v%d.dat", version))
}

// CurrentPath returns the path for the current version
func CurrentPath(deptID, docID string) string {
	return filepath.Join(DocPath(deptID, docID), "current.dat")
}

// WriteVersion writes encrypted data as a new version file
func WriteVersion(deptID, docID string, version int, data []byte) (string, int64, error) {
	if err := checkIDs(deptID, docID); err != nil {
		return "", 0, err
	}
	dir := DocPath(deptID, docID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", 0, fmt.Errorf("create doc dir: %w", err)
	}

	// Encrypt if key is loaded
	encryptedData, err := crypto.EncryptDocument(data)
	if err != nil {
		return "", 0, fmt.Errorf("encrypt: %w", err)
	}

	path := VersionPath(deptID, docID, version)
	if err := os.WriteFile(path, encryptedData, 0644); err != nil {
		return "", 0, fmt.Errorf("write version file: %w", err)
	}

	// Also update current
	current := CurrentPath(deptID, docID)
	os.WriteFile(current, encryptedData, 0644)

	return path, int64(len(data)), nil // return original size, not encrypted size
}

// ReadCurrent reads and decrypts the current version data
func ReadCurrent(deptID, docID string) ([]byte, error) {
	if err := checkIDs(deptID, docID); err != nil {
		return nil, err
	}
	path := CurrentPath(deptID, docID)
	encryptedData, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read current: %w", err)
	}

	// Decrypt if key is loaded
	data, err := crypto.DecryptDocument(encryptedData)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}

	return data, nil
}

// ReadVersion reads and decrypts a specific version
func ReadVersion(deptID, docID string, version int) ([]byte, error) {
	if err := checkIDs(deptID, docID); err != nil {
		return nil, err
	}
	path := VersionPath(deptID, docID, version)
	encryptedData, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read version %d: %w", version, err)
	}

	// Decrypt
	data, err := crypto.DecryptDocument(encryptedData)
	if err != nil {
		return nil, fmt.Errorf("decrypt version %d: %w", version, err)
	}

	return data, nil
}

// MaxFileSize returns configured max file size
func MaxFileSize() int64 {
	m := config.C.Storage.MaxFileSize
	if m == 0 {
		return 50 * 1024 * 1024 // default 50MB
	}
	return m
}

// VersionKeep returns how many versions to keep
func VersionKeep() int {
	v := config.C.Storage.VersionKeep
	if v == 0 {
		return 20
	}
	return v
}

// IsEncryptionEnabled checks if encryption is active
func IsEncryptionEnabled() bool {
	return crypto.IsMasterKeyLoaded()
}
