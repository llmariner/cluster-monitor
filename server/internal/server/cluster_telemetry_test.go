package server

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr/testr"
	v1 "github.com/llmariner/cluster-monitor/api/v1"
	"github.com/llmariner/cluster-monitor/server/internal/store"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
)

func TestSendClusterTelemetry_ClusterSnapshot(t *testing.T) {
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
	assert.Equal(t, defaultClusterID, cs.ClusterID)

	now := cs.CreatedAt
	hs, err := st.ListClusterSnapshotHistories(cs.ClusterID, now.Add(-1*time.Hour), now.Add(1*time.Hour))
	assert.NoError(t, err)
	assert.Len(t, hs, 1)

	cshProto := &v1.ClusterSnapshot{}
	err = proto.Unmarshal(hs[0].Message, cshProto)
	assert.NoError(t, err)
	assert.Len(t, cshProto.Nodes, 2)
}

func TestSendClusterTelemetry_GPUTelemetry(t *testing.T) {
	st, tearDown := store.NewTest(t)
	defer tearDown()

	srv := NewWorkerServiceServer(st, testr.New(t))

	ctx := fakeAuthInto(context.Background())
	req := &v1.SendClusterTelemetryRequest{
		Payloads: []*v1.SendClusterTelemetryRequest_Payload{
			{
				MessageKind: &v1.SendClusterTelemetryRequest_Payload_GpuTelemetry{
					GpuTelemetry: &v1.GpuTelemetry{
						Nodes: []*v1.GpuTelemetry_Node{
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

	now := time.Now()
	hs, err := st.ListGPUTelemetryHistories(defaultClusterID, now.Add(-1*time.Hour), now.Add(1*time.Hour))
	assert.NoError(t, err)
	assert.Len(t, hs, 1)

	gtProto := &v1.GpuTelemetry{}
	err = proto.Unmarshal(hs[0].Message, gtProto)
	assert.NoError(t, err)
	assert.Len(t, gtProto.Nodes, 2)
}
