package daemon

import (
	"context"
	supportlog "github.com/HashCash-Consultants/go/support/log"
	"runtime"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/HashCash-Consultants/go/clients/hcnetcore"
	proto "github.com/HashCash-Consultants/go/protocols/hcnetcore"
	"github.com/HashCash-Consultants/go/support/logmetrics"
	"github.com/HashCash-Consultants/go/xdr"

	"github.com/HashCash-Consultants/soroban-rpc/cmd/soroban-rpc/internal/config"
	"github.com/HashCash-Consultants/soroban-rpc/cmd/soroban-rpc/internal/daemon/interfaces"
)

func (d *Daemon) registerMetrics() {
	// LogMetricsHook is a metric which counts log lines emitted by soroban rpc
	logMetricsHook := logmetrics.New(prometheusNamespace)
	d.logger.AddHook(logMetricsHook)
	for _, counter := range logMetricsHook {
		d.metricsRegistry.MustRegister(counter)
	}

	buildInfoGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{Namespace: prometheusNamespace, Subsystem: "build", Name: "info"},
		[]string{"version", "goversion", "commit", "branch", "build_timestamp"},
	)
	buildInfoGauge.With(prometheus.Labels{
		"version":         config.Version,
		"commit":          config.CommitHash,
		"branch":          config.Branch,
		"build_timestamp": config.BuildTimestamp,
		"goversion":       runtime.Version(),
	}).Inc()

	d.metricsRegistry.MustRegister(prometheus.NewGoCollector())
	d.metricsRegistry.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	d.metricsRegistry.MustRegister(buildInfoGauge)
}

func (d *Daemon) MetricsRegistry() *prometheus.Registry {
	return d.metricsRegistry
}

func (d *Daemon) MetricsNamespace() string {
	return prometheusNamespace
}

type CoreClientWithMetrics struct {
	hcnetcore.Client
	submitMetric  *prometheus.SummaryVec
	opCountMetric *prometheus.SummaryVec
}

func newCoreClientWithMetrics(client hcnetcore.Client, registry *prometheus.Registry) *CoreClientWithMetrics {
	submitMetric := prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Namespace: prometheusNamespace, Subsystem: "txsub", Name: "submission_duration_seconds",
		Help:       "submission durations to Hcnet-Core, sliding window = 10m",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	}, []string{"status"})
	opCountMetric := prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Namespace: prometheusNamespace, Subsystem: "txsub", Name: "operation_count",
		Help:       "number of operations included in a transaction, sliding window = 10m",
		Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
	}, []string{"status"})
	registry.MustRegister(submitMetric, opCountMetric)

	return &CoreClientWithMetrics{
		Client:        client,
		submitMetric:  submitMetric,
		opCountMetric: opCountMetric,
	}
}

func (c *CoreClientWithMetrics) SubmitTransaction(ctx context.Context, envelopeBase64 string) (*proto.TXResponse, error) {
	var envelope xdr.TransactionEnvelope
	err := xdr.SafeUnmarshalBase64(envelopeBase64, &envelope)
	if err != nil {
		return nil, err
	}

	startTime := time.Now()
	response, err := c.Client.SubmitTransaction(ctx, envelopeBase64)
	duration := time.Since(startTime).Seconds()

	var label prometheus.Labels
	if err != nil {
		label = prometheus.Labels{"status": "request_error"}
	} else if response.IsException() {
		label = prometheus.Labels{"status": "exception"}
	} else {
		label = prometheus.Labels{"status": response.Status}
	}

	c.submitMetric.With(label).Observe(duration)
	c.opCountMetric.With(label).Observe(float64(len(envelope.Operations())))
	return response, err
}

func (d *Daemon) CoreClient() interfaces.CoreClient {
	return d.coreClient
}

func (d *Daemon) Logger() *supportlog.Entry {
	return d.logger
}
