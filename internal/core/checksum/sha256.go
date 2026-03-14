package checksum

import (
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"io"
	"os"
	"strings"
)

func FileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var h hash.Hash = sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func MatchesSHA256(path, expected string) (actual string, ok bool, err error) {
	actual, err = FileSHA256(path)
	if err != nil {
		return "", false, err
	}
	return actual, strings.EqualFold(actual, strings.TrimSpace(expected)), nil
}
