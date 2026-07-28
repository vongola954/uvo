package services

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"uvo/internal/models"
)

const (
	JobPending    = "pending"
	JobProcessing = "processing"
	JobDone       = "done"
	JobFailed     = "failed"
)

type JobStore struct {
	db *gorm.DB
}

func NewJobStore(db *gorm.DB) *JobStore {
	return &JobStore{db: db}
}

func idemKey(userID, requestID, jobID string) string {
	if requestID != "" {
		return userID + "|" + requestID
	}
	return jobID
}

// CreateOrClaim inserts a pending job. created=true only for the winning insert —
// callers must Spend/start worker only when created. Concurrent same request_id
// returns the existing row with created=false (no double spend).
func (s *JobStore) CreateOrClaim(userID, requestID string) (job *models.JobRecord, created bool) {
	if requestID != "" {
		var existing models.JobRecord
		if err := s.db.Where("idem_key = ?", idemKey(userID, requestID, "")).First(&existing).Error; err == nil {
			return &existing, false
		}
	}
	id := uuid.New().String()
	j := &models.JobRecord{
		ID:        id,
		UserID:    userID,
		Status:    JobPending,
		RequestID: requestID,
		IdemKey:   idemKey(userID, requestID, id),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := s.db.Create(j).Error; err != nil {
		if requestID != "" {
			var existing models.JobRecord
			if s.db.Where("idem_key = ?", j.IdemKey).First(&existing).Error == nil {
				return &existing, false
			}
		}
		// Create failed without recoverable idem row — do not treat as owned.
		return j, false
	}
	return j, true
}

// Create is CreateOrClaim discarding the created flag (legacy).
func (s *JobStore) Create(userID, requestID string) *models.JobRecord {
	j, _ := s.CreateOrClaim(userID, requestID)
	return j
}

// ClaimProcessing CAS pending → processing. Only one worker wins.
func (s *JobStore) ClaimProcessing(id string) bool {
	res := s.db.Model(&models.JobRecord{}).
		Where("id = ? AND status = ?", id, JobPending).
		Updates(map[string]interface{}{
			"status":     JobProcessing,
			"updated_at": time.Now(),
		})
	return res.Error == nil && res.RowsAffected == 1
}

// Delete removes a job (e.g. pending row after Spend failed).
func (s *JobStore) Delete(id string) {
	_ = s.db.Delete(&models.JobRecord{}, "id = ?", id).Error
}

func (s *JobStore) Get(id string) (*models.JobRecord, bool) {
	var j models.JobRecord
	if err := s.db.First(&j, "id = ?", id).Error; err != nil {
		return nil, false
	}
	return &j, true
}

func (s *JobStore) GetByRequestID(userID, requestID string) (*models.JobRecord, bool) {
	if requestID == "" {
		return nil, false
	}
	var j models.JobRecord
	if err := s.db.Where("idem_key = ?", idemKey(userID, requestID, "")).First(&j).Error; err != nil {
		return nil, false
	}
	return &j, true
}

func (s *JobStore) Update(id string, fn func(*models.JobRecord)) {
	var j models.JobRecord
	if err := s.db.First(&j, "id = ?", id).Error; err != nil {
		return
	}
	fn(&j)
	j.UpdatedAt = time.Now()
	_ = s.db.Save(&j).Error
}

// CleanupOlderThan deletes finished jobs older than age (keeps pending/processing).
func (s *JobStore) CleanupOlderThan(age time.Duration) (int64, error) {
	if age <= 0 {
		age = 7 * 24 * time.Hour
	}
	cut := time.Now().Add(-age)
	res := s.db.Where("created_at < ? AND status IN ?", cut, []string{JobDone, JobFailed}).
		Delete(&models.JobRecord{})
	return res.RowsAffected, res.Error
}
