package collector

import (
	"context"
	"fmt"
	"strconv"

	"github.com/go-logr/logr"
	v1 "github.com/llmariner/cluster-monitor/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	nvidiaGPU corev1.ResourceName = "nvidia.com/gpu"
)

func newClusterSnapshotCollector(
	k8sClient client.Client,
	targetNodeSelector map[string]string,
	logger logr.Logger,
) *clusterSnapshotCollector {
	return &clusterSnapshotCollector{
		k8sClient:          k8sClient,
		targetNodeSelector: targetNodeSelector,
		logger:             logger.WithName("clusterSnapshot"),
	}
}

type clusterSnapshotCollector struct {
	k8sClient          client.Client
	targetNodeSelector map[string]string
	logger             logr.Logger
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
	if err := c.k8sClient.List(ctx, nodeList, &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(c.targetNodeSelector),
	}); err != nil {
		return nil, err
	}
	c.logger.Info("Found Nodes", "count", len(nodeList.Items))

	nodesByName := make(map[string]*v1.ClusterSnapshot_Node)
	for _, node := range nodeList.Items {
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

		nodesByName[node.Name] = &v1.ClusterSnapshot_Node{
			Name:           node.Name,
			GpuCapacity:    gpuCapacity,
			MemoryCapacity: gpuMemory,
			NvidiaAttributes: &v1.ClusterSnapshot_Node_NvidiaAttributes{
				Product: product,
			},
		}
	}

	podList := &corev1.PodList{}
	if err := c.k8sClient.List(ctx, podList); err != nil {
		return nil, err
	}
	c.logger.Info("Found Pods", "count", len(podList.Items))
	for _, pod := range podList.Items {
		if pod.Status.Phase != corev1.PodRunning {
			continue
		}

		v, err := requestedGPUs(&pod)
		if err != nil {
			return nil, fmt.Errorf("requested gpus for pod %s: %s", pod.Name, err)
		}
		if v == 0 {
			continue
		}

		node, ok := nodesByName[pod.Spec.NodeName]
		if !ok {
			// TODO(kenji): Revisit. This can happen when a node is being deleted?
			return nil, fmt.Errorf("node %s not found for pod %s", pod.Spec.NodeName, pod.Name)
		}
		node.GpuOccupancy += int32(v)
		node.PodCount++
	}

	var nodes []*v1.ClusterSnapshot_Node
	for _, n := range nodesByName {
		nodes = append(nodes, n)
	}

	return &v1.ClusterSnapshot{
		Nodes: nodes,
	}, nil
}

func requestedGPUs(pod *corev1.Pod) (int, error) {
	total := 0
	for _, con := range pod.Spec.Containers {
		limit := con.Resources.Limits
		if limit == nil {
			continue
		}

		v, ok := limit[nvidiaGPU]
		if !ok {
			continue
		}
		count, ok := v.AsInt64()
		if !ok {
			return -1, fmt.Errorf("asint64 for %v", v)
		}
		total += int(count)
	}
	return total, nil
}
