package domain

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

const (
	nonceBytes        = 10
	signaturePrefixed = 16
	minSecretLen      = 16
	codeSeparator     = "-"
)

var (
	ErrWeakSecret    = errors.New("reward secret is too short")
	ErrInvalidCode   = errors.New("reward code is malformed")
	ErrForgedCode    = errors.New("reward code signature is invalid")
	codeEncoding     = base32.StdEncoding.WithPadding(base32.NoPadding)
	errNonceGenerate = errors.New("cannot generate nonce")
)

type RewardSigner struct {
	secret []byte
}

func NewRewardSigner(secret string) (*RewardSigner, error) {
	if len(secret) < minSecretLen {
		return nil, ErrWeakSecret
	}

	return &RewardSigner{secret: []byte(secret)}, nil
}

func (s *RewardSigner) Issue(userID uuid.UUID, rewardID string) (string, error) {
	nonce := make([]byte, nonceBytes)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("%w: %w", errNonceGenerate, err)
	}
	encoded := codeEncoding.EncodeToString(nonce)

	return encoded + codeSeparator + s.sign(userID, rewardID, encoded), nil
}

func (s *RewardSigner) Verify(userID uuid.UUID, rewardID, code string) error {
	nonce, signature, ok := strings.Cut(code, codeSeparator)
	if !ok || nonce == "" || signature == "" {
		return ErrInvalidCode
	}
	if _, err := codeEncoding.DecodeString(nonce); err != nil {
		return ErrInvalidCode
	}
	if !hmac.Equal([]byte(signature), []byte(s.sign(userID, rewardID, nonce))) {
		return ErrForgedCode
	}

	return nil
}

func (s *RewardSigner) sign(userID uuid.UUID, rewardID, nonce string) string {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(userID.String()))
	mac.Write([]byte{0})
	mac.Write([]byte(rewardID))
	mac.Write([]byte{0})
	mac.Write([]byte(nonce))

	return codeEncoding.EncodeToString(mac.Sum(nil))[:signaturePrefixed]
}
