package sender

import (
	"context"
	"sync"
	"testing"

	v1 "github.com/llmariner/cluster-monitor/api/v1"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
)

func TestSender(t *testing.T) {
	client := &fakeTelemetryClient{}
	ch := make(chan *v1.SendClusterTelemetryRequest_Payload)
	sender := New(client, ch)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = sender.Start(ctx)
	}()

	ch <- &v1.SendClusterTelemetryRequest_Payload{}

	client.mu.Lock()
	assert.Len(t, client.reqs, 1)
	client.mu.Unlock()
}

type fakeTelemetryClient struct {
	reqs []*v1.SendClusterTelemetryRequest
	mu   sync.Mutex
}

func (c *fakeTelemetryClient) SendClusterTelemetry(
	ctx context.Context,
	req *v1.SendClusterTelemetryRequest,
	opts ...grpc.CallOption,
) (*v1.SendClusterTelemetryResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.reqs = append(c.reqs, req)
	return &v1.SendClusterTelemetryResponse{}, nil
}
