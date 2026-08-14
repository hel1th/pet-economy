package pagination

func SealForTest(plaintext string) string {
	return sealCursor(plaintext)
}
