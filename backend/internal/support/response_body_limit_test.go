package support

import (
	"errors"
	"strings"
	"testing"
)

func TestReadAllWithLimit_ReturnsBodyWithinLimit(t *testing.T) {
	body, err := ReadAllWithLimit(strings.NewReader("hello"), 5)
	if err != nil {
		t.Fatalf("ReadAllWithLimit returned error: %v", err)
	}
	if string(body) != "hello" {
		t.Fatalf("body = %q, want %q", string(body), "hello")
	}
}

func TestReadAllWithLimit_RejectsBodyExceedingLimit(t *testing.T) {
	_, err := ReadAllWithLimit(strings.NewReader("hello"), 4)
	if !errors.Is(err, ErrResponseBodyTooLarge) {
		t.Fatalf("error = %v, want ErrResponseBodyTooLarge", err)
	}
}
