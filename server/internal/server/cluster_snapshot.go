package server

import (
	"context"
	"time"

	v1 "github.com/llmariner/cluster-monitor/api/v1"
	"github.com/llmariner/rbac-manager/pkg/auth"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ListClusterSnapshots lists the cluster snapshots.
func (s *S) ListClusterSnapshots(
	ctx context.Context,
	req *v1.ListClusterSnapshotsRequest,
) (*v1.ListClusterSnapshotsResponse, error) {
	authInfo, ok := auth.ExtractUserInfoFromContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.Unauthenticated, "failed to extract user info from context")
	}

	// Query all snapshots of the tenant.
	cs, err := s.store.ListClusterSnapshotsByTenantID(authInfo.TenantID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list cluster snapshots: %s", err)
	}

	if len(cs) == 0 {
		return nil, status.Errorf(codes.NotFound, "no cluster snapshots found for tenant %s", authInfo.TenantID)
	}

	// List all cluster snapshot histories for each snapshot.

	// Construct datapoints.
	//
	// 1. Sort cluster snapshot histories by its CreatedAt in ascending order.
	// 2. Group them by an interval of 1 hour.
	// 3. For each internal, consturct a datapoint (v1.ListClusterSnapshotsResponse_Datapoint).
	//    The timestamp of the datapoint is the start of the interval.
	//    The values of the datapoint is currently sum of GPU capacities of all cluster snapshots in the interval.
	//    The grouping key is nil as we don't support grouping by any key yet.
	//    Take average if there is more than one snapshot from the same cluster in the interval.

	endTime := time.Now()
	startTime := endTime.Add(-24 * time.Hour) // Default to the last 24 hours.

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
