package store

import (
	"time"

	"gorm.io/gorm"
)

// GPUTelemetryHistory represents the cluster telemetry history.
type GPUTelemetryHistory struct {
	gorm.Model

	ClusterID string `gorm:"index"`

	// Message is a marshalled proto message GpuTelemetry.
	Message []byte

	HistoryCreatedAt time.Time `gorm:"index"`
}

// CreateGPUTelemetryHistory creates a new GPU telemetry history.
func (s *S) CreateGPUTelemetryHistory(c *GPUTelemetryHistory) error {
	if err := s.db.Save(c).Error; err != nil {
		return err
	}

	return nil
}

// ListGPUTelemetryHistories returns a list of GPU telemetry histories for the given cluster ID.
func (s *S) ListGPUTelemetryHistories(clusterID string, startTime, endTime time.Time) ([]*GPUTelemetryHistory, error) {
	var hs []*GPUTelemetryHistory
	if err := s.db.
		Where("cluster_id = ?", clusterID).
		Where("history_created_at >= ?", startTime).
		Where("history_created_at < ?", endTime).Find(&hs).Error; err != nil {
		return nil, err
	}
	return hs, nil
}
