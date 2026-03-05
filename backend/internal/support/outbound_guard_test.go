package support

import (
	"context"
	"errors"
	"net/netip"
	"testing"
)

type fakeNetIPResolver struct {
	lookup func(ctx context.Context, network string, host string) ([]netip.Addr, error)
}

func (f fakeNetIPResolver) LookupNetIP(ctx context.Context, network string, host string) ([]netip.Addr, error) {
	if f.lookup == nil {
		return nil, nil
	}
	return f.lookup(ctx, network, host)
}

func TestValidateOutboundHTTPLiteral_BlocksUnsafeHosts(t *testing.T) {
	tests := []string{
		"http://127.0.0.1:8080",
		"https://10.0.0.12/resource",
		"http://[::1]/",
		"http://localhost:80",
		"http://internal.localhost",
	}

	for _, rawURL := range tests {
		t.Run(rawURL, func(t *testing.T) {
			if _, err := ValidateOutboundHTTPLiteral(rawURL); !errors.Is(err, ErrUnsafeOutboundTarget) {
				t.Fatalf("ValidateOutboundHTTPLiteral(%q) err = %v, want ErrUnsafeOutboundTarget", rawURL, err)
			}
		})
	}
}

func TestValidateOutboundHTTPLiteral_AllowsPublicDomain(t *testing.T) {
	if _, err := ValidateOutboundHTTPLiteral("https://example.com/path"); err != nil {
		t.Fatalf("ValidateOutboundHTTPLiteral public domain error: %v", err)
	}
}

func TestResolveAllowedOutboundIPs_BlocksHostResolvingToPrivateRanges(t *testing.T) {
	resolver := fakeNetIPResolver{
		lookup: func(context.Context, string, string) ([]netip.Addr, error) {
			return []netip.Addr{
				netip.MustParseAddr("93.184.216.34"),
				netip.MustParseAddr("127.0.0.1"),
			}, nil
		},
	}

	_, err := resolveAllowedOutboundIPs(context.Background(), "example.com", resolver)
	if !errors.Is(err, ErrUnsafeOutboundTarget) {
		t.Fatalf("resolveAllowedOutboundIPs err = %v, want ErrUnsafeOutboundTarget", err)
	}
}

func TestResolveAllowedOutboundIPs_AllowsPublicAddresses(t *testing.T) {
	resolver := fakeNetIPResolver{
		lookup: func(context.Context, string, string) ([]netip.Addr, error) {
			return []netip.Addr{
				netip.MustParseAddr("93.184.216.34"),
				netip.MustParseAddr("2606:2800:220:1:248:1893:25c8:1946"),
			}, nil
		},
	}

	ips, err := resolveAllowedOutboundIPs(context.Background(), "example.com", resolver)
	if err != nil {
		t.Fatalf("resolveAllowedOutboundIPs error: %v", err)
	}
	if len(ips) != 2 {
		t.Fatalf("resolved ips = %d, want 2", len(ips))
	}
}

func TestValidateOutboundHTTPLiteral_CanBeRelaxedForLocalDevelopment(t *testing.T) {
	t.Setenv(envAllowPrivateNetworkEgress, "true")

	if _, err := ValidateOutboundHTTPLiteral("http://127.0.0.1:9000"); err != nil {
		t.Fatalf("expected private target to be allowed when %s=true: %v", envAllowPrivateNetworkEgress, err)
	}
}

func TestValidateOutboundIPLiteral_BlocksPrivateRanges(t *testing.T) {
	if err := ValidateOutboundIPLiteral("127.0.0.1"); !errors.Is(err, ErrUnsafeOutboundTarget) {
		t.Fatalf("ValidateOutboundIPLiteral err = %v, want ErrUnsafeOutboundTarget", err)
	}
}

func TestValidateOutboundIPLiteral_AllowsPublicIP(t *testing.T) {
	if err := ValidateOutboundIPLiteral("93.184.216.34"); err != nil {
		t.Fatalf("ValidateOutboundIPLiteral public IP error: %v", err)
	}
}

func TestValidateOutboundIPLiteral_AllowsPrivateWhenEgressOverrideEnabled(t *testing.T) {
	t.Setenv(envAllowPrivateNetworkEgress, "true")

	if err := ValidateOutboundIPLiteral("127.0.0.1"); err != nil {
		t.Fatalf("ValidateOutboundIPLiteral should allow private ip with override: %v", err)
	}
}
