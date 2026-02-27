package scraper

import (
	"testing"

	"github.com/go-rod/rod/lib/proto"
	"github.com/ysmood/gson"
)

func TestContentLengthExceedsLimit(t *testing.T) {
	headers := proto.NetworkHeaders{
		"Content-Length": gson.New("1024"),
	}

	if !contentLengthExceedsLimit(headers, 512) {
		t.Fatal("expected content length to exceed limit")
	}
	if contentLengthExceedsLimit(headers, 1024) {
		t.Fatal("did not expect content length to exceed equal limit")
	}
	if contentLengthExceedsLimit(headers, 2048) {
		t.Fatal("did not expect content length to exceed larger limit")
	}
}

func TestContentLengthExceedsLimit_GracefulOnMissingOrInvalidHeader(t *testing.T) {
	if contentLengthExceedsLimit(proto.NetworkHeaders{}, 1024) {
		t.Fatal("missing content-length should not exceed limit")
	}
	if contentLengthExceedsLimit(proto.NetworkHeaders{"Content-Length": gson.New("invalid")}, 1024) {
		t.Fatal("invalid content-length should not exceed limit")
	}
	if !contentLengthExceedsLimit(proto.NetworkHeaders{}, 0) {
		t.Fatal("non-positive limit should always be treated as exceeded")
	}
}
