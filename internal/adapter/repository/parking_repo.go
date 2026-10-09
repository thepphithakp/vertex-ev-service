package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/vertex/ev-service/internal/adapter/repository/model"
	"github.com/vertex/ev-service/internal/domain"
	"github.com/vertex/ev-service/internal/port"
)

type GORMParkingRepository struct {
	db *gorm.DB
}

func NewGORMParkingRepository(db *gorm.DB) *GORMParkingRepository {
	return &GORMParkingRepository{db: db}
}

// UpsertActive อัปเดต session ที่ active อยู่ ถ้ายังไม่มีก็สร้างใหม่
//
// ไม่ใช้ ON CONFLICT เหมือน push_repo.go เพราะ "active session" ไม่ได้กันซ้ำ
// ด้วย unique key ตรงๆ (ux_parking_sessions_active_user เป็น partial unique
// index ที่ GORM เขียน ON CONFLICT ชี้ตรงไม่ได้ง่ายๆ) — ทำเป็น select แล้ว
// update/create แทน ยอมรับ race เล็กน้อยได้เพราะ user คนเดียวกันไม่น่ากดพร้อมกัน
// สองแท็บพอดีเป๊ะ
func (r *GORMParkingRepository) UpsertActive(ctx context.Context, userID string, in port.ParkingInput) (*domain.ParkingSession, error) {
	var existing model.ParkingSessionRow
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND ended_at IS NULL", userID).
		First(&existing).Error

	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		row := model.ParkingSessionRowFromDomain(domain.ParkingSession{
			ID:             uuid.New(),
			UserID:         userID,
			Floor:          in.Floor,
			Zone:           in.Zone,
			Notes:          in.Notes,
			LocationType:   in.LocationType,
			IsDoubleParked: in.IsDoubleParked,
			ParkedAt:       in.ParkedAt,
			RemindersSent:  nil,
		})
		if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
			return nil, err
		}
		created, err := row.ToDomain()
		if err != nil {
			return nil, err
		}
		return &created, nil
	case err != nil:
		return nil, err
	}

	s, err := existing.ToDomain()
	if err != nil {
		return nil, err
	}

	// เพิ่งเปลี่ยนจาก "ไม่จอดซ้อนคัน" เป็น "จอดซ้อนคัน" — ล้าง RemindersSent
	// เพื่อไม่ให้รอบที่เคยส่งไปในช่วงจอดซ้อนคันครั้งก่อนบล็อกรอบใหม่
	if in.IsDoubleParked && !s.IsDoubleParked {
		s.RemindersSent = nil
	}

	s.Floor = in.Floor
	s.Zone = in.Zone
	s.Notes = in.Notes
	s.LocationType = in.LocationType
	s.IsDoubleParked = in.IsDoubleParked
	s.ParkedAt = in.ParkedAt

	updated := model.ParkingSessionRowFromDomain(s)
	if err := r.db.WithContext(ctx).Save(&updated).Error; err != nil {
		return nil, err
	}
	result, err := updated.ToDomain()
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (r *GORMParkingRepository) EndActive(ctx context.Context, userID string) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&model.ParkingSessionRow{}).
		Where("user_id = ? AND ended_at IS NULL", userID).
		Update("ended_at", now).Error
}

func (r *GORMParkingRepository) GetActive(ctx context.Context, userID string) (*domain.ParkingSession, error) {
	var row model.ParkingSessionRow
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND ended_at IS NULL", userID).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	s, err := row.ToDomain()
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *GORMParkingRepository) ListDoubleParkedActive(ctx context.Context) ([]domain.ParkingSession, error) {
	var rows []model.ParkingSessionRow
	if err := r.db.WithContext(ctx).
		Where("ended_at IS NULL AND is_double_parked = true").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	sessions := make([]domain.ParkingSession, 0, len(rows))
	for _, row := range rows {
		s, err := row.ToDomain()
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, nil
}

func (r *GORMParkingRepository) MarkRemindersSent(ctx context.Context, sessionID uuid.UUID, labels []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row model.ParkingSessionRow
		// FOR UPDATE กัน worker สอง instance (ถ้ามีวันหน้า) อ่าน RemindersSent
		// ตัวเดิมพร้อมกันแล้วเขียนทับกันเอง เหลือแค่ label ล่าสุดที่เขียนทีหลัง
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", sessionID).First(&row).Error; err != nil {
			return err
		}
		s, err := row.ToDomain()
		if err != nil {
			return err
		}

		existing := make(map[string]bool, len(s.RemindersSent))
		for _, l := range s.RemindersSent {
			existing[l] = true
		}
		for _, l := range labels {
			if !existing[l] {
				s.RemindersSent = append(s.RemindersSent, l)
				existing[l] = true
			}
		}

		updated := model.ParkingSessionRowFromDomain(s)
		return tx.Model(&model.ParkingSessionRow{}).
			Where("id = ?", sessionID).
			Update("reminders_sent", updated.RemindersSent).Error
	})
}
