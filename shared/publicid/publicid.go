package publicid

import (
	"crypto/rand"
	"strings"
)

const (
	Length            = 12
	byteSize          = 8
	crockfordAlphabet = "0123456789abcdefghjkmnpqrstvwxyz"
	validAlphabet     = "0123456789abcdefghijklmnopqrstuvwxyz"
)

func New() string {
	buf := make([]byte, byteSize)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}

	return encodeCrockfordBase32(buf)
}

func IsValid(value string) bool {
	if len(value) != Length {
		return false
	}

	for _, r := range value {
		if !strings.ContainsRune(validAlphabet, r) {
			return false
		}
	}

	return true
}

func encodeCrockfordBase32(data []byte) string {
	var sb strings.Builder
	sb.Grow(Length)

	var bitBuffer uint64
	bitCount := 0

	for _, b := range data {
		bitBuffer = (bitBuffer << 8) | uint64(b)
		bitCount += 8

		for bitCount >= 5 {
			bitCount -= 5
			index := (bitBuffer >> uint(bitCount)) & 0x1F
			sb.WriteByte(crockfordAlphabet[index])
		}
	}

	if bitCount > 0 {
		index := (bitBuffer << uint(5-bitCount)) & 0x1F
		sb.WriteByte(crockfordAlphabet[index])
	}

	encoded := sb.String()
	if len(encoded) < Length {
		encoded += strings.Repeat("0", Length-len(encoded))
	}

	return encoded[:Length]
}
