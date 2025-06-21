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
	marshalProto := func(t *testing.T, msg *v1.ClusterSnapshot) []byte {
		data, err := proto.Marshal(msg)
		assert.NoError(t, err)
		return data
	}

	now := time.Now()
	nowT := now.Truncate(time.Hour)

	generateDatapoints := func(vals []int32) []*v1.ListClusterSnapshotsResponse_Datapoint {
		dps := make([]*v1.ListClusterSnapshotsResponse_Datapoint, 0, len(vals))
		for i := 0; i < 24; i++ {
			timestamp := nowT.Add(-time.Duration(24-i) * time.Hour).Unix()
			dps = append(dps, &v1.ListClusterSnapshotsResponse_Datapoint{
				Timestamp: timestamp,
				Values:    []*v1.ListClusterSnapshotsResponse_Value{{GpuCapacity: vals[i]}},
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
						Message: marshalProto(t, &v1.ClusterSnapshot{
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
						Message: marshalProto(t, &v1.ClusterSnapshot{
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
						Message: marshalProto(t, &v1.ClusterSnapshot{
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
