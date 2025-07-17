package collector

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/llmariner/cluster-monitor/agent/internal/collector/prometheus"
	v1 "github.com/llmariner/cluster-monitor/api/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const defaultUpdateInterval = 10 * time.Minute

type collector interface {
	collect(ctx context.Context) (*v1.SendClusterTelemetryRequest_Payload, error)
}

// New returns a new collector instance.
func New(prometheusURL string, targetNodeSelector map[string]string) *C {
	return &C{
		prometheusURL:      prometheusURL,
		targetNodeSelector: targetNodeSelector,
		updateInterval:     defaultUpdateInterval,
		payloadCh:          make(chan *v1.SendClusterTelemetryRequest_Payload),
	}
}

// C is a collector that collects telemetry data.
type C struct {
	prometheusURL string

	targetNodeSelector map[string]string

	collectors []collector

	updateInterval time.Duration

	payloadCh chan *v1.SendClusterTelemetryRequest_Payload

	k8sClient client.Client
	logger    logr.Logger
}

// SetupWithManager registers the collector with the manager.
func (c *C) SetupWithManager(mgr ctrl.Manager) error {
	c.k8sClient = mgr.GetClient()
	c.logger = mgr.GetLogger().WithName("collector")

	c.collectors = []collector{
		newClusterSnapshotCollector(c.k8sClient, c.targetNodeSelector, c.logger),
		// TODO(kenji): Add more collectors.
	}

	if c.prometheusURL != "" {
		c.logger.Info("Adding GPUTelemetryCollector", "prometheusURL", c.prometheusURL)
		client, err := prometheus.NewClient(c.prometheusURL, c.logger)
		if err != nil {
			return err
		}

		cl := newGPUTelemetryCollector(client, c.k8sClient, c.targetNodeSelector, defaultUpdateInterval, c.logger)
		c.collectors = append(c.collectors, cl)
	}

	return mgr.Add(c)
}

// NeedLeaderElection implements LeaderElectionRunnable and always returns true.
func (c *C) NeedLeaderElection() bool {
	return true
}

// Start starts the collector.
func (c *C) Start(ctx context.Context) error {
	if err := c.collect(ctx); err != nil {
		return err
	}

	tick := time.NewTicker(c.updateInterval)
	defer tick.Stop()

	for {
		select {
		case <-tick.C:
			if err := c.collect(ctx); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()

		}
	}
}

func (c *C) collect(ctx context.Context) error {
	c.logger.Info("Collecting telemetry data")

	for _, collector := range c.collectors {
		paylod, err := collector.collect(ctx)
		if err != nil {
			// TODO(kenji): Gracefully handle the error.
			return err
		}
		c.payloadCh <- paylod
	}

	return nil
}

// PayloadCh returns a read-only channel that can be used to receive telemetry payloads.
func (c *C) PayloadCh() <-chan *v1.SendClusterTelemetryRequest_Payload {
	return c.payloadCh
}
