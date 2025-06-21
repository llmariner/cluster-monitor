package server

import (
	"context"
	"sort"
	"time"

	v1 "github.com/llmariner/cluster-monitor/api/v1"
	"github.com/llmariner/cluster-monitor/server/internal/store"
	"github.com/llmariner/rbac-manager/pkg/auth"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
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
	var allHistories []*store.ClusterSnapshotHistory
	for _, c := range cs {
		histories, err := s.store.ListClusterSnapshotHistories(c.ClusterID)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to list cluster snapshot histories for cluster %s: %s", c.ClusterID, err)
		}
		allHistories = append(allHistories, histories...)
	}

	// Construct datapoints.
	//
	// 1. Sort cluster snapshot histories by its CreatedAt in ascending order.
	// 2. Group them by an interval of 1 hour.
	// 3. For each interval, construct a datapoint (v1.ListClusterSnapshotsResponse_Datapoint).
	//    The timestamp of the datapoint is the start of the interval.
	//    The values of the datapoint is currently sum of GPU capacities of all cluster snapshots in the interval.
	//    The grouping key is nil as we don't support grouping by any key yet.
	//    Take average if there is more than one snapshot from the same cluster in the interval.

	endTime := time.Now()
	startTime := endTime.Add(-24 * time.Hour) // Default to the last 24 hours.

	// Filter histories within the time range and sort by CreatedAt
	var filteredHistories []*store.ClusterSnapshotHistory
	for _, h := range allHistories {
		if !h.CreatedAt.Before(startTime) && h.CreatedAt.Before(endTime) {
			filteredHistories = append(filteredHistories, h)
		}
	}

	sort.Slice(filteredHistories, func(i, j int) bool {
		return filteredHistories[i].CreatedAt.Before(filteredHistories[j].CreatedAt)
	})

	// Group by 1-hour intervals
	datapoints := make([]*v1.ListClusterSnapshotsResponse_Datapoint, 0)
	hourInterval := time.Hour

	for current := startTime.Truncate(hourInterval); current.Before(endTime); current = current.Add(hourInterval) {
		intervalEnd := current.Add(hourInterval)

		// Collect all histories in this interval
		// TODO(kenji): Make this more efficient.
		var ihs []*store.ClusterSnapshotHistory
		for _, h := range filteredHistories {
			if !h.CreatedAt.Before(current) && h.CreatedAt.Before(intervalEnd) {
				ihs = append(ihs, h)
			}
		}

		var totalGPUCapacity int32
		if len(ihs) > 0 {
			// Group by cluster ID and calculate average GPU capacity per cluster
			clusterGPUSums := make(map[string]int32)
			clusterCounts := make(map[string]int)

			for _, h := range ihs {
				var snapshot workerv1.ClusterSnapshot
				if err := proto.Unmarshal(h.Message, &snapshot); err != nil {
					continue // Skip invalid messages
				}

				var totalGPU int32
				for _, node := range snapshot.Nodes {
					totalGPU += node.GpuCapacity
				}

				clusterGPUSums[h.ClusterID] += totalGPU
				clusterCounts[h.ClusterID]++
			}

			// Calculate total GPU capacity (sum of averages from each cluster)
			for clusterID, sum := range clusterGPUSums {
				count := clusterCounts[clusterID]
				totalGPUCapacity += sum / int32(count) // Average for this cluster
			}
		}

		datapoint := &v1.ListClusterSnapshotsResponse_Datapoint{
			Timestamp: current.Unix(),
			Values: []*v1.ListClusterSnapshotsResponse_Value{
				{
					GroupingKey: nil,
					GpuCapacity: totalGPUCapacity,
				},
			},
		}
		datapoints = append(datapoints, datapoint)
	}

	return &v1.ListClusterSnapshotsResponse{
		Datapoints: datapoints,
	}, nil
}
