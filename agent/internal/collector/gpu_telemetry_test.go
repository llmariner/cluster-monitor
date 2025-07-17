package collector

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/go-logr/logr/testr"
	"github.com/google/go-cmp/cmp"
	"github.com/llmariner/cluster-monitor/agent/internal/collector/prometheus"
	v1 "github.com/llmariner/cluster-monitor/api/v1"
	pv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGPUTelemetryCollect(t *testing.T) {
	logger := testr.New(t)
	promClient := &fakePromClient{}

	k8sClient := fake.NewFakeClient(
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					corev1.LabelHostname: "host1",
				},
				Name: "node1",
			},
		},
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					corev1.LabelHostname: "host2",
				},
				Name: "node2",
			},
		},
	)

	cl := newGPUTelemetryCollector(promClient, k8sClient, nil, 10*time.Minute, logger)

	got, err := cl.collect(context.Background())
	assert.NoError(t, err)

	want := map[string]*v1.GpuTelemetry_Node{
		"node1": {
			Name:             "node1",
			MaxGpuUsed:       (0.2 + 0.4),
			AvgGpuUsed:       (0.1 + 0.2 + 0.3 + 0.4) / 2.0,
			MaxGpuMemoryUsed: (1100 + 1300),
			AvgGpuMemoryUsed: (1000 + 1100 + 1200 + 1300) / 2.0,
		},
		"node2": {
			Name:             "node2",
			MaxGpuUsed:       0.6,
			AvgGpuUsed:       (0.5 + 0.6) / 2.0,
			MaxGpuMemoryUsed: 1500,
			AvgGpuMemoryUsed: (1400 + 1500) / 2.0,
		},
	}

	tel := got.GetGpuTelemetry()
	assert.Len(t, tel.Nodes, len(want))
	nodesByName := make(map[string]*v1.GpuTelemetry_Node)
	for _, node := range tel.Nodes {
		nodesByName[node.Name] = node
	}
	for name, got := range nodesByName {
		w, ok := want[name]
		assert.True(t, ok)
		assert.Equal(t, w.Name, got.Name)
		assert.InDelta(t, w.MaxGpuUsed, got.MaxGpuUsed, 1e-6)
		assert.InDelta(t, w.AvgGpuUsed, got.AvgGpuUsed, 1e-6)
		assert.InDelta(t, w.MaxGpuMemoryUsed, got.MaxGpuMemoryUsed, 1e-6)
		assert.InDelta(t, w.AvgGpuMemoryUsed, got.AvgGpuMemoryUsed, 1e-6)
	}
}

func TestBuildGPUTelemetryMessage(t *testing.T) {
	tcs := []struct {
		name             string
		gpuUtilByHost    map[string]map[model.Time][]float64
		gpuMemUsedByHost map[string]map[model.Time][]float64
		nodesByHostname  map[string]*corev1.Node
		want             map[string]*v1.GpuTelemetry_Node
	}{
		{
			name: "success",
			gpuUtilByHost: map[string]map[model.Time][]float64{
				"host1": {
					model.Time(0): {10.0, 20.0},
					model.Time(1): {40.0},
				},
				"host2": {
					model.Time(0): {5.0, 15.0},
				},
			},
			gpuMemUsedByHost: map[string]map[model.Time][]float64{
				"host1": {
					model.Time(0): {100.0, 200.0},
					model.Time(1): {400.0},
				},
				"host2": {
					model.Time(0): {50.0, 150.0},
				},
			},
			nodesByHostname: map[string]*corev1.Node{
				"host1": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
					},
				},
				"host2": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "node2",
					},
				},
			},
			want: map[string]*v1.GpuTelemetry_Node{
				"node1": {
					Name:             "node1",
					MaxGpuUsed:       40.0,
					AvgGpuUsed:       (10.0 + 20.0 + 40.0) / 2.0,
					MaxGpuMemoryUsed: 400.0,
					AvgGpuMemoryUsed: (100.0 + 200.0 + 400.0) / 2.0,
				},
				"node2": {
					Name:             "node2",
					MaxGpuUsed:       5.0 + 15.0,
					AvgGpuUsed:       5.0 + 15.0,
					MaxGpuMemoryUsed: 50.0 + 150.0,
					AvgGpuMemoryUsed: 50.0 + 150.0,
				},
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			got, err := buildGPUTelemetryMessage(
				tc.gpuUtilByHost,
				tc.gpuMemUsedByHost,
				tc.nodesByHostname,
			)
			assert.NoError(t, err)

			assert.Len(t, got.Nodes, len(tc.want))

			nodesByName := make(map[string]*v1.GpuTelemetry_Node)
			for _, node := range got.Nodes {
				nodesByName[node.Name] = node
			}
			for name, got := range nodesByName {
				want, ok := tc.want[name]
				assert.True(t, ok)
				assert.Truef(t, proto.Equal(got, want), cmp.Diff(got, want, protocmp.Transform()))
			}
		})
	}
}

func TestComputeMaxAvg(t *testing.T) {
	tcs := []struct {
		name          string
		samplesByTime map[model.Time][]float64
		wantMax       float64
		wantAvg       float64
	}{
		{
			name: "single sample",
			samplesByTime: map[model.Time][]float64{
				model.Time(0): {10.0},
			},
			wantMax: 10.0,
			wantAvg: 10.0,
		},
		{
			name: "multiple samples",
			samplesByTime: map[model.Time][]float64{
				model.Time(0): {10.0, 20.0, 30.0},
				model.Time(1): {5.0, 15.0},
			},
			wantMax: 10.0 + 20.0 + 30.0,
			wantAvg: (10.0 + 20.0 + 30.0 + 5.0 + 15.0) / 2.0,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			gotMax, gotAvg := computeMaxAvg(tc.samplesByTime)
			assert.InDelta(t, tc.wantMax, gotMax, 1e-6)
			assert.InDelta(t, tc.wantAvg, gotAvg, 1e-6)
		})
	}
}

type fakePromClient struct {
}

func (f *fakePromClient) QueryDCGMMetric(ctx context.Context, metricName string, r pv1.Range) (*prometheus.DCGMMetricSamples, error) {
	t0 := model.Now()
	t1 := t0.Add(30 * time.Second)

	key0 := prometheus.DCGMMetricKey{
		Hostname: "host1",
		GPU:      0,
	}
	key1 := prometheus.DCGMMetricKey{
		Hostname: "host1",
		GPU:      1,
	}
	key2 := prometheus.DCGMMetricKey{
		Hostname: "host2",
		GPU:      2,
	}

	switch metricName {
	case prometheus.GPUUtilMetric:
		return &prometheus.DCGMMetricSamples{
			ValuesByKey: map[prometheus.DCGMMetricKey][]model.SamplePair{
				key0: {
					{Timestamp: t0, Value: 0.1},
					{Timestamp: t1, Value: 0.2},
				},
				key1: {
					{Timestamp: t0, Value: 0.3},
					{Timestamp: t1, Value: 0.4},
				},
				key2: {
					{Timestamp: t0, Value: 0.5},
					{Timestamp: t1, Value: 0.6},
				},
			},
		}, nil
	case prometheus.GPUMemoryUsedMetric:
		return &prometheus.DCGMMetricSamples{
			ValuesByKey: map[prometheus.DCGMMetricKey][]model.SamplePair{
				key0: {
					{Timestamp: t0, Value: 1000},
					{Timestamp: t1, Value: 1100},
				},
				key1: {
					{Timestamp: t0, Value: 1200},
					{Timestamp: t1, Value: 1300},
				},
				key2: {
					{Timestamp: t0, Value: 1400},
					{Timestamp: t1, Value: 1500},
				},
			},
		}, nil
	default:
		return nil, fmt.Errorf("unknown metric %s", metricName)
	}
}
