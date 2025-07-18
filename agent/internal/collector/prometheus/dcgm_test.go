package prometheus

import (
	"testing"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
)

func TestNewDCGMMetricKey(t *testing.T) {
	tcs := []struct {
		name    string
		m       model.Metric
		want    DCGMMetricKey
		wantErr bool
	}{
		{
			name: "valid",
			m: model.Metric{
				"Hostname":           "test-host",
				"gpu":                "0",
				"exported_namespace": "test-namespace",
				"exported_pod":       "test-pod",
				"exported_container": "test-container",
			},
			want: DCGMMetricKey{
				Hostname:  "test-host",
				GPU:       0,
				Namespace: "test-namespace",
				Pod:       "test-pod",
				Container: "test-container",
			},
		},
		{
			name: "invalid",
			m: model.Metric{
				"Hostname": "test-host",
			},
			wantErr: true,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NewDCGMMetricKey(tc.m)
			if tc.wantErr {
				assert.Error(t, err)

				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
