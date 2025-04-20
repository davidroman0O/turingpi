package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"sort"
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

// GenerateKeyFromTags creates a unique key based on tag values
func GenerateKeyFromTags(tags map[string]string) string {
	hash := sha256.New()

	// Sort tags by key for consistent hashing
	keys := make([]string, 0, len(tags))
	for k := range tags {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Hash tag key-value pairs in sorted order
	for _, k := range keys {
		hash.Write([]byte(k))
		hash.Write([]byte(tags[k]))
	}

	return hex.EncodeToString(hash.Sum(nil))[:32] // Use first 32 chars for readability
}
