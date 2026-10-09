// Package model เก็บ struct ที่ผูกกับ GORM/schema โดยตรง — ไม่มีใครนอก
// adapter/repository เห็น struct พวกนี้เลย แปลงเป็น domain type ที่ชายแดน
// ด้วย ToDomain()/FromDomain() ตามแบบของ vertex-auth-service
package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"

	"github.com/vertex/ev-service/internal/domain"
)

type ParkingSessionRow struct {
	ID uuid.UUID `gorm:"type:uuid;primaryKey"`

	UserID string `gorm:"not null"`

	Floor          string
	Zone           string
	Notes          string
	LocationType   string `gorm:"not null"`
	IsDoubleParked bool   `gorm:"not null;default:false"`

	ParkedAt time.Time `gorm:"not null"`

	// RemindersSent เก็บเป็น raw JSON array ในฐานข้อมูล — แปลงเป็น []string
	// ตรง ToDomain()/FromDomain() เท่านั้น ไม่มีใครนอกไฟล์นี้เห็น datatypes.JSON
	RemindersSent datatypes.JSON `gorm:"not null;default:'[]'"`

	EndedAt *time.Time

	CreatedAt time.Time `gorm:"not null;default:now()"`
	UpdatedAt time.Time `gorm:"not null;default:now()"`
}

func (ParkingSessionRow) TableName() string { return "parking_sessions" }

// ToDomain แปลงแถวที่อ่านจาก DB เป็น domain type — error ได้เฉพาะตอน
// RemindersSent เป็น JSON ที่ parse ไม่ออก (ไม่ควรเกิดถ้าเขียนผ่าน
// FromDomain เท่านั้น แต่ไม่ไว้ใจเดาเอาว่าข้อมูลในตารางสะอาดเสมอ)
func (r ParkingSessionRow) ToDomain() (domain.ParkingSession, error) {
	sent, err := unmarshalRemindersSent(r.RemindersSent)
	if err != nil {
		return domain.ParkingSession{}, err
	}
	return domain.ParkingSession{
		ID:             r.ID,
		UserID:         r.UserID,
		Floor:          r.Floor,
		Zone:           r.Zone,
		Notes:          r.Notes,
		LocationType:   r.LocationType,
		IsDoubleParked: r.IsDoubleParked,
		ParkedAt:       r.ParkedAt,
		RemindersSent:  sent,
		EndedAt:        r.EndedAt,
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
	}, nil
}

// ParkingSessionRowFromDomain แปลง domain type เป็นแถวที่จะเขียนลง DB —
// ไม่มี error เพราะ json.Marshal ของ []string ไม่มีวันพัง (ไม่มี cyclic
// reference หรือ type ที่ marshal ไม่ได้)
//
// 🔴 nil ต้อง marshal เป็น "[]" ไม่ใช่ "null" — ให้ตรงกับ default ของ column
// (เดิม UpsertActive เซ็ต datatypes.JSON([]byte("[]")) ตรงๆ ตอนสร้างใหม่)
func ParkingSessionRowFromDomain(s domain.ParkingSession) ParkingSessionRow {
	sent := s.RemindersSent
	if sent == nil {
		sent = []string{}
	}
	raw, _ := json.Marshal(sent)
	return ParkingSessionRow{
		ID:             s.ID,
		UserID:         s.UserID,
		Floor:          s.Floor,
		Zone:           s.Zone,
		Notes:          s.Notes,
		LocationType:   s.LocationType,
		IsDoubleParked: s.IsDoubleParked,
		ParkedAt:       s.ParkedAt,
		RemindersSent:  datatypes.JSON(raw),
		EndedAt:        s.EndedAt,
		CreatedAt:      s.CreatedAt,
		UpdatedAt:      s.UpdatedAt,
	}
}

func unmarshalRemindersSent(raw datatypes.JSON) ([]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var sent []string
	if err := json.Unmarshal(raw, &sent); err != nil {
		return nil, err
	}
	return sent, nil
}
