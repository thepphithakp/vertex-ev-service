package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/vertex/ev-service/internal/domain"
	"github.com/vertex/ev-service/internal/port"
)

const maxFieldLength = 256

var validLocationTypes = map[string]bool{"condo": true, "mall": true}

type ParkingService struct {
	repo port.ParkingRepository
	now  func() time.Time
}

func NewParkingService(repo port.ParkingRepository) *ParkingService {
	return &ParkingService{repo: repo, now: time.Now}
}

func (s *ParkingService) SetParking(ctx context.Context, userID string, in port.ParkingInput) (*domain.ParkingSession, error) {
	if userID == "" {
		return nil, &ValidationError{Field: "userId", Reason: "ต้องไม่ว่าง"}
	}
	if !validLocationTypes[in.LocationType] {
		return nil, &ValidationError{Field: "locationType", Reason: "ต้องเป็น condo หรือ mall"}
	}
	// จอดซ้อนคันมีความหมายเฉพาะคอนโด ตามของเดิมฝั่ง PWA (EvScreen.tsx)
	if in.LocationType != "condo" {
		in.IsDoubleParked = false
	}
	for name, v := range map[string]string{"floor": in.Floor, "zone": in.Zone, "notes": in.Notes} {
		if len(v) > maxFieldLength {
			return nil, &ValidationError{Field: name, Reason: fmt.Sprintf("ยาวเกิน %d ตัวอักษร", maxFieldLength)}
		}
	}
	if in.ParkedAt.IsZero() {
		in.ParkedAt = s.now()
	}

	in.Floor = strings.TrimSpace(in.Floor)
	in.Zone = strings.TrimSpace(in.Zone)
	in.Notes = strings.TrimSpace(in.Notes)

	return s.repo.UpsertActive(ctx, userID, in)
}

func (s *ParkingService) ClearParking(ctx context.Context, userID string) error {
	if userID == "" {
		return &ValidationError{Field: "userId", Reason: "ต้องไม่ว่าง"}
	}
	return s.repo.EndActive(ctx, userID)
}

func (s *ParkingService) GetParking(ctx context.Context, userID string) (*domain.ParkingSession, error) {
	if userID == "" {
		return nil, &ValidationError{Field: "userId", Reason: "ต้องไม่ว่าง"}
	}
	return s.repo.GetActive(ctx, userID)
}

// ValidationError บอกว่าผู้เรียกส่งอะไรมาผิด เพื่อให้ handler ตอบ 400 ได้
type ValidationError struct {
	Field  string
	Reason string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s %s", e.Field, e.Reason)
}
