package service

import (
	"context"
	"os"
	"path/filepath"

	"github.com/c-wind/mist-docs/internal/crypto"
	"github.com/c-wind/mist-docs/internal/store"
)

// ==================== Yjs State 持久化（加密） ====================

func GetDocumentYjsState(docID string) ([]byte, error) {
	doc, err := GetDocumentByID(context.Background(), docID)
	if err != nil || doc == nil {
		return nil, nil
	}

	path := filepath.Join(store.DocPath(doc.DepartmentID, doc.ID), "yjs.state.dat")
	encryptedData, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Decrypt
	if crypto.IsMasterKeyLoaded() {
		data, err := crypto.DecryptDocument(encryptedData)
		if err != nil {
			// Maybe file was written before encryption was enabled
			return encryptedData, nil
		}
		return data, nil
	}

	return encryptedData, nil
}

func SaveDocumentYjsState(docID string, state []byte) error {
	doc, err := GetDocumentByID(context.Background(), docID)
	if err != nil || doc == nil {
		return nil
	}

	dir := store.DocPath(doc.DepartmentID, doc.ID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Encrypt
	var dataToWrite []byte
	if crypto.IsMasterKeyLoaded() {
		encrypted, err := crypto.EncryptDocument(state)
		if err != nil {
			return err
		}
		dataToWrite = encrypted
	} else {
		dataToWrite = state
	}

	path := filepath.Join(dir, "yjs.state.dat")
	return os.WriteFile(path, dataToWrite, 0644)
}

// ==================== 简化权限检查（WS 用） ====================

func CheckPermissionSimple(ctx context.Context, userID, deptID, docID string) (string, error) {
	return CheckPermission(ctx, userID, deptID, "document", docID)
}
