package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
)

// GenerateContentHash reads the content from the provided reader and generates a SHA256 hash
// The reader is consumed in the process
func GenerateContentHash(reader io.Reader) (string, error) {
	hash := sha256.New()
	if _, err := io.Copy(hash, reader); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// GenerateKeyFromMetadata creates a unique key based on metadata fields
func GenerateKeyFromMetadata(osType, osVersion, filename string) string {
	hash := sha256.New()
	hash.Write([]byte(osType))
	hash.Write([]byte(osVersion))
	hash.Write([]byte(filename))
	return hex.EncodeToString(hash.Sum(nil))[:32] // Use first 32 chars for readability
}
