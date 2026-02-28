package instanceid

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

const (
	envInstanceID           = "MAGPIE_INSTANCE_ID"
	envInstanceIDFile       = "MAGPIE_INSTANCE_ID_FILE"
	envInstanceName         = "MAGPIE_INSTANCE_NAME"
	envInstanceRegion       = "MAGPIE_INSTANCE_REGION"
	envInstanceScope        = "MAGPIE_INSTANCE_SCOPE"
	envBackendPort          = "BACKEND_PORT"
	envBackendPortLegacy    = "backend-port"
	envRotatingPortStart    = "ROTATING_PROXY_PORT_START"
	envRotatingPortEnd      = "ROTATING_PROXY_PORT_END"
	envPodName              = "POD_NAME"
	envPodUID               = "POD_UID"
	envHostname             = "HOSTNAME"
	defaultBackendPort      = 5656
	defaultRotatingPortFrom = 20000
	defaultRotatingPortTo   = 20100
	instanceIDFileMode      = 0o600
	instanceIDDirMode       = 0o700
)

var (
	once           sync.Once
	value          string
	errIDFileEmpty = errors.New("instance id file is empty")
)

func Get() string {
	once.Do(func() {
		resolved := strings.TrimSpace(os.Getenv(envInstanceID))
		if resolved == "" {
			resolved = resolveFileBackedID()
		}
		if resolved == "" {
			resolved = generateStableFallbackID()
		}
		value = resolved
	})
	return value
}

func resolveFileBackedID() string {
	path := resolveConfiguredInstanceIDPath()
	if path == "" {
		return ""
	}

	if persisted, err := readPersistedID(path); err == nil {
		return persisted
	}

	if err := os.MkdirAll(filepath.Dir(path), instanceIDDirMode); err == nil {
		candidate := generateUUID()
		if err := createIDFile(path, candidate); err == nil {
			return candidate
		}
		if persisted, err := readPersistedID(path); err == nil {
			return persisted
		}
	}

	return ""
}

func readPersistedID(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	resolved := strings.TrimSpace(string(data))
	if resolved == "" {
		return "", errIDFileEmpty
	}
	return resolved, nil
}

func createIDFile(path string, id string) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, instanceIDFileMode)
	if err != nil {
		return err
	}

	_, writeErr := file.WriteString(id + "\n")
	syncErr := file.Sync()
	closeErr := file.Close()
	if writeErr != nil || syncErr != nil || closeErr != nil {
		_ = os.Remove(path)
		return errors.Join(writeErr, syncErr, closeErr)
	}

	return nil
}

func resolveConfiguredInstanceIDPath() string {
	return strings.TrimSpace(os.Getenv(envInstanceIDFile))
}

func generateUUID() string {
	// RFC 4122 version 4 UUID.
	var b [16]byte
	if _, err := rand.Read(b[:]); err == nil {
		b[6] = (b[6] & 0x0f) | 0x40 // Version 4
		b[8] = (b[8] & 0x3f) | 0x80 // RFC 4122 variant
		return formatUUID(b)
	}

	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "localhost"
	}
	sum := sha256.Sum256([]byte(hostname))
	var fallback [16]byte
	copy(fallback[:], sum[:16])
	fallback[6] = (fallback[6] & 0x0f) | 0x50 // Version 5
	fallback[8] = (fallback[8] & 0x3f) | 0x80 // RFC 4122 variant
	return formatUUID(fallback)
}

func generateStableFallbackID() string {
	seed := fallbackSeed()
	if seed == "" {
		return generateUUID()
	}
	sum := sha256.Sum256([]byte(seed))
	var b [16]byte
	copy(b[:], sum[:16])
	b[6] = (b[6] & 0x0f) | 0x50 // Version 5
	b[8] = (b[8] & 0x3f) | 0x80 // RFC 4122 variant
	return formatUUID(b)
}

func fallbackSeed() string {
	if scoped := strings.TrimSpace(os.Getenv(envInstanceScope)); scoped != "" {
		return "scope=" + scoped
	}

	parts := make([]string, 0, 12)
	appendEnvPart := func(label, key string) {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			parts = append(parts, label+"="+value)
		}
	}
	appendEnvPart("instance_name", envInstanceName)
	appendEnvPart("instance_region", envInstanceRegion)
	appendEnvPart("pod_name", envPodName)
	appendEnvPart("pod_uid", envPodUID)
	appendEnvPart("hostname_env", envHostname)

	startPort := envIntOrDefault(envRotatingPortStart, defaultRotatingPortFrom)
	endPort := envIntOrDefault(envRotatingPortEnd, defaultRotatingPortTo)
	if endPort < startPort {
		startPort, endPort = endPort, startPort
	}
	parts = append(parts, fmt.Sprintf("rotating_port_range=%d-%d", startPort, endPort))

	backendPort := resolveBackendPort()
	parts = append(parts, fmt.Sprintf("backend_port=%d", backendPort))

	if machineID := readMachineID(); machineID != "" {
		parts = append(parts, "machine_id="+machineID)
	}
	if hostname, _ := os.Hostname(); strings.TrimSpace(hostname) != "" {
		parts = append(parts, "hostname="+strings.TrimSpace(hostname))
	}

	return strings.Join(parts, "|")
}

func resolveBackendPort() int {
	if value := parseEnvInt(envBackendPort); value > 0 {
		return value
	}
	if value := parseEnvInt(envBackendPortLegacy); value > 0 {
		return value
	}
	if value := parseBackendPortFromArgs(); value > 0 {
		return value
	}
	return defaultBackendPort
}

func parseBackendPortFromArgs() int {
	for idx := 0; idx < len(os.Args); idx++ {
		arg := strings.TrimSpace(os.Args[idx])
		if strings.HasPrefix(arg, "-backend-port=") {
			if parsed, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(arg, "-backend-port="))); err == nil && parsed > 0 {
				return parsed
			}
			continue
		}
		if arg == "-backend-port" && idx+1 < len(os.Args) {
			if parsed, err := strconv.Atoi(strings.TrimSpace(os.Args[idx+1])); err == nil && parsed > 0 {
				return parsed
			}
		}
	}
	return 0
}

func envIntOrDefault(key string, fallback int) int {
	if parsed := parseEnvInt(key); parsed != 0 {
		return parsed
	}
	return fallback
}

func parseEnvInt(key string) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return 0
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}
	return parsed
}

func readMachineID() string {
	paths := []string{"/etc/machine-id", "/var/lib/dbus/machine-id"}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		value := strings.TrimSpace(string(data))
		if value != "" {
			return value
		}
	}
	return ""
}

func formatUUID(b [16]byte) string {
	hexed := hex.EncodeToString(b[:])
	return hexed[0:8] + "-" + hexed[8:12] + "-" + hexed[12:16] + "-" + hexed[16:20] + "-" + hexed[20:32]
}

// ResetForTests clears cached instance identity. Intended for tests only.
func ResetForTests() {
	once = sync.Once{}
	value = ""
}
