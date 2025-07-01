package server

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr/testr"
	"github.com/google/go-cmp/cmp"
	v1 "github.com/llmariner/cluster-monitor/api/v1"
	"github.com/llmariner/cluster-monitor/server/internal/store"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestListClusterSnapshots(t *testing.T) {
	now := time.Now()
	nowT := now.Truncate(time.Hour)

	generateDatapoints := func(vals []int32) []*v1.ListClusterSnapshotsResponse_Datapoint {
		dps := make([]*v1.ListClusterSnapshotsResponse_Datapoint, 0, len(vals))
		for i := 0; i < 24; i++ {
			timestamp := nowT.Add(-time.Duration(24-i) * time.Hour).Unix()

			gpuCapacity := vals[i]
			nodeCount := int32(0)
			if vals[i] > 0 {
				nodeCount = 2
			}

			dps = append(dps, &v1.ListClusterSnapshotsResponse_Datapoint{
				Timestamp: timestamp,
				Values: []*v1.ListClusterSnapshotsResponse_Value{
					{
						GpuCapacity: gpuCapacity,
						NodeCount:   nodeCount,
					},
				},
			})
		}
		return dps
	}

	tcs := []struct {
		name  string
		setup func(*testing.T, *store.S)
		req   *v1.ListClusterSnapshotsRequest
		want  *v1.ListClusterSnapshotsResponse
	}{
		{
			name: "success",
			setup: func(t *testing.T, st *store.S) {
				cs := []*store.ClusterSnapshot{
					{
						ClusterID: "cid0",
						TenantID:  defaultTenantID,
					},
					{
						ClusterID: "cid1",
						TenantID:  defaultTenantID,
					},
				}

				for _, c := range cs {
					err := st.CreateOrUpdateClusterSnapshot(c)
					assert.NoError(t, err)
				}

				chs := []*store.ClusterSnapshotHistory{
					{
						ClusterID:        "cid0",
						HistoryCreatedAt: now.Add(-3 * time.Hour),
						Message: marshalSnapshotProto(t, &v1.ClusterSnapshot{
							Nodes: []*v1.ClusterSnapshot_Node{
								{
									GpuCapacity: 1,
								},
								{
									GpuCapacity: 2,
								},
							},
						}),
					},
					{
						ClusterID:        "cid0",
						HistoryCreatedAt: now.Add(-2 * time.Hour),
						Message: marshalSnapshotProto(t, &v1.ClusterSnapshot{
							Nodes: []*v1.ClusterSnapshot_Node{
								{
									GpuCapacity: 1,
								},
								{
									GpuCapacity: 2,
								},
							},
						}),
					},
					{
						ClusterID:        "cid0",
						HistoryCreatedAt: now.Add(-2*time.Hour + 1*time.Minute),
						Message: marshalSnapshotProto(t, &v1.ClusterSnapshot{
							Nodes: []*v1.ClusterSnapshot_Node{
								{
									GpuCapacity: 1,
								},
								{
									GpuCapacity: 2,
								},
							},
						}),
					},
				}
				for _, ch := range chs {
					err := st.CreateClusterSnapshotHistory(ch)
					assert.NoError(t, err)
				}
			},
			req: &v1.ListClusterSnapshotsRequest{},
			want: &v1.ListClusterSnapshotsResponse{
				Datapoints: generateDatapoints([]int32{
					0, 0, 0, 0, 0, 0,
					0, 0, 0, 0, 0, 0,
					0, 0, 0, 0, 0, 0,
					0, 0, 0, 3, 3, 0,
				}),
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			st, tearDown := store.NewTest(t)
			defer tearDown()
			tc.setup(t, st)

			srv := New(st, testr.New(t))
			ctx := fakeAuthInto(context.Background())

			got, err := srv.ListClusterSnapshots(ctx, tc.req)
			assert.NoError(t, err)

			assert.Truef(t, proto.Equal(got, tc.want), cmp.Diff(got, tc.want, protocmp.Transform()))
		})
	}

}

func TestCalculateSnapshotValues(t *testing.T) {
	tcs := []struct {
		name             string
		hs               []*store.ClusterSnapshotHistory
		clusterNamesByID map[string]string
		allGroupingVales []string
		groupBy          v1.ListClusterSnapshotsRequest_GroupBy
		want             []*v1.ListClusterSnapshotsResponse_Value
	}{
		{
			name:             "empty history",
			hs:               []*store.ClusterSnapshotHistory{},
			allGroupingVales: []string{},
			groupBy:          v1.ListClusterSnapshotsRequest_GROUP_BY_UNSPECIFIED,
			want: []*v1.ListClusterSnapshotsResponse_Value{
				{
					GpuCapacity: 0,
				},
			},
		},
		{
			name: "single cluster",
			hs: []*store.ClusterSnapshotHistory{
				{
					ClusterID: "cid0",
					Message: marshalSnapshotProto(t, &v1.ClusterSnapshot{
						Nodes: []*v1.ClusterSnapshot_Node{
							{
								GpuCapacity:    1,
								MemoryCapacity: 1024 * 1024 * 1024,
							},
							{
								GpuCapacity:    2,
								MemoryCapacity: 2 * 1024 * 1024 * 1024,
							},
						},
					}),
				},
			},
			allGroupingVales: []string{"cid0"},
			clusterNamesByID: map[string]string{
				"cid0": "cluster0",
			},
			groupBy: v1.ListClusterSnapshotsRequest_GROUP_BY_UNSPECIFIED,
			want: []*v1.ListClusterSnapshotsResponse_Value{
				{
					NodeCount:        2,
					GpuCapacity:      3,
					MemoryCapacityGb: 3,
				},
			},
		},
		{
			name: "two clusters",
			hs: []*store.ClusterSnapshotHistory{
				{
					ClusterID: "cid0",
					Message: marshalSnapshotProto(t, &v1.ClusterSnapshot{
						Nodes: []*v1.ClusterSnapshot_Node{
							{
								GpuCapacity:    1,
								MemoryCapacity: toGB,
							},
							{
								GpuCapacity:    2,
								MemoryCapacity: 2 * toGB,
							},
						},
					}),
				},
				{
					ClusterID: "cid1",
					Message: marshalSnapshotProto(t, &v1.ClusterSnapshot{
						Nodes: []*v1.ClusterSnapshot_Node{
							{
								GpuCapacity:    10,
								MemoryCapacity: 10 * toGB,
							},
						},
					}),
				},
			},
			clusterNamesByID: map[string]string{
				"cid0": "cluster0",
				"cid1": "cluster1",
			},
			allGroupingVales: []string{"cluster0", "cluster1"},
			groupBy:          v1.ListClusterSnapshotsRequest_GROUP_BY_UNSPECIFIED,
			want: []*v1.ListClusterSnapshotsResponse_Value{
				{
					NodeCount:        3,
					GpuCapacity:      13,
					MemoryCapacityGb: 13,
				},
			},
		},
		{
			name: "group by clusters",
			hs: []*store.ClusterSnapshotHistory{
				{
					ClusterID: "cid0",
					Message: marshalSnapshotProto(t, &v1.ClusterSnapshot{
						Nodes: []*v1.ClusterSnapshot_Node{
							{
								GpuCapacity:    1,
								MemoryCapacity: toGB,
							},
							{
								GpuCapacity:    2,
								MemoryCapacity: 2 * toGB,
							},
						},
					}),
				},
				{
					ClusterID: "cid1",
					Message: marshalSnapshotProto(t, &v1.ClusterSnapshot{
						Nodes: []*v1.ClusterSnapshot_Node{
							{
								GpuCapacity:    10,
								MemoryCapacity: 10 * toGB,
							},
						},
					}),
				},
			},
			clusterNamesByID: map[string]string{
				"cid0": "cluster0",
				"cid1": "cluster1",
			},
			allGroupingVales: []string{"cluster0", "cluster1", "cluster2"},
			groupBy:          v1.ListClusterSnapshotsRequest_GROUP_BY_CLUSTER,
			want: []*v1.ListClusterSnapshotsResponse_Value{
				{
					GroupingValue:    "cluster0",
					NodeCount:        2,
					GpuCapacity:      3,
					MemoryCapacityGb: 3,
				},
				{
					GroupingValue:    "cluster1",
					NodeCount:        1,
					GpuCapacity:      10,
					MemoryCapacityGb: 10,
				},
				{
					GroupingValue:    "cluster2",
					NodeCount:        0,
					GpuCapacity:      0,
					MemoryCapacityGb: 0,
				},
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			got, err := calculateSnapshotValues(tc.hs, tc.groupBy, tc.clusterNamesByID, tc.allGroupingVales)
			assert.NoError(t, err)
			assert.Len(t, got, len(tc.want))
			for i, want := range tc.want {
				assert.Truef(t, proto.Equal(got[i], want), cmp.Diff(got[i], want, protocmp.Transform()))
			}
		})
	}
}

func TestGetAllGroupingValues(t *testing.T) {
	hs := []*store.ClusterSnapshotHistory{
		{
			ClusterID: "cid0",
			Message: marshalSnapshotProto(t, &v1.ClusterSnapshot{
				Nodes: []*v1.ClusterSnapshot_Node{
					{
						NvidiaAttributes: &v1.ClusterSnapshot_Node_NvidiaAttributes{
							Product: "product0",
						},
					},
					{
						NvidiaAttributes: &v1.ClusterSnapshot_Node_NvidiaAttributes{
							Product: "product1",
						},
					},
				},
			}),
		},
		{
			ClusterID: "cid1",
			Message: marshalSnapshotProto(t, &v1.ClusterSnapshot{
				Nodes: []*v1.ClusterSnapshot_Node{
					{
						NvidiaAttributes: &v1.ClusterSnapshot_Node_NvidiaAttributes{
							Product: "product1",
						},
					},
				},
			}),
		},
	}

	clusterNamesByID := map[string]string{
		"cid0": "cluster0",
		"cid1": "cluster1",
	}

	tcs := []struct {
		name    string
		groupBy v1.ListClusterSnapshotsRequest_GroupBy
		want    []string
	}{
		{
			name:    "no grouping",
			groupBy: v1.ListClusterSnapshotsRequest_GROUP_BY_UNSPECIFIED,
			want: []string{
				"cluster0",
				"cluster1",
			},
		},
		{
			name:    "group by cluster",
			groupBy: v1.ListClusterSnapshotsRequest_GROUP_BY_CLUSTER,
			want: []string{
				"cluster0",
				"cluster1",
			},
		},
		{
			name:    "group by product",
			groupBy: v1.ListClusterSnapshotsRequest_GROUP_BY_PRODUCT,
			want: []string{
				"product0",
				"product1",
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			got, err := getAllGroupingValues(hs, tc.groupBy, clusterNamesByID)
			assert.NoError(t, err)

			assert.ElementsMatch(t, tc.want, got)
		})
	}
}

func marshalSnapshotProto(t *testing.T, msg *v1.ClusterSnapshot) []byte {
	data, err := proto.Marshal(msg)
	assert.NoError(t, err)
	return data
}
