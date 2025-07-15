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

func TestListGpuUsagesTest(t *testing.T) {
	now := time.Date(2025, 7, 17, 11, 40, 0, 0, time.Local)
	nowT := now.Truncate(time.Hour)

	generateDatapoints := func(vals []float32) []*v1.ListGpuUsagesResponse_Datapoint {
		var dps []*v1.ListGpuUsagesResponse_Datapoint
		for i := 0; i < 24; i++ {
			timestamp := nowT.Add(time.Duration(i-24) * time.Hour).Unix()
			dps = append(dps, &v1.ListGpuUsagesResponse_Datapoint{
				Timestamp: timestamp,
				Values: []*v1.ListGpuUsagesResponse_Value{
					{
						ClusterName: "cluster0",
						NodeName:    "node0",
						AvgGpuUsed:  vals[i],
					},
				},
			})
		}
		return dps
	}

	tcs := []struct {
		name  string
		setup func(*testing.T, *store.S)
		req   *v1.ListGpuUsagesRequest
		want  *v1.ListGpuUsagesResponse
	}{
		{
			setup: func(t *testing.T, st *store.S) {
				cs := []*store.ClusterSnapshot{
					{
						ClusterID: "cid0",
						Name:      "cluster0",
						TenantID:  defaultTenantID,
					},
				}

				for _, c := range cs {
					err := st.CreateOrUpdateClusterSnapshot(c)
					assert.NoError(t, err)
				}

				ghs := []*store.GPUTelemetryHistory{
					{
						ClusterID:        "cid0",
						HistoryCreatedAt: now.Add(-3 * time.Hour),
						Message: marshalTelemetryProto(t, &v1.GpuTelemetry{
							Nodes: []*v1.GpuTelemetry_Node{
								{
									Name:       "node0",
									AvgGpuUsed: 1.0,
								},
							},
						}),
					},
					{
						ClusterID:        "cid0",
						HistoryCreatedAt: now.Add(-2 * time.Hour),
						Message: marshalTelemetryProto(t, &v1.GpuTelemetry{
							Nodes: []*v1.GpuTelemetry_Node{
								{
									Name:       "node0",
									AvgGpuUsed: 2.0,
								},
							},
						}),
					},
					{
						ClusterID:        "cid0",
						HistoryCreatedAt: now.Add(-2*time.Hour + 1*time.Minute),
						Message: marshalTelemetryProto(t, &v1.GpuTelemetry{
							Nodes: []*v1.GpuTelemetry_Node{
								{
									Name:       "node0",
									AvgGpuUsed: 3.0,
								},
							},
						}),
					},
				}
				for _, gh := range ghs {
					err := st.CreateGPUTelemetryHistory(gh)
					assert.NoError(t, err)
				}
			},
			req: &v1.ListGpuUsagesRequest{
				Filter: &v1.RequestFilter{
					EndTimestamp: now.Unix(),
				},
			},
			want: &v1.ListGpuUsagesResponse{
				Datapoints: generateDatapoints([]float32{
					0, 0, 0, 0, 0, 0,
					0, 0, 0, 0, 0, 0,
					0, 0, 0, 0, 0, 0,
					0, 0, 0, 1, 2.5, 0,
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

			got, err := srv.ListGpuUsages(ctx, tc.req)
			assert.NoError(t, err)

			assert.Len(t, got.Datapoints, len(tc.want.Datapoints))
			for i, g := range got.Datapoints {
				w := tc.want.Datapoints[i]
				assert.Equal(t, g.Timestamp, w.Timestamp)
				assert.Len(t, g.Values, len(w.Values))
				for j, gv := range g.Values {
					wv := w.Values[j]
					assert.Equal(t, gv.ClusterName, wv.ClusterName)
					assert.Equal(t, gv.NodeName, wv.NodeName)
					assert.InDelta(t, gv.AvgGpuUsed, wv.AvgGpuUsed, 0.0001)
				}
			}
		})
	}
}

func TestCalculateGPUUsageValues(t *testing.T) {
	tcs := []struct {
		name             string
		hs               []*store.GPUTelemetryHistory
		clusterNamesByID map[string]string
		allClusterNodes  []clusterNode
		want             []*v1.ListGpuUsagesResponse_Value
	}{
		{
			name: "empty history",
		},
		{
			name: "single cluster, single node",
			hs: []*store.GPUTelemetryHistory{
				{
					ClusterID: "cid0",
					Message: marshalTelemetryProto(t, &v1.GpuTelemetry{
						Nodes: []*v1.GpuTelemetry_Node{
							{
								Name:             "node0",
								MaxGpuUsed:       1,
								AvgGpuUsed:       0.5,
								MaxGpuMemoryUsed: 1000 * toGB,
								AvgGpuMemoryUsed: 500 * toGB,
							},
						},
					}),
				},
			},
			clusterNamesByID: map[string]string{
				"cid0": "cluster0",
			},
			allClusterNodes: []clusterNode{
				{clusterName: "cluster0", nodeName: "node0"},
			},
			want: []*v1.ListGpuUsagesResponse_Value{
				{
					ClusterName:        "cluster0",
					NodeName:           "node0",
					MaxGpuUsed:         1,
					AvgGpuUsed:         0.5,
					MaxGpuMemoryUsedGb: 1000,
					AvgGpuMemoryUsedGb: 500,
				},
			},
		},
		{
			name: "single cluster, multiple nodes",
			hs: []*store.GPUTelemetryHistory{
				{
					ClusterID: "cid0",
					Message: marshalTelemetryProto(t, &v1.GpuTelemetry{
						Nodes: []*v1.GpuTelemetry_Node{
							{
								Name:             "node0",
								MaxGpuUsed:       1,
								AvgGpuUsed:       0.5,
								MaxGpuMemoryUsed: 1000 * toGB,
								AvgGpuMemoryUsed: 500 * toGB,
							},
							{
								Name:             "node1",
								MaxGpuUsed:       2,
								AvgGpuUsed:       1.0,
								MaxGpuMemoryUsed: 2000 * toGB,
								AvgGpuMemoryUsed: 1000 * toGB,
							},
						},
					}),
				},
			},
			clusterNamesByID: map[string]string{
				"cid0": "cluster0",
			},
			allClusterNodes: []clusterNode{
				{clusterName: "cluster0", nodeName: "node0"},
				{clusterName: "cluster0", nodeName: "node1"},
			},
			want: []*v1.ListGpuUsagesResponse_Value{
				{
					ClusterName:        "cluster0",
					NodeName:           "node0",
					MaxGpuUsed:         1,
					AvgGpuUsed:         0.5,
					MaxGpuMemoryUsedGb: 1000,
					AvgGpuMemoryUsedGb: 500,
				},
				{
					ClusterName:        "cluster0",
					NodeName:           "node1",
					MaxGpuUsed:         2,
					AvgGpuUsed:         1.0,
					MaxGpuMemoryUsedGb: 2000,
					AvgGpuMemoryUsedGb: 1000,
				},
			},
		},
		{
			name: "multiple clusters, multiple nodes",
			hs: []*store.GPUTelemetryHistory{
				{
					ClusterID: "cid0",
					Message: marshalTelemetryProto(t, &v1.GpuTelemetry{
						Nodes: []*v1.GpuTelemetry_Node{
							{
								Name:             "node0",
								MaxGpuUsed:       1,
								AvgGpuUsed:       0.5,
								MaxGpuMemoryUsed: 1000 * toGB,
								AvgGpuMemoryUsed: 500 * toGB,
							},
							{
								Name:             "node1",
								MaxGpuUsed:       2,
								AvgGpuUsed:       1.0,
								MaxGpuMemoryUsed: 2000 * toGB,
								AvgGpuMemoryUsed: 1000 * toGB,
							},
						},
					}),
				},
				{
					ClusterID: "cid1",
					Message: marshalTelemetryProto(t, &v1.GpuTelemetry{
						Nodes: []*v1.GpuTelemetry_Node{
							{
								Name:             "node2",
								MaxGpuUsed:       4,
								AvgGpuUsed:       2.0,
								MaxGpuMemoryUsed: 4000 * toGB,
								AvgGpuMemoryUsed: 2000 * toGB,
							},
						},
					}),
				},
			},
			clusterNamesByID: map[string]string{
				"cid0": "cluster0",
				"cid1": "cluster1",
			},
			allClusterNodes: []clusterNode{
				{clusterName: "cluster0", nodeName: "node0"},
				{clusterName: "cluster0", nodeName: "node1"},
				{clusterName: "cluster1", nodeName: "node2"},
			},
			want: []*v1.ListGpuUsagesResponse_Value{
				{
					ClusterName:        "cluster0",
					NodeName:           "node0",
					MaxGpuUsed:         1,
					AvgGpuUsed:         0.5,
					MaxGpuMemoryUsedGb: 1000,
					AvgGpuMemoryUsedGb: 500,
				},
				{
					ClusterName:        "cluster0",
					NodeName:           "node1",
					MaxGpuUsed:         2,
					AvgGpuUsed:         1.0,
					MaxGpuMemoryUsedGb: 2000,
					AvgGpuMemoryUsedGb: 1000,
				},
				{
					ClusterName:        "cluster1",
					NodeName:           "node2",
					MaxGpuUsed:         4,
					AvgGpuUsed:         2.0,
					MaxGpuMemoryUsedGb: 4000,
					AvgGpuMemoryUsedGb: 2000,
				},
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			got, err := calculateGPUUsageValues(tc.hs, tc.clusterNamesByID, tc.allClusterNodes)
			assert.NoError(t, err)
			assert.Len(t, got, len(tc.want))
			for i, want := range tc.want {
				assert.Truef(t, proto.Equal(got[i], want), cmp.Diff(got[i], want, protocmp.Transform()))
			}
		})
	}
}

func TestGetAllClusterNodes(t *testing.T) {
	hs := []*store.GPUTelemetryHistory{
		{
			ClusterID: "cid0",
			Message: marshalTelemetryProto(t, &v1.GpuTelemetry{
				Nodes: []*v1.GpuTelemetry_Node{
					{
						Name: "node0",
					},
					{
						Name: "node1",
					},
				},
			}),
		},
		{
			ClusterID: "cid1",
			Message: marshalTelemetryProto(t, &v1.GpuTelemetry{
				Nodes: []*v1.GpuTelemetry_Node{
					{
						Name: "node2",
					},
				},
			}),
		},
	}

	clusterNamesByID := map[string]string{
		"cid0": "cluster0",
		"cid1": "cluster1",
	}

	got, err := getAllClusterNodes(hs, clusterNamesByID)
	assert.NoError(t, err)
	want := []clusterNode{
		{clusterName: "cluster0", nodeName: "node0"},
		{clusterName: "cluster0", nodeName: "node1"},
		{clusterName: "cluster1", nodeName: "node2"},
	}
	assert.ElementsMatch(t, want, got)
}

func marshalTelemetryProto(t *testing.T, msg *v1.GpuTelemetry) []byte {
	data, err := proto.Marshal(msg)
	assert.NoError(t, err)
	return data
}
