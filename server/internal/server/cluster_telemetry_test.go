package server

import (
	"context"
	"testing"

	"github.com/go-logr/logr/testr"
	v1 "github.com/llmariner/cluster-monitor/api/v1"
	"github.com/llmariner/cluster-monitor/server/internal/store"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
)

func TestListClusters(t *testing.T) {
	st, tearDown := store.NewTest(t)
	defer tearDown()

	srv := NewWorkerServiceServer(st, testr.New(t))

	ctx := fakeAuthInto(context.Background())
	req := &v1.SendClusterTelemetryRequest{
		Payloads: []*v1.SendClusterTelemetryRequest_Payload{
			{
				MessageKind: &v1.SendClusterTelemetryRequest_Payload_ClusterSnapshot{
					ClusterSnapshot: &v1.ClusterSnapshot{
						Nodes: []*v1.ClusterSnapshot_Node{
							{
								Name: "node1",
							},
							{
								Name: "node2",
							},
						},
					},
				},
			},
		},
	}
	_, err := srv.SendClusterTelemetry(ctx, req)
	assert.NoError(t, err)

	cs, err := st.GetClusterSnapshotByID(defaultClusterID)
	assert.NoError(t, err)

	csProto := &v1.ClusterSnapshot{}
	err = proto.Unmarshal(cs.Message, csProto)
	assert.NoError(t, err)
	assert.Len(t, csProto.Nodes, 2)
}
