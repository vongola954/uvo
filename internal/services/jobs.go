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
func (s *JobStore) CreateOrClaim(userID, requestID string) (job *models.JobRecord, created bool, err error) {
	if requestID != "" {
		var existing models.JobRecord
		if err := s.db.Where("idem_key = ?", idemKey(userID, requestID, "")).First(&existing).Error; err == nil {
			return &existing, false, nil
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
				return &existing, false, nil
			}
		}
		// Create failed without recoverable idem row — never return a phantom job.
		return nil, false, err
	}
	return j, true, nil
}

// Create is CreateOrClaim discarding the created flag (legacy).
func (s *JobStore) Create(userID, requestID string) *models.JobRecord {
	j, _, _ := s.CreateOrClaim(userID, requestID)
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

const DefaultStaleJobAge = 15 * time.Minute

// SetCreditsSpent records how many credits were taken for this job (for refunds).
func (s *JobStore) SetCreditsSpent(id string, n int) {
	if n <= 0 {
		return
	}
	_ = s.db.Model(&models.JobRecord{}).Where("id = ?", id).
		Updates(map[string]interface{}{"credits_spent": n, "updated_at": time.Now()}).Error
}

// MarkFailedRefunded CAS-fails a job and marks credits as already refunded (avoids double refund).
func (s *JobStore) MarkFailedRefunded(id, errMsg string) {
	_ = s.TryMarkFailedRefunded(id, errMsg)
}

// TryMarkFailedRefunded CAS-fails pending/processing → failed+refunded.
// Returns true only when this caller should issue the credit refund.
func (s *JobStore) TryMarkFailedRefunded(id, errMsg string) bool {
	res := s.db.Model(&models.JobRecord{}).
		Where("id = ? AND status IN ? AND refunded = ?", id, []string{JobPending, JobProcessing}, false).
		Updates(map[string]interface{}{
			"status":     JobFailed,
			"error":      errMsg,
			"refunded":   true,
			"updated_at": time.Now(),
		})
	return res.Error == nil && res.RowsAffected == 1
}

// CompleteDone CAS-marks a processing job done. Fails if already failed/refunded/timed out.
func (s *JobStore) CompleteDone(id string, fields map[string]interface{}) bool {
	if fields == nil {
		fields = map[string]interface{}{}
	}
	fields["status"] = JobDone
	fields["updated_at"] = time.Now()
	res := s.db.Model(&models.JobRecord{}).
		Where("id = ? AND status = ? AND refunded = ?", id, JobProcessing, false).
		Updates(fields)
	return res.Error == nil && res.RowsAffected == 1
}

// Touch bumps updated_at on a processing job (heartbeat against stale sweeper).
// Jobs older than AbsoluteMaxJobAge stop receiving heartbeats so they can be swept.
func (s *JobStore) Touch(id string) {
	cut := time.Now().Add(-AbsoluteMaxJobAge)
	_ = s.db.Model(&models.JobRecord{}).
		Where("id = ? AND status = ? AND created_at > ?", id, JobProcessing, cut).
		Update("updated_at", time.Now()).Error
}

const DefaultStaleProcessingAge = 45 * time.Minute

// AbsoluteMaxJobAge caps how long heartbeats can keep a job alive.
const AbsoluteMaxJobAge = 2 * time.Hour

// FailStaleAndRefund fails stale pending/processing jobs and refunds CreditsSpent.
// Pending uses `age` (default 15m). Processing uses a longer window so AceMusic jobs
// with heartbeats are not refunded while still running.
func (s *JobStore) FailStaleAndRefund(age time.Duration, credits *CreditService) int {
	if age <= 0 {
		age = DefaultStaleJobAge
	}
	if credits == nil {
		return 0
	}
	n := 0
	n += s.failStaleStatus(JobPending, age, credits)
	n += s.failStaleStatus(JobProcessing, DefaultStaleProcessingAge, credits)
	n += s.failAbsoluteMaxAge(credits)
	return n
}

func (s *JobStore) failStaleStatus(status string, age time.Duration, credits *CreditService) int {
	cut := time.Now().Add(-age)
	var list []models.JobRecord
	if err := s.db.Where("status = ? AND updated_at < ? AND refunded = ?", status, cut, false).
		Find(&list).Error; err != nil {
		return 0
	}
	n := 0
	for _, j := range list {
		if !s.tryMarkFailedRefundedBefore(j.ID, "timeout: генерация не завершилась вовремя, кредиты возвращены", cut) {
			continue
		}
		spent := j.CreditsSpent
		if spent <= 0 {
			spent = 1
		}
		credits.Refund(j.UserID, spent)
		n++
	}
	return n
}

func (s *JobStore) failAbsoluteMaxAge(credits *CreditService) int {
	cut := time.Now().Add(-AbsoluteMaxJobAge)
	var list []models.JobRecord
	if err := s.db.Where("status IN ? AND created_at < ? AND refunded = ?",
		[]string{JobPending, JobProcessing}, cut, false).Find(&list).Error; err != nil {
		return 0
	}
	n := 0
	for _, j := range list {
		if !s.TryMarkFailedRefunded(j.ID, "timeout: превышен абсолютный лимит генерации, кредиты возвращены") {
			continue
		}
		spent := j.CreditsSpent
		if spent <= 0 {
			spent = 1
		}
		credits.Refund(j.UserID, spent)
		n++
	}
	return n
}

// tryMarkFailedRefundedBefore CAS-fails only if updated_at is still before cut
// (avoids racing a live heartbeat Touch).
func (s *JobStore) tryMarkFailedRefundedBefore(id, errMsg string, cut time.Time) bool {
	res := s.db.Model(&models.JobRecord{}).
		Where("id = ? AND status IN ? AND refunded = ? AND updated_at < ?",
			id, []string{JobPending, JobProcessing}, false, cut).
		Updates(map[string]interface{}{
			"status":     JobFailed,
			"error":      errMsg,
			"refunded":   true,
			"updated_at": time.Now(),
		})
	return res.Error == nil && res.RowsAffected == 1
}
