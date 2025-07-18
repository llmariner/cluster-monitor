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

// ListGpuUsages lists GPU usages.
func (s *S) ListGpuUsages(
	ctx context.Context,
	req *v1.ListGpuUsagesRequest,
) (*v1.ListGpuUsagesResponse, error) {
	authInfo, ok := auth.ExtractUserInfoFromContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.Unauthenticated, "failed to extract user info from context")
	}

	// Query all clusters of the tenant.
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

	var hs []*store.GPUTelemetryHistory
	clusterNamesByID := map[string]string{}
	for _, c := range cs {
		chs, err := s.store.ListGPUTelemetryHistories(c.ClusterID, startTime, endTime)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to list gpu telemetry histories for cluster %s: %s", c.ClusterID, err)
		}

		hs = append(hs, chs...)
		clusterNamesByID[c.ClusterID] = c.Name
	}

	allClusterNodes, err := getAllClusterNodes(hs, clusterNamesByID)
	if err != nil {
		return nil, err
	}

	// Group histories by intervals in a single pass
	intervalBuckets := make(map[int64][]*store.GPUTelemetryHistory)
	for _, h := range hs {
		t := h.HistoryCreatedAt.Truncate(defaultInterval)
		intervalBuckets[t.Unix()] = append(intervalBuckets[t.Unix()], h)
	}

	// Process each interval bucket
	var dps []*v1.ListGpuUsagesResponse_Datapoint
	for t := startTime; t.Before(endTime); t = t.Add(defaultInterval) {
		hs := intervalBuckets[t.Unix()]
		vs, err := calculateGPUUsageValues(hs, clusterNamesByID, allClusterNodes)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to calculate GPU usage values: %s", err)
		}
		dp := &v1.ListGpuUsagesResponse_Datapoint{
			Timestamp: t.Unix(),
			Values:    vs,
		}
		dps = append(dps, dp)
	}

	return &v1.ListGpuUsagesResponse{
		Datapoints: dps,
	}, nil
}

func calculateGPUUsageValues(
	hs []*store.GPUTelemetryHistory,
	clusterNamesByID map[string]string,
	allClusterNodes []clusterNode,
) ([]*v1.ListGpuUsagesResponse_Value, error) {
	type stat struct {
		hsCount int32

		maxGPUUsed float32
		avgGPUUsed float32

		maxGPUMMemoryUsed int64
		avgGPUMemoryUsed  int64
	}

	stats := make(map[clusterNode]*stat)

	// Group snapshots by a cluster and a node.
	for _, h := range hs {
		var snapshot v1.GpuTelemetry
		if err := proto.Unmarshal(h.Message, &snapshot); err != nil {
			return nil, err
		}

		found := make(map[clusterNode]bool)

		for _, node := range snapshot.Nodes {
			cname, ok := clusterNamesByID[h.ClusterID]
			if !ok {
				return nil, fmt.Errorf("no cluster name found for %q", h.ClusterID)
			}
			cn := clusterNode{
				clusterName: cname,
				nodeName:    node.Name,
			}

			s, ok := stats[cn]
			if !ok {
				s = &stat{}
				stats[cn] = s
			}

			if !found[cn] {
				s.hsCount++
				found[cn] = true
			}

			s.maxGPUUsed = max(s.maxGPUUsed, node.MaxGpuUsed)
			s.avgGPUUsed += node.AvgGpuUsed
			s.maxGPUMMemoryUsed = max(s.maxGPUMMemoryUsed, node.MaxGpuMemoryUsed)
			s.avgGPUMemoryUsed += node.AvgGpuMemoryUsed
		}
	}

	// Calculate the average by dividing by the number of snapshot histories.
	for _, s := range stats {
		s.avgGPUUsed /= float32(s.hsCount)
		s.avgGPUMemoryUsed /= int64(s.hsCount)
	}

	var vals []*v1.ListGpuUsagesResponse_Value
	for _, cn := range allClusterNodes {
		s, ok := stats[cn]
		if !ok {
			// Add missing value.
			vals = append(vals, &v1.ListGpuUsagesResponse_Value{
				ClusterName: cn.clusterName,
				NodeName:    cn.nodeName,
			})
			continue
		}

		vals = append(vals, &v1.ListGpuUsagesResponse_Value{
			ClusterName: cn.clusterName,
			NodeName:    cn.nodeName,

			MaxGpuUsed: s.maxGPUUsed,
			AvgGpuUsed: s.avgGPUUsed,

			MaxGpuMemoryUsedGb: float32(s.maxGPUMMemoryUsed) / float32(toGB),
			AvgGpuMemoryUsedGb: float32(s.avgGPUMemoryUsed) / float32(toGB),
		})
	}

	return vals, nil
}

type clusterNode struct {
	clusterName string
	nodeName    string
}

func getAllClusterNodes(
	hs []*store.GPUTelemetryHistory,
	clusterNamesByID map[string]string,
) ([]clusterNode, error) {
	cnMap := map[clusterNode]bool{}
	for _, h := range hs {
		var tel v1.GpuTelemetry
		if err := proto.Unmarshal(h.Message, &tel); err != nil {
			return nil, err
		}

		cname, ok := clusterNamesByID[h.ClusterID]
		if !ok {
			return nil, fmt.Errorf("no cluster name found for %q", h.ClusterID)
		}

		for _, node := range tel.Nodes {
			cn := clusterNode{
				clusterName: cname,
				nodeName:    node.Name,
			}
			cnMap[cn] = true
		}
	}

	var cns []clusterNode
	for cn := range cnMap {
		cns = append(cns, cn)
	}
	sort.Slice(cns, func(i, j int) bool {
		if cns[i].clusterName != cns[j].clusterName {
			return cns[i].clusterName < cns[j].clusterName
		}
		return cns[i].nodeName < cns[j].nodeName
	})

	return cns, nil
}
