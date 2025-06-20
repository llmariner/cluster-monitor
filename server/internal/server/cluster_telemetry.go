package server

import (
	"context"

	v1 "github.com/llmariner/cluster-monitor/api/v1"
	"github.com/llmariner/cluster-monitor/server/internal/store"
	"github.com/llmariner/rbac-manager/pkg/auth"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// SendClusterTelemetry processes the telemetry data sent from the client.
func (ws *WS) SendClusterTelemetry(
	ctx context.Context,
	req *v1.SendClusterTelemetryRequest,
) (*v1.SendClusterTelemetryResponse, error) {
	clusterInfo, err := ws.extractClusterInfoFromContext(ctx)
	if err != nil {
		return nil, err
	}

	if len(req.Payloads) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "no payloads provided")
	}

	for _, payload := range req.Payloads {
		switch payload.MessageKind.(type) {
		case *v1.SendClusterTelemetryRequest_Payload_ClusterSnapshot:
			if err := ws.processClusterSnapshot(ctx, clusterInfo, payload.GetClusterSnapshot()); err != nil {
				return nil, status.Errorf(codes.Internal, "failed to process cluster snapshot: %v", err)
			}
		default:
			return nil, status.Errorf(codes.InvalidArgument, "unsupported message kind: %T", payload.MessageKind)
		}
	}

	return &v1.SendClusterTelemetryResponse{}, nil
}

func (ws *WS) processClusterSnapshot(
	ctx context.Context,
	clusterInfo *auth.ClusterInfo,
	csProto *v1.ClusterSnapshot,
) error {
	cs := &store.ClusterSnapshot{
		ClusterID: clusterInfo.ClusterID,
		Name:      clusterInfo.ClusterName,
	}
	if err := ws.store.CreateOrUpdateClusterSnapshot(cs); err != nil {
		return err
	}

	msg, err := proto.Marshal(csProto)
	if err != nil {
		return err
	}

	csh := &store.ClusterSnapshotHistory{
		ClusterID: clusterInfo.ClusterID,
		Message:   msg,
	}
	if err := ws.store.CreateClusterSnapshotHistory(csh); err != nil {
		return err
	}

	return nil
}
