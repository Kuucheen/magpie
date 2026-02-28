package config

import (
	"os"
	"strconv"
	"strings"
)

const envStrictSecretValidation = "STRICT_SECRET_VALIDATION"

func StrictSecretValidationEnabled() bool {
	defaultValue := InProductionMode

	raw, exists := os.LookupEnv(envStrictSecretValidation)
	if !exists {
		return defaultValue
	}

	parsed, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		return defaultValue
	}

	return parsed
}
