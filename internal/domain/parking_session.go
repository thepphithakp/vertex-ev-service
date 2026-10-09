package domain

import (
	"time"

	"github.com/google/uuid"
)

// ParkingSession คือที่จอดรถปัจจุบันของผู้ใช้คนหนึ่ง — มีได้แค่หนึ่งแถวที่
// EndedAt เป็น NULL ต่อ user (บังคับด้วย unique partial index ในฐานข้อมูล)
//
// พอร์ตมาจาก ParkingState ของ PWA (src/lib/db.ts) ซึ่งเดิมเก็บใน IndexedDB
// อย่างเดียว — ตารางนี้คือฝั่ง server ที่ทำให้ worker รู้ว่าต้องเตือนใครเมื่อไหร่
//
// 🔴 type นี้เป็น domain ล้วน ไม่มี gorm/json tag — เดิมเป็นทั้ง GORM model
// (มี gorm tag + TableName()) และ API response DTO (มี json tag) ในตัวเดียว
// ทำให้เปลี่ยน schema หรือ wire contract กระทบ domain ตรงๆ ย้าย GORM ไปอยู่
// adapter/repository/model (ParkingSessionRow) และ wire format ไปอยู่
// adapter/handler (parkingSessionResponse) แทน
type ParkingSession struct {
	ID uuid.UUID

	UserID string

	Floor          string
	Zone           string
	Notes          string
	LocationType   string
	IsDoubleParked bool

	// ParkedAt คือเวลาที่เริ่มจอด — ใช้กำหนดว่า reminder รอบไหน "เลยมาแล้วก่อนจอด"
	// บ้าง (เช่น จอดตอน 11:30 รอบ 9:00/10:00/11:00 ไม่ควรเด้งย้อนหลัง)
	ParkedAt time.Time

	// RemindersSent เก็บ label ของรอบที่ยิง push ไปแล้ว เช่น ["09:00","10:00"]
	// กัน worker ยิงซ้ำตอนมันทำงานรอบถัดไปแล้วเจอ session เดิม — เดิมเป็น
	// datatypes.JSON (raw bytes) บังคับให้ทั้ง repository และ worker ต้อง
	// json.Unmarshal เองซ้ำสองที่ ย้ายมาเป็น []string ตรงๆ ให้ adapter ที่
	// เก็บจริง (GORM) เป็นคนแปลง ไม่ใช่ทุกจุดที่ใช้
	RemindersSent []string

	// EndedAt ไม่ใช่ NULL แปลว่า session นี้จบแล้ว (ผู้ใช้กด Clear หรือเลื่อนรถแล้ว)
	// เก็บไว้เป็นประวัติแทนการลบทิ้ง ต่างจาก IndexedDB ฝั่ง client ที่ clear ทับเลย
	EndedAt *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}
