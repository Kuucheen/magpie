package support

import "testing"

func TestValidatePassword_AcceptsStrongPassword(t *testing.T) {
	if err := ValidatePassword("StrongPassword1"); err != nil {
		t.Fatalf("ValidatePassword returned error: %v", err)
	}
}

func TestValidatePassword_RejectsWeakPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
	}{
		{name: "too short", password: "Short1Aa"},
		{name: "missing upper", password: "lowercasepass1"},
		{name: "missing lower", password: "UPPERCASEPASS1"},
		{name: "missing digit", password: "NoDigitsHereA"},
		{name: "contains space", password: "Bad Password1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidatePassword(tc.password); err == nil {
				t.Fatalf("ValidatePassword(%q) unexpectedly succeeded", tc.password)
			}
		})
	}
}
