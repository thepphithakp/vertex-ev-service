package model

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"

	"github.com/vertex/ev-service/internal/domain"
)

func TestParkingSessionRow_RoundTrip(t *testing.T) {
	want := domain.ParkingSession{
		ID:             uuid.New(),
		UserID:         "u1",
		Floor:          "B2",
		Zone:           "A",
		Notes:          "ใกล้ลิฟต์",
		LocationType:   "mall",
		IsDoubleParked: true,
		ParkedAt:       time.Now().Truncate(time.Second),
		RemindersSent:  []string{"09:00", "10:00"},
		CreatedAt:      time.Now().Truncate(time.Second),
		UpdatedAt:      time.Now().Truncate(time.Second),
	}

	row := ParkingSessionRowFromDomain(want)
	got, err := row.ToDomain()
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != want.ID || got.UserID != want.UserID || len(got.RemindersSent) != 2 {
		t.Fatalf("round trip ไม่ตรง: %+v", got)
	}
	if got.RemindersSent[0] != "09:00" || got.RemindersSent[1] != "10:00" {
		t.Fatalf("RemindersSent ไม่ตรงลำดับเดิม: %v", got.RemindersSent)
	}
}

// TestParkingSessionRowFromDomain_NilRemindersSentBecomesEmptyArray ยืนยันว่า
// session ใหม่ (RemindersSent เป็น nil) เขียนลง DB เป็น "[]" ไม่ใช่ "null" —
// ตรงกับ default ของ column และพฤติกรรมเดิมก่อน refactor (UpsertActive เคย
// เซ็ต datatypes.JSON([]byte("[]")) ตรงๆ ตอนสร้าง session ใหม่)
func TestParkingSessionRowFromDomain_NilRemindersSentBecomesEmptyArray(t *testing.T) {
	row := ParkingSessionRowFromDomain(domain.ParkingSession{RemindersSent: nil})
	if string(row.RemindersSent) != "[]" {
		t.Fatalf("RemindersSent ของ session ใหม่ต้องเป็น \"[]\" ได้ %q", string(row.RemindersSent))
	}
}

func TestParkingSessionRow_ToDomain_EmptyRawIsNilNotError(t *testing.T) {
	row := ParkingSessionRow{RemindersSent: datatypes.JSON(nil)}
	got, err := row.ToDomain()
	if err != nil {
		t.Fatal(err)
	}
	if len(got.RemindersSent) != 0 {
		t.Fatalf("raw ว่าง ต้องได้ RemindersSent ว่าง ได้ %v", got.RemindersSent)
	}
}

func TestParkingSessionRow_ToDomain_InvalidJSONReturnsError(t *testing.T) {
	row := ParkingSessionRow{RemindersSent: datatypes.JSON([]byte("not json"))}
	if _, err := row.ToDomain(); err == nil {
		t.Fatal("JSON ที่ parse ไม่ออก ต้องคืน error ไม่ใช่เงียบๆ")
	}
}
