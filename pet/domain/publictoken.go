package domain

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"sync"

	"github.com/google/uuid"
)

const publicTokenHexLen = 16

var (
	publicTokenMu     sync.RWMutex
	publicTokenSecret []byte
)

func SetPublicTokenSecret(secret string) {
	publicTokenMu.Lock()
	defer publicTokenMu.Unlock()

	publicTokenSecret = []byte(secret)
}

func PublicToken(id uuid.UUID) string {
	if id == uuid.Nil {
		return ""
	}

	publicTokenMu.RLock()
	secret := publicTokenSecret
	publicTokenMu.RUnlock()

	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(id.String()))

	return hex.EncodeToString(mac.Sum(nil))[:publicTokenHexLen]
}
