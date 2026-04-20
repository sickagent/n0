package discovery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
	"n0/pkg/shared/natsclient"
)

const grpcDiscoveryPrefix = "discovery.grpc."

type grpcAddressResponse struct {
	GRPCAddr string `json:"grpc_addr"`
}

// ResolveGRPCAddr asks NATS for the current gRPC address of a service.
// If no responder is available, fallbackAddr is used when provided.
func ResolveGRPCAddr(ctx context.Context, nc *natsclient.Client, serviceName, fallbackAddr string) (string, error) {
	if nc != nil {
		msg, err := nc.Conn.RequestWithContext(ctx, grpcDiscoverySubject(serviceName), nil)
		switch {
		case err == nil:
			var resp grpcAddressResponse
			if err := json.Unmarshal(msg.Data, &resp); err != nil {
				return "", fmt.Errorf("decode discovery response for %s: %w", serviceName, err)
			}
			if resp.GRPCAddr == "" {
				return "", fmt.Errorf("empty grpc address returned for %s", serviceName)
			}
			return resp.GRPCAddr, nil
		case errors.Is(err, nats.ErrNoResponders):
			// Fall back to static config below.
		default:
			if fallbackAddr == "" {
				return "", fmt.Errorf("resolve grpc address for %s: %w", serviceName, err)
			}
		}
	}

	if fallbackAddr == "" {
		return "", fmt.Errorf("no grpc address available for %s", serviceName)
	}
	return fallbackAddr, nil
}

// RegisterGRPCResponder exposes the current gRPC address of a service over NATS request/reply.
func RegisterGRPCResponder(nc *natsclient.Client, serviceName, grpcAddr, advertiseAddr string, log *zap.Logger) (*nats.Subscription, error) {
	if nc == nil {
		return nil, fmt.Errorf("nats client is required for grpc discovery responder")
	}

	effectiveAddr := effectiveAdvertiseAddr(grpcAddr, advertiseAddr)
	if effectiveAddr == "" {
		return nil, fmt.Errorf("empty grpc advertise address for %s", serviceName)
	}

	sub, err := nc.Conn.Subscribe(grpcDiscoverySubject(serviceName), func(msg *nats.Msg) {
		payload, err := json.Marshal(grpcAddressResponse{GRPCAddr: effectiveAddr})
		if err != nil {
			if log != nil {
				log.Warn("failed to marshal grpc discovery response", zap.String("service", serviceName), zap.Error(err))
			}
			return
		}
		if err := msg.Respond(payload); err != nil && log != nil {
			log.Warn("failed to respond to grpc discovery request", zap.String("service", serviceName), zap.Error(err))
		}
	})
	if err != nil {
		return nil, fmt.Errorf("subscribe grpc discovery for %s: %w", serviceName, err)
	}

	if log != nil {
		log.Info("grpc discovery responder registered",
			zap.String("service", serviceName),
			zap.String("grpc_addr", effectiveAddr),
		)
	}
	return sub, nil
}

func grpcDiscoverySubject(serviceName string) string {
	return grpcDiscoveryPrefix + serviceName
}

func effectiveAdvertiseAddr(grpcAddr, advertiseAddr string) string {
	addr := strings.TrimSpace(advertiseAddr)
	if addr != "" {
		return addr
	}

	host, port, err := net.SplitHostPort(strings.TrimSpace(grpcAddr))
	if err != nil {
		return strings.TrimSpace(grpcAddr)
	}

	switch host {
	case "", "0.0.0.0", "::":
		return net.JoinHostPort("localhost", port)
	default:
		return net.JoinHostPort(host, port)
	}
}
