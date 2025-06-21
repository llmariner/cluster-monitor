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
	const (
		defaultDuration = 24 * time.Hour
		defaultInterval = time.Hour
	)

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

	endTime := time.Now().Truncate(defaultInterval)
	startTime := endTime.Add(-1 * defaultDuration)

	// List all cluster snapshot histories for each snapshot.
	var hs []*store.ClusterSnapshotHistory
	for _, c := range cs {
		chs, err := s.store.ListClusterSnapshotHistories(c.ClusterID, startTime.Add(-1*defaultInterval), endTime)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to list cluster snapshot histories for cluster %s: %s", c.ClusterID, err)
		}
		hs = append(hs, chs...)
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

	sort.Slice(hs, func(i, j int) bool {
		return hs[i].HistoryCreatedAt.Before(hs[j].HistoryCreatedAt)
	})

	// Group histories by hourly intervals in a single pass
	intervalBuckets := make(map[int64][]*store.ClusterSnapshotHistory)
	for _, h := range hs {
		t := h.HistoryCreatedAt.Truncate(defaultInterval)
		intervalBuckets[t.Unix()] = append(intervalBuckets[t.Unix()], h)
	}

	// Process each interval bucket
	var dps []*v1.ListClusterSnapshotsResponse_Datapoint
	for t := startTime; t.Before(endTime); t = t.Add(defaultInterval) {
		hs := intervalBuckets[t.Unix()]
		v, err := calculateTotalGPUCapacity(hs)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to calculate total gpu capacity: %s", err)
		}
		dp := &v1.ListClusterSnapshotsResponse_Datapoint{
			Timestamp: t.Unix(),
			Values: []*v1.ListClusterSnapshotsResponse_Value{
				{
					GroupingKey: nil,
					GpuCapacity: v,
				},
			},
		}
		dps = append(dps, dp)
	}

	return &v1.ListClusterSnapshotsResponse{
		Datapoints: dps,
	}, nil
}

func calculateTotalGPUCapacity(hs []*store.ClusterSnapshotHistory) (int32, error) {
	var totalGPUCapacity int32

	// Group by cluster ID and calculate average GPU capacity per cluster
	clusterGPUSums := make(map[string]int32)
	clusterCounts := make(map[string]int)

	for _, h := range hs {
		var snapshot v1.ClusterSnapshot
		if err := proto.Unmarshal(h.Message, &snapshot); err != nil {
			return 0, err
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

	return totalGPUCapacity, nil
}
