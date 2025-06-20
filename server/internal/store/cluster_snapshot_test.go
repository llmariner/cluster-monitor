package store

import (
	"testing"

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
	st, teardown := NewTest(t)
	defer teardown()

	err := st.CreateClusterSnapshotHistory(&ClusterSnapshotHistory{
		ClusterID: "cid0",
	})
	assert.NoError(t, err)

	err = st.CreateClusterSnapshotHistory(&ClusterSnapshotHistory{
		ClusterID: "cid0",
	})
	assert.NoError(t, err)

	hs, err := st.ListClusterSnapshotHistories("cid0")
	assert.NoError(t, err)
	assert.Len(t, hs, 2)
}
