package database

import (
	"errors"
	"strings"
	"testing"

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
