package server

import (
	"context"
	"fmt"
	"time"

	v1 "github.com/llmariner/cluster-monitor/api/v1"
	"github.com/llmariner/rbac-manager/pkg/auth"
)

// ListClusterSnapshots lists the cluster snapshots.
func (s *S) ListClusterSnapshots(
	ctx context.Context,
	req *v1.ListClusterSnapshotsRequest,
) (*v1.ListClusterSnapshotsResponse, error) {
	_, ok := auth.ExtractUserInfoFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("failed to extract user info from context")
	}

	// Fake value for testing.
	return &v1.ListClusterSnapshotsResponse{
		Datapoints: []*v1.ListClusterSnapshotsResponse_Datapoint{
			{
				Timestamp: time.Now().Unix(),
				Values: []*v1.ListClusterSnapshotsResponse_Value{
					{
						GroupingKey: nil,
						GpuCapacity: 1,
					},
				},
			},
		},
	}, nil
}
