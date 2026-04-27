package id

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
)

// ID length constants
const (
	EntryLen   = 6 // hex chars - 16^6 = 16.7M possible entries
	ArticleLen = 6 // hex chars per article hash portion
)

var (
	mu      sync.Mutex
	entropy []byte
)

func init() {
	// Pre-generate entropy for faster ID generation
	entropy = make([]byte, 32)
	rand.Read(entropy)
}

// Entry generates a new entry ID (e.g., "a3f2b1")
func Entry() string {
	return generate(EntryLen)
}

// Article generates a new article ID (e.g., "a3f2b1-c7d4e9")
func Article(entryID string) string {
	return fmt.Sprintf("%s-%s", entryID, generate(ArticleLen))
}

func generate(length int) string {
	mu.Lock()
	defer mu.Unlock()

	// Mix in random bytes for unpredictability
	entropy = mixEntropy(entropy)

	// Convert to hex string
	b := make([]byte, length/2+1)
	copy(b, entropy)

	hexStr := hex.EncodeToString(b)[:length]
	return hexStr
}

func mixEntropy(e []byte) []byte {
	// XOR with fresh random data periodically
	var fresh [32]byte
	rand.Read(fresh[:])

	for i := range e {
		e[i] ^= fresh[i]
	}

	// Rotate
	n := int(fresh[0]) % len(e)
	result := make([]byte, len(e))
	copy(result, e[n:])
	copy(result[len(e)-n:], e[:n])

	return result
}
