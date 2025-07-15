package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGPUTelemetryHistory(t *testing.T) {
	ids := func(hs []*GPUTelemetryHistory) []string {
		var ids []string
		for _, h := range hs {
			ids = append(ids, h.ClusterID)
		}
		return ids
	}

	now := time.Now()

	st, teardown := NewTest(t)
	defer teardown()

	hs := []*GPUTelemetryHistory{
		{
			ClusterID:        "cid0",
			HistoryCreatedAt: now,
		},
		{
			ClusterID:        "cid0",
			HistoryCreatedAt: now.Add(time.Hour),
		},
		{
			ClusterID:        "cid1",
			HistoryCreatedAt: now.Add(2 * time.Hour),
		},
	}
	for _, h := range hs {
		err := st.CreateGPUTelemetryHistory(h)
		assert.NoError(t, err)
	}

	tcs := []struct {
		name      string
		cid       string
		startTime time.Time
		endTime   time.Time
		want      []*GPUTelemetryHistory
	}{
		{
			name:      "all histories for cid0",
			cid:       "cid0",
			startTime: now.Add(-time.Hour),
			endTime:   now.Add(2 * time.Hour),
			want:      []*GPUTelemetryHistory{hs[0], hs[1]},
		},
		{
			name:      "all histories for cid1",
			cid:       "cid1",
			startTime: now.Add(-time.Hour),
			endTime:   now.Add(3 * time.Hour),
			want:      []*GPUTelemetryHistory{hs[2]},
		},
		{
			name:      "no histories for cid2",
			cid:       "cid2",
			startTime: now.Add(-time.Hour),
			endTime:   now.Add(2 * time.Hour),
			want:      []*GPUTelemetryHistory{},
		},
		{
			name:      "no histories before start time",
			cid:       "cid0",
			startTime: now.Add(2 * time.Hour),
			endTime:   now.Add(3 * time.Hour),
			want:      []*GPUTelemetryHistory{},
		},
		{
			name:      "no histories after end time",
			cid:       "cid0",
			startTime: now.Add(-time.Hour),
			endTime:   now.Add(-time.Minute),
			want:      []*GPUTelemetryHistory{},
		},
		{
			name:      "partial time range",
			cid:       "cid0",
			startTime: now.Add(-time.Hour),
			endTime:   now.Add(time.Hour / 2),
			want:      []*GPUTelemetryHistory{hs[0]},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			hs, err := st.ListGPUTelemetryHistories(tc.cid, tc.startTime, tc.endTime)
			assert.NoError(t, err)
			assert.ElementsMatch(t, ids(tc.want), ids(hs))
		})
	}

}
