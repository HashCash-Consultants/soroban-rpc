package interfaces

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	proto "github.com/HashCash-Consultants/go/protocols/hcnetcore"
)

// Daemon defines the interface that the Daemon would be implementing.
// this would be useful for decoupling purposes, allowing to test components without
// the actual daemon.
type Daemon interface {
	MetricsRegistry() *prometheus.Registry
	MetricsNamespace() string
	CoreClient() CoreClient
}

type CoreClient interface {
	Info(ctx context.Context) (*proto.InfoResponse, error)
	SubmitTransaction(context.Context, string) (*proto.TXResponse, error)
}
