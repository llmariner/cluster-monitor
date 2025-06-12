package store

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
)

// ClusterSnapshot represents the cluster snapshot.
//
// TODO(kenji): Store historical data instead of just keeping the current status.
type ClusterSnapshot struct {
	gorm.Model

	ClusterID string `gorm:"uniqueIndex"`
	Name      string

	TenantID string `gorm:"index"`

	// Message is a marshalled proto message ClusterSnapshot.
	Message []byte
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

	// Found an existing record. Update it.
	if existing.ClusterID != c.ClusterID {
		return fmt.Errorf("cluster ID mismatch: cannot update existing record with different cluster ID: %s != %s", existing.ClusterID, c.ClusterID)
	}
	if existing.TenantID != c.TenantID {
		return fmt.Errorf("tenant ID mismatch: cannot update existing record with different tenant ID: %s != %s", existing.TenantID, c.TenantID)
	}

	existing.Name = c.Name
	existing.Message = c.Message
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
