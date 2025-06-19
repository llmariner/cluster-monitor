package sender

import (
	"context"

	"github.com/go-logr/logr"
	v1 "github.com/llmariner/cluster-monitor/api/v1"
	"github.com/llmariner/rbac-manager/pkg/auth"
	"google.golang.org/grpc"
	ctrl "sigs.k8s.io/controller-runtime"
)

type telemetryClient interface {
	SendClusterTelemetry(context.Context, *v1.SendClusterTelemetryRequest, ...grpc.CallOption) (*v1.SendClusterTelemetryResponse, error)
}

// New creates a new sender instance.
func New(
	telemetryClient telemetryClient,
	payloadCh <-chan *v1.SendClusterTelemetryRequest_Payload,
) *S {
	return &S{
		telemetryClient: telemetryClient,
		payloadCh:       payloadCh,
	}
}

// S is a sender.
type S struct {
	telemetryClient telemetryClient
	payloadCh       <-chan *v1.SendClusterTelemetryRequest_Payload
	logger          logr.Logger

	// notifyCh is notified when the payload is sent. Only for testing.
	notifyCh chan struct{}
}

// SetupWithManager registers the sender  with the manager.
func (s *S) SetupWithManager(mgr ctrl.Manager) error {
	s.logger = mgr.GetLogger().WithName("sender")
	return mgr.Add(s)
}

// NeedLeaderElection implements LeaderElectionRunnable and always returns true.
func (s *S) NeedLeaderElection() bool {
	return true
}

// Start starts the sender.
func (s *S) Start(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case payload, ok := <-s.payloadCh:
			if !ok {
				return nil
			}

			s.logger.Info("Sending cluster telemetry")

			ctx = auth.AppendWorkerAuthorization(ctx)

			// TODO(kenji): Implement buffering.
			req := &v1.SendClusterTelemetryRequest{
				Payloads: []*v1.SendClusterTelemetryRequest_Payload{payload},
			}
			// TODO(kenji): Implement retry and/or gracefully handle the error.
			if _, err := s.telemetryClient.SendClusterTelemetry(ctx, req); err != nil {
				return err
			}

			if s.notifyCh != nil {
				s.notifyCh <- struct{}{}
			}
		}
	}
}
