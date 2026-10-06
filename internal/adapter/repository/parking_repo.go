package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

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
	var existing domain.ParkingSession
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND ended_at IS NULL", userID).
		First(&existing).Error

	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		s := &domain.ParkingSession{
			ID:             uuid.New(),
			UserID:         userID,
			Floor:          in.Floor,
			Zone:           in.Zone,
			Notes:          in.Notes,
			LocationType:   in.LocationType,
			IsDoubleParked: in.IsDoubleParked,
			ParkedAt:       in.ParkedAt,
			RemindersSent:  datatypes.JSON([]byte("[]")),
		}
		if err := r.db.WithContext(ctx).Create(s).Error; err != nil {
			return nil, err
		}
		return s, nil
	case err != nil:
		return nil, err
	}

	// เพิ่งเปลี่ยนจาก "ไม่จอดซ้อนคัน" เป็น "จอดซ้อนคัน" — ล้าง RemindersSent
	// เพื่อไม่ให้รอบที่เคยส่งไปในช่วงจอดซ้อนคันครั้งก่อนบล็อกรอบใหม่
	if in.IsDoubleParked && !existing.IsDoubleParked {
		existing.RemindersSent = datatypes.JSON([]byte("[]"))
	}

	existing.Floor = in.Floor
	existing.Zone = in.Zone
	existing.Notes = in.Notes
	existing.LocationType = in.LocationType
	existing.IsDoubleParked = in.IsDoubleParked
	existing.ParkedAt = in.ParkedAt

	if err := r.db.WithContext(ctx).Save(&existing).Error; err != nil {
		return nil, err
	}
	return &existing, nil
}

func (r *GORMParkingRepository) EndActive(ctx context.Context, userID string) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&domain.ParkingSession{}).
		Where("user_id = ? AND ended_at IS NULL", userID).
		Update("ended_at", now).Error
}

func (r *GORMParkingRepository) GetActive(ctx context.Context, userID string) (*domain.ParkingSession, error) {
	var s domain.ParkingSession
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND ended_at IS NULL", userID).
		First(&s).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *GORMParkingRepository) ListDoubleParkedActive(ctx context.Context) ([]domain.ParkingSession, error) {
	var sessions []domain.ParkingSession
	err := r.db.WithContext(ctx).
		Where("ended_at IS NULL AND is_double_parked = true").
		Find(&sessions).Error
	return sessions, err
}

func (r *GORMParkingRepository) MarkRemindersSent(ctx context.Context, sessionID uuid.UUID, labels []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var s domain.ParkingSession
		// FOR UPDATE กัน worker สอง instance (ถ้ามีวันหน้า) อ่าน RemindersSent
		// ตัวเดิมพร้อมกันแล้วเขียนทับกันเอง เหลือแค่ label ล่าสุดที่เขียนทีหลัง
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", sessionID).First(&s).Error; err != nil {
			return err
		}

		var sent []string
		if len(s.RemindersSent) > 0 {
			if err := json.Unmarshal(s.RemindersSent, &sent); err != nil {
				return err
			}
		}
		existing := make(map[string]bool, len(sent))
		for _, l := range sent {
			existing[l] = true
		}
		for _, l := range labels {
			if !existing[l] {
				sent = append(sent, l)
				existing[l] = true
			}
		}

		raw, err := json.Marshal(sent)
		if err != nil {
			return err
		}
		return tx.Model(&domain.ParkingSession{}).
			Where("id = ?", sessionID).
			Update("reminders_sent", datatypes.JSON(raw)).Error
	})
}
