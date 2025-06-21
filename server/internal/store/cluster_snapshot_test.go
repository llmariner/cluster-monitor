package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func TestCreateOrUpdateClusterSnapshot(t *testing.T) {
	st, teardown := NewTest(t)
	defer teardown()

	_, err := st.GetClusterSnapshotByID("cid0")
	assert.Error(t, err)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)

	c := &ClusterSnapshot{
		ClusterID: "cid0",
		Name:      "name0",
		TenantID:  "tid0",
	}
	err = st.CreateOrUpdateClusterSnapshot(c)
	assert.NoError(t, err)

	got, err := st.GetClusterSnapshotByID("cid0")
	assert.NoError(t, err)
	assert.Equal(t, c.ClusterID, got.ClusterID)
	assert.Equal(t, c.Name, got.Name)
	assert.Equal(t, c.TenantID, got.TenantID)

	// Update the name.
	c.Name = "name1"
	err = st.CreateOrUpdateClusterSnapshot(c)
	assert.NoError(t, err)

	got, err = st.GetClusterSnapshotByID("cid0")
	assert.NoError(t, err)
	assert.Equal(t, c.Name, got.Name)
}

func TestListClusterSnapshotsByTenantID(t *testing.T) {
	names := func(cs []*ClusterSnapshot) []string {
		var names []string
		for _, c := range cs {
			names = append(names, c.Name)
		}
		return names
	}

	st, teardown := NewTest(t)
	defer teardown()

	cs := []*ClusterSnapshot{
		{
			ClusterID: "cid0",
			Name:      "name0",
			TenantID:  "tid0",
		},
		{
			ClusterID: "cid1",
			Name:      "name1",
			TenantID:  "tid0",
		},
		{
			ClusterID: "cid2",
			Name:      "name2",
			TenantID:  "tid2",
		},
	}

	for _, c := range cs {
		err := st.CreateOrUpdateClusterSnapshot(c)
		assert.NoError(t, err)
	}

	got, err := st.ListClusterSnapshotsByTenantID("tid0")
	assert.NoError(t, err)
	assert.Len(t, got, 2)
	assert.ElementsMatch(t, []string{"name0", "name1"}, names(got))

	got, err = st.ListClusterSnapshotsByTenantID("tid2")
	assert.NoError(t, err)
	assert.Len(t, got, 1)
	assert.ElementsMatch(t, []string{"name2"}, names(got))
}

func TestClusterSnapshotHistory(t *testing.T) {
	ids := func(hs []*ClusterSnapshotHistory) []string {
		var ids []string
		for _, h := range hs {
			ids = append(ids, h.ClusterID)
		}
		return ids
	}

	now := time.Now()

	st, teardown := NewTest(t)
	defer teardown()

	hs := []*ClusterSnapshotHistory{
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
		err := st.CreateClusterSnapshotHistory(h)
		assert.NoError(t, err)
	}

	tcs := []struct {
		name      string
		cid       string
		startTime time.Time
		endTime   time.Time
		want      []*ClusterSnapshotHistory
	}{
		{
			name:      "all histories for cid0",
			cid:       "cid0",
			startTime: now.Add(-time.Hour),
			endTime:   now.Add(2 * time.Hour),
			want:      []*ClusterSnapshotHistory{hs[0], hs[1]},
		},
		{
			name:      "all histories for cid1",
			cid:       "cid1",
			startTime: now.Add(-time.Hour),
			endTime:   now.Add(3 * time.Hour),
			want:      []*ClusterSnapshotHistory{hs[2]},
		},
		{
			name:      "no histories for cid2",
			cid:       "cid2",
			startTime: now.Add(-time.Hour),
			endTime:   now.Add(2 * time.Hour),
			want:      []*ClusterSnapshotHistory{},
		},
		{
			name:      "no histories before start time",
			cid:       "cid0",
			startTime: now.Add(2 * time.Hour),
			endTime:   now.Add(3 * time.Hour),
			want:      []*ClusterSnapshotHistory{},
		},
		{
			name:      "no histories after end time",
			cid:       "cid0",
			startTime: now.Add(-time.Hour),
			endTime:   now.Add(-time.Minute),
			want:      []*ClusterSnapshotHistory{},
		},
		{
			name:      "partial time range",
			cid:       "cid0",
			startTime: now.Add(-time.Hour),
			endTime:   now.Add(time.Hour / 2),
			want:      []*ClusterSnapshotHistory{hs[0]},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			hs, err := st.ListClusterSnapshotHistories(tc.cid, tc.startTime, tc.endTime)
			assert.NoError(t, err)
			assert.ElementsMatch(t, ids(tc.want), ids(hs))
		})
	}

}
