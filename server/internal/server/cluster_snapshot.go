package server

import (
	"context"
	"fmt"
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

	defaultInterval = time.Hour
	defaultDuration = 7 * 24 * time.Hour
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

	startTime, endTime, err := getStartEndTime(req.Filter, time.Now(), defaultDuration)
	if err != nil {
		return nil, err
	}

	// List all cluster snapshot histories for each snapshot.
	var hs []*store.ClusterSnapshotHistory
	clusterNamesByID := map[string]string{}
	for _, c := range cs {
		chs, err := s.store.ListClusterSnapshotHistories(c.ClusterID, startTime, endTime)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to list cluster snapshot histories for cluster %s: %s", c.ClusterID, err)
		}

		hs = append(hs, chs...)
		clusterNamesByID[c.ClusterID] = c.Name
	}

	allGvals, err := getAllGroupingValues(hs, req.GroupBy, clusterNamesByID)
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
		vs, err := calculateSnapshotValues(hs, req.GroupBy, clusterNamesByID, allGvals)
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
		Datapoints:   dps,
		ClusterCount: int32(len(clusterNamesByID)),
	}, nil
}

func calculateSnapshotValues(
	hs []*store.ClusterSnapshotHistory,
	groupBy v1.ListClusterSnapshotsRequest_GroupBy,
	clusterNamesByID map[string]string,
	allGroupingVales []string,
) ([]*v1.ListClusterSnapshotsResponse_Value, error) {
	type stat struct {
		hsCount        int32
		nodeCount      int32
		gpuCapacity    int32
		memoryCapacity int64
		gpuOccupancy   int32
		podCount       int32
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
			v, err := getGroupingValue(h.ClusterID, node, groupBy, clusterNamesByID)
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
			s.gpuOccupancy += node.GpuOccupancy
			s.podCount += node.PodCount
		}
	}

	// Calculate the average by dividing by the number of snapshot histories.
	for _, s := range stats {
		s.nodeCount /= s.hsCount
		s.gpuCapacity /= s.hsCount
		s.memoryCapacity /= int64(s.hsCount)
		s.gpuOccupancy /= s.hsCount
		s.podCount /= s.hsCount
	}

	if groupBy == v1.ListClusterSnapshotsRequest_GROUP_BY_UNSPECIFIED {
		// No grouping. Aggregate all stats into a single value.
		var val v1.ListClusterSnapshotsResponse_Value
		for _, s := range stats {
			val.NodeCount += s.nodeCount
			val.GpuCapacity += s.gpuCapacity
			val.MemoryCapacityGb += int32(s.memoryCapacity / toGB)
			val.GpuOccupancy += s.gpuOccupancy
			val.PodCount += s.podCount
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
			GpuOccupancy:     s.gpuOccupancy,
			PodCount:         s.podCount,
		})
	}

	return vals, nil
}

func getAllGroupingValues(
	hs []*store.ClusterSnapshotHistory,
	groupBy v1.ListClusterSnapshotsRequest_GroupBy,
	clusterNamesByID map[string]string,
) ([]string, error) {
	gvalMap := map[string]bool{}
	for _, h := range hs {
		var snapshot v1.ClusterSnapshot
		if err := proto.Unmarshal(h.Message, &snapshot); err != nil {
			return nil, err
		}

		for _, node := range snapshot.Nodes {
			v, err := getGroupingValue(h.ClusterID, node, groupBy, clusterNamesByID)
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

func getGroupingValue(
	clusterID string,
	node *v1.ClusterSnapshot_Node,
	groupBy v1.ListClusterSnapshotsRequest_GroupBy,
	clusterNamesByID map[string]string,
) (string, error) {
	switch groupBy {
	// Still use the cluster name for UNSPECIFIED (to average histories per cluster). It will be summed up later.
	case v1.ListClusterSnapshotsRequest_GROUP_BY_UNSPECIFIED,
		v1.ListClusterSnapshotsRequest_GROUP_BY_CLUSTER:
		cname, ok := clusterNamesByID[clusterID]
		if !ok {
			return "", fmt.Errorf("no cluster name found for %q", clusterID)
		}
		return cname, nil
	case v1.ListClusterSnapshotsRequest_GROUP_BY_PRODUCT:
		if node.NvidiaAttributes == nil {
			return "unknown", nil
		}
		return node.NvidiaAttributes.Product, nil
	default:
		return "", status.Errorf(codes.InvalidArgument, "invalid value to groupBy: %v", groupBy)
	}

}

func getStartEndTime(filter *v1.RequestFilter, now time.Time, duration time.Duration) (time.Time, time.Time, error) {
	if filter == nil {
		filter = &v1.RequestFilter{}
	}

	var (
		startTime time.Time
		endTime   time.Time
	)

	switch t := filter.EndTimestamp; {
	case t > 0:
		endTime = time.Unix(t, 0)
	case t == 0:
		// Set the endtime so that it includes the most recent hour after truncation.
		//
		// But we also don't want to advance if there is no datapoint reported from the agent in
		// the most recent hour. So we add half of the default interval to the current time.
		endTime = now.Add(defaultInterval / 2)
	default:
		return time.Time{}, time.Time{}, status.Errorf(codes.InvalidArgument, "endTimestamp must be a non-negative value")
	}
	endTime = endTime.Truncate(defaultInterval)

	switch t := filter.StartTimestamp; {
	case t > 0:
		startTime = time.Unix(t, 0)
	case t == 0:
		startTime = endTime.Add(-1 * duration)
	default:
		return time.Time{}, time.Time{}, status.Errorf(codes.InvalidArgument, "startTimestamp must be a non-negative value")
	}
	startTime = startTime.Truncate(defaultInterval)

	if !startTime.Before(endTime) {
		return time.Time{}, time.Time{}, status.Errorf(codes.InvalidArgument, "startTimestamp must be before endTimestamp")
	}

	return startTime, endTime, nil
}
