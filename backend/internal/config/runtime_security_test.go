package config

import "testing"

func TestStrictSecretValidationEnabled_DefaultsToFalseOutsideProduction(t *testing.T) {
	prev := InProductionMode
	InProductionMode = false
	t.Cleanup(func() {
		InProductionMode = prev
	})

	t.Setenv(envStrictSecretValidation, "")
	t.Setenv(envAppEnvironment, "")
	t.Setenv(envRuntimeEnvironment, "")
	t.Setenv(envGoEnvironment, "")
	t.Setenv(envMagpieEnvironment, "")

	if StrictSecretValidationEnabled() {
		t.Fatal("expected strict secret validation to default false outside production")
	}
}

func TestStrictSecretValidationEnabled_DefaultsToTrueWhenRuntimeEnvIsProduction(t *testing.T) {
	prev := InProductionMode
	InProductionMode = false
	t.Cleanup(func() {
		InProductionMode = prev
	})

	t.Setenv(envStrictSecretValidation, "")
	t.Setenv(envAppEnvironment, "production")

	if !StrictSecretValidationEnabled() {
		t.Fatal("expected strict secret validation to default true when APP_ENV=production")
	}
}

func TestStrictSecretValidationEnabled_ExplicitOverrideWins(t *testing.T) {
	prev := InProductionMode
	InProductionMode = false
	t.Cleanup(func() {
		InProductionMode = prev
	})

	t.Setenv(envAppEnvironment, "production")
	t.Setenv(envStrictSecretValidation, "false")

	if StrictSecretValidationEnabled() {
		t.Fatal("expected explicit STRICT_SECRET_VALIDATION=false override to disable strict validation")
	}
}

func TestRuntimeEnvironmentIndicatesProduction_RecognizesCommonEnvs(t *testing.T) {
	t.Setenv(envAppEnvironment, "")
	t.Setenv(envRuntimeEnvironment, "")
	t.Setenv(envGoEnvironment, "")
	t.Setenv(envMagpieEnvironment, "prod")

	if !runtimeEnvironmentIndicatesProduction() {
		t.Fatal("expected MAGPIE_ENV=prod to be recognized as production")
	}
}
