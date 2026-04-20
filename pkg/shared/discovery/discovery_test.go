package discovery

import (
	"context"
	"testing"
)

func TestResolveGRPCAddrFallbackWithoutNATS(t *testing.T) {
	addr, err := ResolveGRPCAddr(context.Background(), nil, "meta-service", "localhost:8080")
	if err != nil {
		t.Fatalf("resolve fallback: %v", err)
	}
	if addr != "localhost:8080" {
		t.Fatalf("expected fallback localhost:8080, got %s", addr)
	}
}

func TestEffectiveAdvertiseAddr(t *testing.T) {
	cases := []struct {
		name      string
		grpcAddr  string
		advertise string
		want      string
	}{
		{name: "explicit advertise", grpcAddr: ":8080", advertise: "query-engine:8080", want: "query-engine:8080"},
		{name: "listen all interfaces", grpcAddr: ":8080", want: "localhost:8080"},
		{name: "host preserved", grpcAddr: "127.0.0.1:8080", want: "127.0.0.1:8080"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := effectiveAdvertiseAddr(tc.grpcAddr, tc.advertise)
			if got != tc.want {
				t.Fatalf("expected %s, got %s", tc.want, got)
			}
		})
	}
}
