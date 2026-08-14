package pagination

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"sync"
)

const cursorKeyInfo = "pagination-cursor-v1"

var (
	cursorKeyMu sync.RWMutex
	cursorKey   = deriveCursorKey("")
)

func SetCursorSecret(secret string) {
	cursorKeyMu.Lock()
	defer cursorKeyMu.Unlock()

	cursorKey = deriveCursorKey(secret)
}

func deriveCursorKey(secret string) [32]byte {
	return sha256.Sum256([]byte(cursorKeyInfo + "\x00" + secret))
}

func currentCursorKey() [32]byte {
	cursorKeyMu.RLock()
	defer cursorKeyMu.RUnlock()

	return cursorKey
}

func newCursorAEAD() (cipher.AEAD, error) {
	key := currentCursorKey()

	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}

	return cipher.NewGCM(block)
}

func sealCursor(plaintext string) string {
	aead, err := newCursorAEAD()
	if err != nil {
		return ""
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return ""
	}

	sealed := aead.Seal(nonce, nonce, []byte(plaintext), nil)

	return base64.RawURLEncoding.EncodeToString(sealed)
}

func openCursor(raw string) (string, error) {
	sealed, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return "", ErrInvalidCursor
	}

	aead, err := newCursorAEAD()
	if err != nil {
		return "", ErrInvalidCursor
	}

	if len(sealed) < aead.NonceSize() {
		return "", ErrInvalidCursor
	}

	nonce, ciphertext := sealed[:aead.NonceSize()], sealed[aead.NonceSize():]

	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", ErrInvalidCursor
	}

	return string(plaintext), nil
}
