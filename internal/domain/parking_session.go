package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// ParkingSession คือที่จอดรถปัจจุบันของผู้ใช้คนหนึ่ง — มีได้แค่หนึ่งแถวที่
// EndedAt เป็น NULL ต่อ user (บังคับด้วย unique partial index ในฐานข้อมูล)
//
// พอร์ตมาจาก ParkingState ของ PWA (src/lib/db.ts) ซึ่งเดิมเก็บใน IndexedDB
// อย่างเดียว — ตารางนี้คือฝั่ง server ที่ทำให้ worker รู้ว่าต้องเตือนใครเมื่อไหร่
type ParkingSession struct {
	ID uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`

	UserID string `gorm:"not null" json:"userId"`

	Floor          string `json:"floor"`
	Zone           string `json:"zone"`
	Notes          string `json:"notes"`
	LocationType   string `gorm:"not null" json:"locationType"`
	IsDoubleParked bool   `gorm:"not null;default:false" json:"isDoubleParked"`

	// ParkedAt คือเวลาที่เริ่มจอด — ใช้กำหนดว่า reminder รอบไหน "เลยมาแล้วก่อนจอด"
	// บ้าง (เช่น จอดตอน 11:30 รอบ 9:00/10:00/11:00 ไม่ควรเด้งย้อนหลัง)
	ParkedAt time.Time `gorm:"not null" json:"parkedAt"`

	// RemindersSent เก็บ label ของรอบที่ยิง push ไปแล้ว เช่น ["09:00","10:00"]
	// กัน worker ยิงซ้ำตอนมันทำงานรอบถัดไปแล้วเจอ session เดิม
	RemindersSent datatypes.JSON `gorm:"not null;default:'[]'" json:"remindersSent"`

	// EndedAt ไม่ใช่ NULL แปลว่า session นี้จบแล้ว (ผู้ใช้กด Clear หรือเลื่อนรถแล้ว)
	// เก็บไว้เป็นประวัติแทนการลบทิ้ง ต่างจาก IndexedDB ฝั่ง client ที่ clear ทับเลย
	EndedAt *time.Time `json:"endedAt,omitempty"`

	CreatedAt time.Time `gorm:"not null;default:now()" json:"createdAt"`
	UpdatedAt time.Time `gorm:"not null;default:now()" json:"updatedAt"`
}

func (ParkingSession) TableName() string { return "parking_sessions" }
