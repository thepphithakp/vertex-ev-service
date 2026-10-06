package port

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/vertex/ev-service/internal/domain"
)

type ParkingRepository interface {
	// UpsertActive สร้างหรืออัปเดต session ที่กำลัง active ของ user คนนี้
	//
	// ถ้า isDoubleParked เปลี่ยนจาก false เป็น true (เพิ่งเริ่มจอดซ้อนคันใหม่)
	// ต้องล้าง RemindersSent — ป้องกัน session เก่าที่เคยจอดซ้อนคันแล้วเลิก
	// แล้วกลับมาจอดซ้อนคันใหม่ ไม่ให้ reminder ที่เคยส่งไปแล้วครั้งก่อนบล็อกรอบใหม่
	UpsertActive(ctx context.Context, userID string, in ParkingInput) (*domain.ParkingSession, error)

	// EndActive ปิด session ที่ active อยู่ของ user คนนี้ (ถ้ามี) — ไม่ error ถ้าไม่มี
	EndActive(ctx context.Context, userID string) error

	GetActive(ctx context.Context, userID string) (*domain.ParkingSession, error)

	// ListDoubleParkedActive ใช้โดย worker เท่านั้น — session ที่ยัง active และ
	// กำลังจอดซ้อนคันอยู่ทั้งหมดในระบบ ไม่กรองตาม user
	ListDoubleParkedActive(ctx context.Context) ([]domain.ParkingSession, error)

	// MarkRemindersSent บันทึกว่ารอบเหล่านี้ถูกส่งไปแล้ว — ต้องเป็น atomic update
	// กับค่าที่มีอยู่ ไม่ใช่เขียนทับทั้งก้อน เผื่อมีการแก้ field อื่นพร้อมกัน
	MarkRemindersSent(ctx context.Context, sessionID uuid.UUID, labels []string) error
}

type ParkingInput struct {
	Floor          string
	Zone           string
	Notes          string
	LocationType   string
	IsDoubleParked bool
	ParkedAt       time.Time
}

// Notifier ส่งการแจ้งเตือนไปหา user หนึ่งคน — ตัวจริงยิงไปที่ notification-service
type Notifier interface {
	Notify(ctx context.Context, userID, title, body string) error
}

type ParkingUseCase interface {
	SetParking(ctx context.Context, userID string, in ParkingInput) (*domain.ParkingSession, error)
	ClearParking(ctx context.Context, userID string) error
	GetParking(ctx context.Context, userID string) (*domain.ParkingSession, error)
}
