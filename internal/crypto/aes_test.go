package crypto

import (
	"bytes"
	"strings"
	"testing"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	for _, msg := range [][]byte{nil, []byte("x"), []byte("<p>中文内容</p>"), bytes.Repeat([]byte("ab"), 1<<16)} {
		ct, err := Encrypt(msg, key)
		if err != nil {
			t.Fatal(err)
		}
		if len(msg) > 0 && bytes.Contains(ct, msg) {
			t.Fatal("ciphertext contains the plaintext")
		}
		pt, err := Decrypt(ct, key)
		if err != nil || !bytes.Equal(pt, msg) {
			t.Fatalf("round trip failed: err=%v", err)
		}
	}
}

func TestEncryptUsesFreshNonce(t *testing.T) {
	key, _ := GenerateKey()
	a, _ := Encrypt([]byte("same"), key)
	b, _ := Encrypt([]byte("same"), key)
	if bytes.Equal(a, b) {
		t.Fatal("two encryptions of the same text must differ (random nonce)")
	}
}

func TestDecryptRejectsWrongKeyAndTampering(t *testing.T) {
	k1, _ := GenerateKey()
	k2, _ := GenerateKey()
	ct, _ := Encrypt([]byte("secret"), k1)
	if _, err := Decrypt(ct, k2); err == nil {
		t.Fatal("wrong key accepted")
	}
	tampered := append([]byte{}, ct...)
	tampered[len(tampered)-1] ^= 0x01
	if _, err := Decrypt(tampered, k1); err == nil {
		t.Fatal("tampered ciphertext accepted")
	}
	if _, err := Decrypt([]byte("short"), k1); err == nil {
		t.Fatal("short ciphertext accepted")
	}
}

func TestKeyLengthEnforced(t *testing.T) {
	if _, err := Encrypt([]byte("x"), make([]byte, 16)); err == nil {
		t.Fatal("16-byte key accepted for AES-256")
	}
	if _, err := Decrypt(make([]byte, 64), make([]byte, 31)); err == nil {
		t.Fatal("31-byte key accepted")
	}
}

func TestEncodeDecodeKey(t *testing.T) {
	key, _ := GenerateKey()
	got, err := DecodeKey(EncodeKey(key))
	if err != nil || !bytes.Equal(got, key) {
		t.Fatalf("key round trip: err=%v", err)
	}
	if _, err := DecodeKey("not base64!"); err == nil {
		t.Fatal("invalid base64 accepted")
	}
	if _, err := DecodeKey(EncodeKey([]byte("too short"))); err == nil || !strings.Contains(err.Error(), "32 bytes") {
		t.Fatalf("short key: err=%v", err)
	}
}

// Without a master key (dev mode) documents are stored as-is.
func TestDocumentPassthroughWithoutMasterKey(t *testing.T) {
	old := masterKey
	masterKey = nil
	t.Cleanup(func() { masterKey = old })
	in := []byte("<p>plain</p>")
	ct, err := EncryptDocument(in)
	if err != nil || !bytes.Equal(ct, in) {
		t.Fatalf("EncryptDocument without key: %q %v", ct, err)
	}
	pt, err := DecryptDocument(ct)
	if err != nil || !bytes.Equal(pt, in) {
		t.Fatalf("DecryptDocument without key: %q %v", pt, err)
	}
	if IsMasterKeyLoaded() {
		t.Fatal("IsMasterKeyLoaded should be false")
	}
}
