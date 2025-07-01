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

const (
	toGB int64 = 1024 * 1024 * 1024
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

	allGvals, err := getAllGroupingValues(hs, req.GroupBy)
	if err != nil {
		return nil, err
	}

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
		vs, err := calculateSnapshotValues(hs, allGvals, req.GroupBy)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to calculate snapshot values: %s", err)
		}
		dp := &v1.ListClusterSnapshotsResponse_Datapoint{
			Timestamp: t.Unix(),
			Values:    vs,
		}
		dps = append(dps, dp)
	}

	return &v1.ListClusterSnapshotsResponse{
		Datapoints: dps,
	}, nil
}

func calculateSnapshotValues(
	hs []*store.ClusterSnapshotHistory,
	allGroupingVales []string,
	groupBy v1.ListClusterSnapshotsRequest_GroupBy,
) ([]*v1.ListClusterSnapshotsResponse_Value, error) {
	type stat struct {
		hsCount        int32
		nodeCount      int32
		gpuCapacity    int32
		memoryCapacity int64
	}
	stats := make(map[string]*stat)

	// Group snapshots by a grouping key (or cluster if there is no grouping specified).
	for _, h := range hs {
		var snapshot v1.ClusterSnapshot
		if err := proto.Unmarshal(h.Message, &snapshot); err != nil {
			return nil, err
		}

		found := make(map[string]bool)

		for _, node := range snapshot.Nodes {
			v, err := getGroupingValue(h.ClusterID, node, groupBy)
			if err != nil {
				return nil, err
			}

			s, ok := stats[v]
			if !ok {
				s = &stat{}
				stats[v] = s
			}

			if !found[v] {
				s.hsCount++
				found[v] = true
			}

			s.nodeCount++
			s.gpuCapacity += node.GpuCapacity
			s.memoryCapacity += node.MemoryCapacity
		}
	}

	// Calculate the average by dividing by the number of snapshot histories.
	for _, s := range stats {
		s.nodeCount /= s.hsCount
		s.gpuCapacity /= s.hsCount
		s.memoryCapacity /= int64(s.hsCount)
	}

	if groupBy == v1.ListClusterSnapshotsRequest_GROUP_BY_UNSPECIFIED {
		// No grouping. Aggregate all stats into a single value.
		var val v1.ListClusterSnapshotsResponse_Value
		for _, s := range stats {
			val.NodeCount += s.nodeCount
			val.GpuCapacity += s.gpuCapacity
			val.MemoryCapacityGb += int32(s.memoryCapacity / toGB)
		}
		return []*v1.ListClusterSnapshotsResponse_Value{&val}, nil
	}

	var vals []*v1.ListClusterSnapshotsResponse_Value
	for _, gval := range allGroupingVales {
		s, ok := stats[gval]
		if !ok {
			// Add missing value.
			vals = append(vals, &v1.ListClusterSnapshotsResponse_Value{
				GroupingValue: gval,
			})
			continue
		}

		vals = append(vals, &v1.ListClusterSnapshotsResponse_Value{
			GroupingValue:    gval,
			NodeCount:        s.nodeCount,
			GpuCapacity:      s.gpuCapacity,
			MemoryCapacityGb: int32(s.memoryCapacity / toGB),
		})
	}

	return vals, nil
}

func getAllGroupingValues(
	hs []*store.ClusterSnapshotHistory,
	groupBy v1.ListClusterSnapshotsRequest_GroupBy,
) ([]string, error) {
	gvalMap := map[string]bool{}
	for _, h := range hs {
		var snapshot v1.ClusterSnapshot
		if err := proto.Unmarshal(h.Message, &snapshot); err != nil {
			return nil, err
		}

		for _, node := range snapshot.Nodes {
			v, err := getGroupingValue(h.ClusterID, node, groupBy)
			if err != nil {
				return nil, err
			}
			gvalMap[v] = true
		}
	}

	var gvals []string
	for v := range gvalMap {
		gvals = append(gvals, v)
	}
	sort.Strings(gvals)

	return gvals, nil
}

func getGroupingValue(clusterID string, node *v1.ClusterSnapshot_Node, groupBy v1.ListClusterSnapshotsRequest_GroupBy) (string, error) {
	switch groupBy {
	case v1.ListClusterSnapshotsRequest_GROUP_BY_UNSPECIFIED:
		// Still use the cluster name (to average histories per cluster). It will be summed up later.
		return clusterID, nil
	case v1.ListClusterSnapshotsRequest_GROUP_BY_CLUSTER:
		return clusterID, nil
	case v1.ListClusterSnapshotsRequest_GROUP_BY_PRODUCT:
		return node.NvidiaAttributes.Product, nil
	default:
		return "", status.Errorf(codes.InvalidArgument, "invalid value to groupBy: %v", groupBy)
	}

}
