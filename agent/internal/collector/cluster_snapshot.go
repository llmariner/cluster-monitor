package collector

import (
	"context"
	"strconv"

	"github.com/go-logr/logr"
	v1 "github.com/llmariner/cluster-monitor/api/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	nvidiaGPU corev1.ResourceName = "nvidia.com/gpu"
)

func newClusterSnapshotCollector(
	k8sClient client.Client,
	logger logr.Logger,
) *clusterSnapshotCollector {
	return &clusterSnapshotCollector{
		k8sClient: k8sClient,
		logger:    logger.WithName("clusterSnapshot"),
	}
}

type clusterSnapshotCollector struct {
	k8sClient client.Client
	logger    logr.Logger
}

func (c *clusterSnapshotCollector) collect(ctx context.Context) (*v1.SendClusterTelemetryRequest_Payload, error) {
	snapshot, err := c.buildSnapshot(ctx)
	if err != nil {
		return nil, err
	}

	return &v1.SendClusterTelemetryRequest_Payload{
		MessageKind: &v1.SendClusterTelemetryRequest_Payload_ClusterSnapshot{
			ClusterSnapshot: snapshot,
		},
	}, nil
}

func (c *clusterSnapshotCollector) buildSnapshot(ctx context.Context) (*v1.ClusterSnapshot, error) {
	nodeList := &corev1.NodeList{}
	if err := c.k8sClient.List(ctx, nodeList); err != nil {
		return nil, err
	}

	var nodes []*v1.ClusterSnapshot_Node
	for _, node := range nodeList.Items {
		// TODO(kenji): Obtain other GPU attributes such as GPU memory,

		var gpuCapacity int32
		if v, ok := node.Status.Capacity[nvidiaGPU]; ok {
			gpuCapacity = int32(v.ToDec().Value())
		}

		var gpuMemory int64
		if v, ok := node.Labels["nvidia.com/gpu.memory"]; ok {
			m, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return nil, err
			}
			gpuMemory = m * 1024 * 1024 // Convert to bytes
		}

		var product string
		if v, ok := node.Labels["nvidia.com/gpu.product"]; ok {
			product = v
		} else {
			product = "unknown"
		}

		nodes = append(nodes, &v1.ClusterSnapshot_Node{
			Name:           node.Name,
			GpuCapacity:    gpuCapacity,
			MemoryCapacity: gpuMemory,
			NvidiaAttributes: &v1.ClusterSnapshot_Node_NvidiaAttributes{
				Product: product,
			},
		})

	}

	return &v1.ClusterSnapshot{
		Nodes: nodes,
	}, nil
}
