package config

import (
	"os"
	"strconv"
	"strings"
)

const envStrictSecretValidation = "STRICT_SECRET_VALIDATION"
const (
	envAppEnvironment     = "APP_ENV"
	envRuntimeEnvironment = "ENVIRONMENT"
	envGoEnvironment      = "GO_ENV"
	envMagpieEnvironment  = "MAGPIE_ENV"
)

func StrictSecretValidationEnabled() bool {
	defaultValue := InProductionMode || RuntimeEnvironmentIndicatesProduction()

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

func runtimeEnvironmentIndicatesProduction() bool {
	return RuntimeEnvironmentIndicatesProduction()
}

func RuntimeEnvironmentIndicatesProduction() bool {
	for _, key := range []string{envAppEnvironment, envRuntimeEnvironment, envGoEnvironment, envMagpieEnvironment} {
		value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
		switch value {
		case "prod", "production":
			return true
		}
	}
	return false
}
