package database

import (
	"errors"
	"strings"
	"testing"

	"magpie/internal/domain"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRunSchemaEnsureSteps_StopsAtFirstFailure(t *testing.T) {
	expectedErr := errors.New("boom")
	var calls []string

	err := runSchemaEnsureSteps(nil, []schemaEnsureStep{
		{
			name: "step one",
			run: func(db *gorm.DB) error {
				calls = append(calls, "step one")
				return nil
			},
		},
		{
			name: "step two",
			run: func(db *gorm.DB) error {
				calls = append(calls, "step two")
				return expectedErr
			},
		},
		{
			name: "step three",
			run: func(db *gorm.DB) error {
				calls = append(calls, "step three")
				return nil
			},
		},
	})
	if err == nil {
		t.Fatal("runSchemaEnsureSteps returned nil error")
	}
	if !errors.Is(err, expectedErr) {
		t.Fatalf("runSchemaEnsureSteps error = %v, want wrapped %v", err, expectedErr)
	}
	if !strings.Contains(err.Error(), "step two") {
		t.Fatalf("runSchemaEnsureSteps error = %q, expected failing step name", err.Error())
	}
	if len(calls) != 2 {
		t.Fatalf("runSchemaEnsureSteps executed %d steps, want 2", len(calls))
	}
}

func TestEnsureUserAuthSchema_NormalizesEmailsAndCreatesCaseInsensitiveUniqueness(t *testing.T) {
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	if err := db.AutoMigrate(&domain.User{}); err != nil {
		t.Fatalf("migrate users: %v", err)
	}

	users := []domain.User{
		{Email: " MixedCase@Example.com ", Password: "hash-one", Role: "user"},
		{Email: "another@example.com", Password: "hash-two", Role: "user"},
	}
	if err := db.Create(&users).Error; err != nil {
		t.Fatalf("seed users: %v", err)
	}

	if err := ensureUserAuthSchema(db); err != nil {
		t.Fatalf("ensureUserAuthSchema: %v", err)
	}

	var stored []domain.User
	if err := db.Order("id ASC").Find(&stored).Error; err != nil {
		t.Fatalf("load users: %v", err)
	}
	if got := stored[0].Email; got != "mixedcase@example.com" {
		t.Fatalf("normalized email = %q, want %q", got, "mixedcase@example.com")
	}

	err = db.Create(&domain.User{Email: "MIXEDCASE@example.com", Password: "hash-three", Role: "user"}).Error
	if err == nil {
		t.Fatal("expected case-insensitive uniqueness violation")
	}
	normalized := strings.ToLower(err.Error())
	if !strings.Contains(normalized, "unique constraint failed") &&
		!strings.Contains(normalized, "duplicate key value violates unique constraint") {
		t.Fatalf("unexpected uniqueness error: %v", err)
	}
}

func TestEnsureUserAuthSchema_RejectsDuplicateCanonicalEmails(t *testing.T) {
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	if err := db.AutoMigrate(&domain.User{}); err != nil {
		t.Fatalf("migrate users: %v", err)
	}

	users := []domain.User{
		{Email: "duplicate@example.com", Password: "hash-one", Role: "user"},
		{Email: "DUPLICATE@example.com", Password: "hash-two", Role: "user"},
	}
	if err := db.Create(&users).Error; err != nil {
		t.Fatalf("seed users: %v", err)
	}

	err = ensureUserAuthSchema(db)
	if err == nil {
		t.Fatal("expected duplicate canonical email error")
	}
	if !strings.Contains(err.Error(), "duplicate user emails exist after normalization") {
		t.Fatalf("unexpected error: %v", err)
	}
}
