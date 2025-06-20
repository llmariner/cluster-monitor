package store

import (
	"errors"

	"gorm.io/gorm"
)

// ClusterSnapshot represents the cluster snapshot.
type ClusterSnapshot struct {
	gorm.Model

	ClusterID string `gorm:"uniqueIndex"`
	Name      string

	TenantID string `gorm:"index"`
}

// ClusterSnapshotHistory represents the cluster snapshot history.
type ClusterSnapshotHistory struct {
	gorm.Model

	ClusterID string `gorm:"index"`

	// Message is a marshalled proto message ClusterSnapshot.
	Message []byte

	// TODO(kenji): Add SnapshotCreatedAt?
}

// CreateOrUpdateClusterSnapshot creates a new cluster snapshot or updates the existing one.
func (s *S) CreateOrUpdateClusterSnapshot(c *ClusterSnapshot) error {
	var existing ClusterSnapshot
	if err := s.db.Where("cluster_id = ?", c.ClusterID).Take(&existing).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		// No existing record. Create a new one.
		if err := s.db.Create(c).Error; err != nil {
			return err
		}
		return nil
	}

	if existing.Name == c.Name {
		// No need to update.
		return nil
	}
	existing.Name = c.Name
	if err := s.db.Save(&existing).Error; err != nil {
		return err
	}

	return nil
}

// GetClusterSnapshotByID gets a cluster snapshot by its ID.
func (s *S) GetClusterSnapshotByID(clusterID string) (*ClusterSnapshot, error) {
	var c ClusterSnapshot
	if err := s.db.Where("cluster_id = ?", clusterID).Take(&c).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

// ListClusterSnapshotsByTenantID lists cluster snapshots by tenant ID.
func (s *S) ListClusterSnapshotsByTenantID(tenantID string) ([]*ClusterSnapshot, error) {
	var cs []*ClusterSnapshot
	if err := s.db.Where("tenant_id = ?", tenantID).Find(&cs).Error; err != nil {
		return nil, err
	}
	return cs, nil
}

// CreateClusterSnapshotHistory creates a new cluster snapshot history.
func (s *S) CreateClusterSnapshotHistory(c *ClusterSnapshotHistory) error {
	if err := s.db.Save(c).Error; err != nil {
		return err
	}

	return nil
}

// ListClusterSnapshotHistories returns a list of cluster snapshots for the given cluster ID.
func (s *S) ListClusterSnapshotHistories(clusterID string) ([]*ClusterSnapshotHistory, error) {
	var hs []*ClusterSnapshotHistory
	if err := s.db.Where("cluster_id = ?", clusterID).Find(&hs).Error; err != nil {
		return nil, err
	}
	return hs, nil
}
