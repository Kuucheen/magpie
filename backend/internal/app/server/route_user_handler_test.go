package server

import (
	"errors"
	"testing"

	"magpie/internal/database"
	"magpie/internal/domain"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCreateUserWithFirstAdminRole_RequiresConfiguredBootstrapToken(t *testing.T) {
	setupUserRegistrationTestDB(t)

	user := &domain.User{Email: "first@example.com", Password: "password-hash"}
	err := createUserWithFirstAdminRole(user, userRegistrationPolicy{})
	if !errors.Is(err, errAdminBootstrapTokenNotDefined) {
		t.Fatalf("expected errAdminBootstrapTokenNotDefined, got %v", err)
	}
}

func TestCreateUserWithFirstAdminRole_RequiresValidBootstrapTokenForFirstUser(t *testing.T) {
	setupUserRegistrationTestDB(t)

	user := &domain.User{Email: "first@example.com", Password: "password-hash"}
	err := createUserWithFirstAdminRole(user, userRegistrationPolicy{
		AdminBootstrapToken:    "expected-token",
		ProvidedBootstrapToken: "wrong-token",
	})
	if !errors.Is(err, errAdminBootstrapTokenInvalid) {
		t.Fatalf("expected errAdminBootstrapTokenInvalid, got %v", err)
	}
}

func TestCreateUserWithFirstAdminRole_AssignsAdminToFirstUser(t *testing.T) {
	setupUserRegistrationTestDB(t)

	user := &domain.User{Email: "first@example.com", Password: "password-hash"}
	err := createUserWithFirstAdminRole(user, userRegistrationPolicy{
		AdminBootstrapToken:    "expected-token",
		ProvidedBootstrapToken: "expected-token",
	})
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
	if err := createUserWithFirstAdminRole(admin, userRegistrationPolicy{
		AdminBootstrapToken:    "expected-token",
		ProvidedBootstrapToken: "expected-token",
	}); err != nil {
		t.Fatalf("bootstrap admin failed: %v", err)
	}

	blockedUser := &domain.User{Email: "second@example.com", Password: "password-hash"}
	err := createUserWithFirstAdminRole(blockedUser, userRegistrationPolicy{
		DisablePublicRegistration: true,
		AdminBootstrapToken:       "expected-token",
	})
	if !errors.Is(err, errPublicRegistrationDisabled) {
		t.Fatalf("expected errPublicRegistrationDisabled, got %v", err)
	}

	allowedUser := &domain.User{Email: "third@example.com", Password: "password-hash"}
	err = createUserWithFirstAdminRole(allowedUser, userRegistrationPolicy{
		DisablePublicRegistration: false,
		AdminBootstrapToken:       "expected-token",
	})
	if err != nil {
		t.Fatalf("expected follow-up user to be created, got %v", err)
	}
	if allowedUser.Role != "user" {
		t.Fatalf("expected follow-up user role user, got %q", allowedUser.Role)
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
