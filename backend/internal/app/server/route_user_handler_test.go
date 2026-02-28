package server

import (
	"errors"
	"testing"

	"magpie/internal/config"
	"magpie/internal/database"
	"magpie/internal/domain"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCreateUserWithFirstAdminRole_AssignsAdminToFirstUser(t *testing.T) {
	setupUserRegistrationTestDB(t)

	user := &domain.User{Email: "first@example.com", Password: "password-hash"}
	err := createUserWithFirstAdminRole(user, userRegistrationPolicy{})
	if err != nil {
		t.Fatalf("createUserWithFirstAdminRole failed: %v", err)
	}

	if user.Role != "admin" {
		t.Fatalf("expected first user role admin, got %q", user.Role)
	}
}

func TestCreateUserWithFirstAdminRole_RespectsPublicRegistrationFlagAfterBootstrap(t *testing.T) {
	setupUserRegistrationTestDB(t)

	admin := &domain.User{Email: "admin@example.com", Password: "password-hash"}
	if err := createUserWithFirstAdminRole(admin, userRegistrationPolicy{}); err != nil {
		t.Fatalf("bootstrap admin failed: %v", err)
	}

	blockedUser := &domain.User{Email: "second@example.com", Password: "password-hash"}
	err := createUserWithFirstAdminRole(blockedUser, userRegistrationPolicy{
		DisablePublicRegistration: true,
	})
	if !errors.Is(err, errPublicRegistrationDisabled) {
		t.Fatalf("expected errPublicRegistrationDisabled, got %v", err)
	}

	allowedUser := &domain.User{Email: "third@example.com", Password: "password-hash"}
	err = createUserWithFirstAdminRole(allowedUser, userRegistrationPolicy{
		DisablePublicRegistration: false,
	})
	if err != nil {
		t.Fatalf("expected follow-up user to be created, got %v", err)
	}
	if allowedUser.Role != "user" {
		t.Fatalf("expected follow-up user role user, got %q", allowedUser.Role)
	}
}

func TestCreateUserWithFirstAdminRole_BlocksFirstAdminBootstrapWhenPolicyDisablesIt(t *testing.T) {
	setupUserRegistrationTestDB(t)

	user := &domain.User{Email: "first@example.com", Password: "password-hash"}
	err := createUserWithFirstAdminRole(user, userRegistrationPolicy{
		DisablePublicFirstAdminBootstrap: true,
	})
	if !errors.Is(err, errPublicFirstAdminBootstrap) {
		t.Fatalf("expected errPublicFirstAdminBootstrap, got %v", err)
	}
}

func TestResolveUserRegistrationPolicy_ProductionDefaults(t *testing.T) {
	prevProduction := config.InProductionMode
	config.SetProductionMode(true)
	t.Cleanup(func() {
		config.SetProductionMode(prevProduction)
	})

	policy := resolveUserRegistrationPolicy()
	if !policy.DisablePublicRegistration {
		t.Fatal("expected DisablePublicRegistration=true in production by default")
	}
	if !policy.DisablePublicFirstAdminBootstrap {
		t.Fatal("expected DisablePublicFirstAdminBootstrap=true in production by default")
	}
}

func TestResolveUserRegistrationPolicy_ProductionOverrides(t *testing.T) {
	prevProduction := config.InProductionMode
	config.SetProductionMode(true)
	t.Cleanup(func() {
		config.SetProductionMode(prevProduction)
	})

	t.Setenv(envDisablePublicRegistration, "false")
	t.Setenv(envEnablePublicFirstAdminBootstrap, "true")

	policy := resolveUserRegistrationPolicy()
	if policy.DisablePublicRegistration {
		t.Fatal("expected DisablePublicRegistration=false when override is set")
	}
	if policy.DisablePublicFirstAdminBootstrap {
		t.Fatal("expected DisablePublicFirstAdminBootstrap=false when override is set")
	}
}

func TestResolveUserRegistrationPolicy_LocalDefaultsRemainOpen(t *testing.T) {
	prevProduction := config.InProductionMode
	config.SetProductionMode(false)
	t.Cleanup(func() {
		config.SetProductionMode(prevProduction)
	})

	policy := resolveUserRegistrationPolicy()
	if policy.DisablePublicRegistration {
		t.Fatal("expected DisablePublicRegistration=false outside production by default")
	}
	if policy.DisablePublicFirstAdminBootstrap {
		t.Fatal("expected DisablePublicFirstAdminBootstrap=false outside production by default")
	}
}

func setupUserRegistrationTestDB(t *testing.T) {
	t.Helper()

	prevDB := database.DB
	t.Cleanup(func() {
		database.DB = prevDB
	})

	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	if _, err := database.SetupDB(func(cfg *database.Config) {
		cfg.ExistingDB = db
		cfg.AutoMigrate = true
		cfg.Migrations = []any{domain.User{}}
		cfg.SeedDefaults = false
	}); err != nil {
		t.Fatalf("setup db: %v", err)
	}
}
