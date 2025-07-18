package collector

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/llmariner/cluster-monitor/agent/internal/collector/prometheus"
	v1 "github.com/llmariner/cluster-monitor/api/v1"
	pv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	defaultPrometheusStep = 30 * time.Second
)

type prometheusClient interface {
	QueryDCGMMetric(ctx context.Context, metricName string, r pv1.Range) (*prometheus.DCGMMetricSamples, error)
}

func newGPUTelemetryCollector(
	promClient prometheusClient,
	k8sClient client.Client,
	targetNodeSelector map[string]string,
	metricsDuration time.Duration,
	logger logr.Logger,
) *gpuTelemetryCollector {
	return &gpuTelemetryCollector{
		promClient:         promClient,
		k8sClient:          k8sClient,
		targetNodeSelector: targetNodeSelector,
		metricsDuration:    metricsDuration,
		logger:             logger.WithName("gpuTelemetry"),
	}
}

type gpuTelemetryCollector struct {
	promClient prometheusClient
	k8sClient  client.Client

	targetNodeSelector map[string]string

	metricsDuration time.Duration

	logger logr.Logger
}

func (c *gpuTelemetryCollector) collect(ctx context.Context) (*v1.SendClusterTelemetryRequest_Payload, error) {
	c.logger.Info("Collecting GPU telemetry")

	// TODO(kenji): We might want to add some adjustments to the time range to take into the delay
	// in metrics collection.
	now := time.Now()
	rng := pv1.Range{
		Start: now.Add(-c.metricsDuration),
		End:   now,
		Step:  defaultPrometheusStep,
	}

	nodesByHostname, err := c.buildHostnameToNodes(ctx)
	if err != nil {
		return nil, err
	}

	gpuUtilSamples, err := c.promClient.QueryDCGMMetric(ctx, prometheus.GPUUtilMetric, rng)
	if err != nil {
		return nil, err
	}
	gpuUtilByHost := buildSampleMapByHost(gpuUtilSamples)

	gpuMemUsedSamples, err := c.promClient.QueryDCGMMetric(ctx, prometheus.GPUMemoryUsedMetric, rng)
	if err != nil {
		return nil, err
	}
	gpuMemUsedByHost := buildSampleMapByHost(gpuMemUsedSamples)

	gpuTelemetry, err := buildGPUTelemetryMessage(gpuUtilByHost, gpuMemUsedByHost, nodesByHostname)
	if err != nil {
		return nil, err
	}

	c.logger.Info("Collected GPU telemetry",
		"nodes", len(gpuTelemetry.Nodes),
		"GPU util sample keys", len(gpuUtilSamples.ValuesByKey),
		"GPU memory used sample keys", len(gpuMemUsedSamples.ValuesByKey),
	)

	return &v1.SendClusterTelemetryRequest_Payload{
		MessageKind: &v1.SendClusterTelemetryRequest_Payload_GpuTelemetry{
			GpuTelemetry: gpuTelemetry,
		},
	}, nil
}

func (c *gpuTelemetryCollector) buildHostnameToNodes(ctx context.Context) (map[string]*corev1.Node, error) {
	nodeList := &corev1.NodeList{}
	if err := c.k8sClient.List(ctx, nodeList, &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(c.targetNodeSelector),
	}); err != nil {
		return nil, err
	}

	nodesByHostname := make(map[string]*corev1.Node)
	for _, node := range nodeList.Items {
		hostname, ok := node.Labels[corev1.LabelHostname]
		if !ok {
			c.logger.Info("Node does not have a hostname label, skipping", "node", node.Name)
			continue
		}
		nodesByHostname[hostname] = &node
	}
	return nodesByHostname, nil
}

func buildSampleMapByHost(samples *prometheus.DCGMMetricSamples) map[string]map[model.Time][]float64 {
	samplesByHost := make(map[string]map[model.Time][]float64)

	for k, samples := range samples.ValuesByKey {
		// Ignore host-level metrics (i.e., no pod labels).
		if k.Pod == "" {
			continue
		}

		m, ok := samplesByHost[k.Hostname]
		if !ok {
			m = make(map[model.Time][]float64)
			samplesByHost[k.Hostname] = m
		}

		for _, s := range samples {
			m[s.Timestamp] = append(m[s.Timestamp], float64(s.Value))
		}
	}

	return samplesByHost
}

func buildGPUTelemetryMessage(
	gpuUtilByHost map[string]map[model.Time][]float64,
	gpuMemUsedByHost map[string]map[model.Time][]float64,
	nodesByHostname map[string]*corev1.Node,
) (*v1.GpuTelemetry, error) {
	var nodes []*v1.GpuTelemetry_Node
	// Collect unique hostnames from both GPU utilization and memory used samples
	hostnames := make(map[string]struct{})
	for hostname := range gpuUtilByHost {
		hostnames[hostname] = struct{}{}
	}
	for hostname := range gpuMemUsedByHost {
		hostnames[hostname] = struct{}{}
	}

	for hostname := range hostnames {
		node, ok := nodesByHostname[hostname]
		if !ok {
			return nil, fmt.Errorf("node not found for hostname %s", hostname)
		}

		var maxGPUUsed, avgGPUUsed float64
		if samplesByTime, ok := gpuUtilByHost[hostname]; ok {
			maxGPUUsed, avgGPUUsed = computeMaxAvg(samplesByTime)
		}

		var maxGPUMemUsed, avgGPUMemUsed float64
		if samplesByTime, ok := gpuMemUsedByHost[hostname]; ok {
			maxGPUMemUsed, avgGPUMemUsed = computeMaxAvg(samplesByTime)
		}

		nodes = append(nodes, &v1.GpuTelemetry_Node{
			Name:       node.Name,
			MaxGpuUsed: float32(maxGPUUsed),
			AvgGpuUsed: float32(avgGPUUsed),
			// Convert MB to bytes.
			MaxGpuMemoryUsed: int64(maxGPUMemUsed * 1024 * 1024),
			AvgGpuMemoryUsed: int64(avgGPUMemUsed * 1024 * 1024),
		})
	}

	return &v1.GpuTelemetry{Nodes: nodes}, nil
}

func computeMaxAvg(samplesByTime map[model.Time][]float64) (float64, float64) {
	var maxV, avgV float64

	for _, samples := range samplesByTime {
		var total float64
		for _, v := range samples {
			total += v
		}
		maxV = max(maxV, total)
		avgV += total
	}
	avgV /= float64(len(samplesByTime))

	return maxV, avgV
}
