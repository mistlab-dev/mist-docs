package store

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

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

// DocPath returns the directory for a document's files
func DocPath(deptID, docID string) string {
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

// DeleteDoc removes all files for a document
func DeleteDoc(deptID, docID string) error {
	dir := DocPath(deptID, docID)
	return os.RemoveAll(dir)
}

// MoveToTrash moves document files to trash
func MoveToTrash(deptID, docID string) error {
	src := DocPath(deptID, docID)
	trashDir := filepath.Join(RootPath(), "_trash", docID)
	if err := os.MkdirAll(filepath.Dir(trashDir), 0755); err != nil {
		return err
	}
	return os.Rename(src, trashDir)
}

// RestoreFromTrash restores from trash back to original location
func RestoreFromTrash(deptID, docID string) error {
	trashPath := filepath.Join(RootPath(), "_trash", docID)
	dst := DocPath(deptID, docID)
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	return os.Rename(trashPath, dst)
}

// PurgeFromTrash permanently deletes from trash
func PurgeFromTrash(docID string) error {
	trashPath := filepath.Join(RootPath(), "_trash", docID)
	return os.RemoveAll(trashPath)
}

// CopyVersion makes a copy of a version file (for snapshot/restore)
func CopyVersion(deptID, docID string, fromVersion, toVersion int) error {
	src := VersionPath(deptID, docID, fromVersion)
	dst := VersionPath(deptID, docID, toVersion)

	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
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