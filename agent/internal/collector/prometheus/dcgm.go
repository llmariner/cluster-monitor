package prometheus

import (
	"fmt"
	"strconv"

	"github.com/prometheus/common/model"
)

const (
	// GPUUtilMetric is the metric for GPU utilization.
	GPUUtilMetric = "DCGM_FI_DEV_GPU_UTIL"
	// GPUMemoryUsedMetric is the metric for GPU memory used.
	GPUMemoryUsedMetric = "DCGM_FI_DEV_FB_USED"
	// GPUMemoryFreeMetric is the metric for GPU memory free.
	GPUMemoryFreeMetric = "DCGM_FI_DEV_FB_FREE"
)

// DCGMMetricKey is a key for DCGM metrics.
type DCGMMetricKey struct {
	Hostname string
	GPU      int

	Namespace string
	Pod       string
	Container string
}

// NewDCGMMetricKey creates a new DCGMMetricKey from a metric.
func NewDCGMMetricKey(m model.Metric) (DCGMMetricKey, error) {
	hostname, ok := m["Hostname"]
	if !ok {
		return DCGMMetricKey{}, fmt.Errorf("label 'hostname' not found in label set: %v", m)
	}

	gpuStr, ok := m["gpu"]
	if !ok {
		return DCGMMetricKey{}, fmt.Errorf("label 'gpu' not found in label set: %v", m)
	}
	gpu, err := strconv.Atoi(string(gpuStr))
	if err != nil {
		return DCGMMetricKey{}, fmt.Errorf("label 'gpu' is not a valid integer: %s", gpuStr)
	}

	namespace, ok := m["namespace"]
	if !ok {
		return DCGMMetricKey{}, fmt.Errorf("label 'namespace' not found in label set: %v", m)
	}

	pod, ok := m["exported_pod"]
	if !ok {
		return DCGMMetricKey{}, fmt.Errorf("label 'exported_pod' not found in label set: %v", m)
	}

	container, ok := m["exported_container"]
	if !ok {
		return DCGMMetricKey{}, fmt.Errorf("label 'exported_container' not found in label set: %v", m)
	}

	return DCGMMetricKey{
		Hostname:  string(hostname),
		GPU:       gpu,
		Namespace: string(namespace),
		Pod:       string(pod),
		Container: string(container),
	}, nil
}

// DCGMMetricSamples holds samples for DCGM metrics.
type DCGMMetricSamples struct {
	ValuesByKey map[DCGMMetricKey][]float64
}
