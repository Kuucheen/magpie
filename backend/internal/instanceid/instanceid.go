package instanceid

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const envInstanceID = "MAGPIE_INSTANCE_ID"

var (
	once  sync.Once
	value string
)

func Get() string {
	once.Do(func() {
		resolved := strings.TrimSpace(os.Getenv(envInstanceID))
		if resolved == "" {
			resolved = generateUUID()
		}
		value = resolved
	})
	return value
}

func generateUUID() string {
	// RFC 4122 version 4 UUID.
	var b [16]byte
	if _, err := rand.Read(b[:]); err == nil {
		b[6] = (b[6] & 0x0f) | 0x40
		b[8] = (b[8] & 0x3f) | 0x80
		hexed := hex.EncodeToString(b[:])
		return hexed[0:8] + "-" + hexed[8:12] + "-" + hexed[12:16] + "-" + hexed[16:20] + "-" + hexed[20:32]
	}

	hostname, _ := os.Hostname()
	hostname = strings.TrimSpace(hostname)
	if hostname == "" {
		hostname = "instance"
	}
	return hostname + "-" + strconv.FormatInt(time.Now().UnixNano(), 10)
}

// ResetForTests clears cached instance identity. Intended for tests only.
func ResetForTests() {
	once = sync.Once{}
	value = ""
}
