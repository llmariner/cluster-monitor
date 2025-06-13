package collector

import (
	"context"

	"github.com/go-logr/logr"
	v1 "github.com/llmariner/cluster-monitor/api/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// New return a new collector instance.
func New() *C {
	return &C{
		payloadCh: make(chan *v1.SendClusterTelemetryRequest_Payload),
	}
}

// C is a collector that collects telemetry data.
type C struct {
	payloadCh chan *v1.SendClusterTelemetryRequest_Payload

	k8sClient client.Client
	logger    logr.Logger
}

// SetupWithManager registers the collector with the manager.
func (c *C) SetupWithManager(mgr ctrl.Manager) error {
	c.k8sClient = mgr.GetClient()
	c.logger = mgr.GetLogger().WithName("collector")
	return mgr.Add(c)
}

// NeedLeaderElection implements LeaderElectionRunnable and always returns true.
func (c *C) NeedLeaderElection() bool {
	return true
}

// Start starts the collector.
func (c *C) Start(ctx context.Context) error {
	// TODO(kenji): Implement the collector logic here.
	return nil
}

// PayloadCh returns a read-only channel that can be used to receive telemetry payloads.
func (c *C) PayloadCh() <-chan *v1.SendClusterTelemetryRequest_Payload {
	return c.payloadCh
}
