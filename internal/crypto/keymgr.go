package crypto

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/c-wind/mist-docs/internal/database"
	"github.com/google/uuid"
)

const (
	// Key types
	KeyTypeMaster = "master"
	KeyTypeDEK    = "dek"    // Data Encryption Key
	KeyTypeKEK    = "kek"    // Key Encryption Key (for rotation)

	// Key status
	KeyStatusActive   = "active"
	KeyStatusInactive = "inactive"
	KeyStatusRotated  = "rotated"
)

// EncryptedKey is stored in the database
type EncryptedKey struct {
	ID          string `json:"id"`
	Type        string `json:"type"`        // master / dek / kek
	Status      string `json:"status"`      // active / inactive / rotated
	Encrypted   string `json:"encrypted"`   // base64 encoded encrypted key
	Algorithm   string `json:"algorithm"`   // AES-256-GCM
	CreatedAt   string `json:"created_at"`
	RotatedFrom string `json:"rotated_from,omitempty"` // previous key ID
	CreatedBy   string `json:"created_by"`
}

// MasterKey holds the decrypted master key in memory
var masterKey []byte

// InitMasterKey loads or initializes the master key
func InitMasterKey() error {
	// Priority 1: environment variable
	if mk := os.Getenv("MISTDOCS_MASTER_KEY"); mk != "" {
		key, err := DecodeKey(mk)
		if err != nil {
			return fmt.Errorf("invalid MISTDOCS_MASTER_KEY: %w", err)
		}
		masterKey = key
		log.Println("[Crypto] Master key loaded from environment")
		return nil
	}

	// Priority 2: secrets file
	secretPaths := []string{
		"secrets/master.key",
		"/etc/mist-docs/secrets/master.key",
	}
	for _, p := range secretPaths {
		data, err := os.ReadFile(p)
		if err == nil {
			key, err := DecodeKey(string(data))
			if err != nil {
				return fmt.Errorf("invalid key file %s: %w", p, err)
			}
			masterKey = key
			log.Printf("[Crypto] Master key loaded from %s", p)
			return nil
		}
	}

	// Priority 3: first run, need to initialize
	return fmt.Errorf("master key not found. Run: mist-docs keygen")
}

// IsMasterKeyLoaded checks if master key is available
func IsMasterKeyLoaded() bool {
	return masterKey != nil
}

// Keygen generates and saves a new master key
func Keygen() error {
	// Check if already initialized
	for _, p := range []string{"secrets/master.key", "/etc/mist-docs/secrets/master.key"} {
		if _, err := os.Stat(p); err == nil {
			return fmt.Errorf("master key already exists at %s (backup first, then delete to regenerate)", p)
		}
	}

	key, err := GenerateKey()
	if err != nil {
		return err
	}

	encoded := EncodeKey(key)

	// Try to save to secrets file
	dir := "secrets"
	if err := os.MkdirAll(dir, 0700); err != nil {
		dir = "/etc/mist-docs/secrets"
		os.MkdirAll(dir, 0700)
	}

	path := filepath.Join(dir, "master.key")
	if err := os.WriteFile(path, []byte(encoded), 0600); err != nil {
		// Fallback: print to stdout
		fmt.Println("=== Master Key (save this securely!) ===")
		fmt.Println(encoded)
		fmt.Println("=== Store this in one of: ===")
		fmt.Println("  - Environment: MISTDOCS_MASTER_KEY=" + encoded)
		fmt.Println("  - File: secrets/master.key")
		return nil
	}

	fmt.Printf("Master key generated: %s\n", path)
	fmt.Printf("Key: %s\n", encoded)
	fmt.Println("⚠️  Back up this key! Loss = permanent data loss.")

	return nil
}

// EnsureDEK ensures there's an active DEK in the database
func EnsureDEK(ctx context.Context) error {
	// Check if active DEK exists
	var count int
	err := database.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM md_keys WHERE type='dek' AND status='active'`).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	// Generate new DEK
	return RotateDEK(ctx, "system")
}

// GetActiveDEK returns the current active DEK (decrypted)
func GetActiveDEK(ctx context.Context) ([]byte, error) {
	var encrypted string
	err := database.DB.QueryRowContext(ctx,
		`SELECT encrypted FROM md_keys WHERE type='dek' AND status='active' ORDER BY created_at DESC LIMIT 1`).Scan(&encrypted)
	if err != nil {
		return nil, fmt.Errorf("no active DEK: %w", err)
	}

	// encrypted is base64(nonce+ciphertext), decode it first
	cipherData, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return nil, fmt.Errorf("decode encrypted DEK: %w", err)
	}

	// Decrypt with master key
	dek, err := Decrypt(cipherData, masterKey)
	if err != nil {
		return nil, fmt.Errorf("decrypt DEK: %w", err)
	}

	return dek, nil
}

// RotateDEK generates a new DEK, encrypts it with the master key
func RotateDEK(ctx context.Context, operator string) error {
	newDEK, err := GenerateKey()
	if err != nil {
		return err
	}

	// Encrypt DEK with master key
	encryptedDEK, err := Encrypt(newDEK, masterKey)
	if err != nil {
		return err
	}

	// Mark old DEKs as rotated
	database.DB.ExecContext(ctx, `UPDATE md_keys SET status='rotated' WHERE type='dek' AND status='active'`)

	// Store new DEK
	keyRecord := EncryptedKey{
		ID:        uuid.New().String(),
		Type:      KeyTypeDEK,
		Status:    KeyStatusActive,
		Encrypted: EncodeKey(encryptedDEK),
		Algorithm: "AES-256-GCM",
		CreatedAt: time.Now().Format("2006-01-02 15:04:05"),
		CreatedBy: operator,
	}

	_, err = database.DB.ExecContext(ctx,
		`INSERT INTO md_keys (id, type, status, encrypted, algorithm, created_at, created_by) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		keyRecord.ID, keyRecord.Type, keyRecord.Status, keyRecord.Encrypted,
		keyRecord.Algorithm, keyRecord.CreatedAt, keyRecord.CreatedBy,
	)
	if err != nil {
		return err
	}

	log.Printf("[Crypto] New DEK generated: %s (by %s)", keyRecord.ID, operator)
	return nil
}

// EncryptDocument encrypts document data with the current DEK
func EncryptDocument(plaintext []byte) ([]byte, error) {
	if masterKey == nil {
		return plaintext, nil // no encryption if no master key
	}

	dek, err := GetActiveDEK(context.Background())
	if err != nil {
		return nil, fmt.Errorf("get DEK: %w", err)
	}

	return Encrypt(plaintext, dek)
}

// DecryptDocument decrypts document data
func DecryptDocument(ciphertext []byte) ([]byte, error) {
	if masterKey == nil {
		return ciphertext, nil // no encryption
	}

	dek, err := GetActiveDEK(context.Background())
	if err != nil {
		return nil, fmt.Errorf("get DEK: %w", err)
	}

	return Decrypt(ciphertext, dek)
}

// KeyInfo returns info about current keys
func KeyInfo(ctx context.Context) (map[string]interface{}, error) {
	info := map[string]interface{}{
		"master_key_loaded": masterKey != nil,
	}

	rows, err := database.DB.QueryContext(ctx,
		`SELECT id, type, status, algorithm, created_at, created_by FROM md_keys ORDER BY created_at DESC`)
	if err != nil {
		return info, err
	}
	defer rows.Close()

	keys := []map[string]interface{}{}
	for rows.Next() {
		var id, ktype, status, algorithm, createdAt, createdBy string
		rows.Scan(&id, &ktype, &status, &algorithm, &createdAt, &createdBy)
		keys = append(keys, map[string]interface{}{
			"id": id, "type": ktype, "status": status,
			"algorithm": algorithm, "created_at": createdAt, "created_by": createdBy,
		})
	}
	info["keys"] = keys

	return info, nil
}

// ShowKey prints master key info
func ShowKey() {
	if masterKey == nil {
		fmt.Println("Master key: NOT LOADED")
		return
	}
	fmt.Printf("Master key: loaded (%d bytes)\n", len(masterKey))
	fmt.Printf("Key (base64): %s\n", EncodeKey(masterKey))
}

// InitKeyTables ensures the key management table exists
func InitKeyTables(ctx context.Context) error {
	_, err := database.DB.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS md_keys (
			id VARCHAR(36) PRIMARY KEY,
			type VARCHAR(10) NOT NULL,
			status VARCHAR(10) NOT NULL DEFAULT 'active',
			encrypted TEXT NOT NULL,
			algorithm VARCHAR(20) NOT NULL DEFAULT 'AES-256-GCM',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			rotated_from VARCHAR(36) DEFAULT NULL,
			created_by VARCHAR(100) DEFAULT 'system',
			INDEX idx_type_status (type, status)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
	`)
	return err
}
