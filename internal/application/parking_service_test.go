package application

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/vertex/ev-service/internal/domain"
	"github.com/vertex/ev-service/internal/port"
)

type fakeRepo struct {
	active map[string]*domain.ParkingSession
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{active: map[string]*domain.ParkingSession{}}
}

func (r *fakeRepo) UpsertActive(ctx context.Context, userID string, in port.ParkingInput) (*domain.ParkingSession, error) {
	s := &domain.ParkingSession{
		UserID: userID, Floor: in.Floor, Zone: in.Zone, Notes: in.Notes,
		LocationType: in.LocationType, IsDoubleParked: in.IsDoubleParked, ParkedAt: in.ParkedAt,
	}
	r.active[userID] = s
	return s, nil
}

func (r *fakeRepo) EndActive(ctx context.Context, userID string) error {
	delete(r.active, userID)
	return nil
}

func (r *fakeRepo) GetActive(ctx context.Context, userID string) (*domain.ParkingSession, error) {
	return r.active[userID], nil
}

func (r *fakeRepo) ListDoubleParkedActive(ctx context.Context) ([]domain.ParkingSession, error) {
	return nil, nil
}

func (r *fakeRepo) MarkRemindersSent(ctx context.Context, sessionID uuid.UUID, labels []string) error {
	return nil
}

func TestSetParking_MallClearsDoubleParkedFlag(t *testing.T) {
	repo := newFakeRepo()
	svc := NewParkingService(repo)
	s, err := svc.SetParking(context.Background(), "user-1", port.ParkingInput{
		LocationType: "mall", IsDoubleParked: true,
	})
	if err != nil {
		t.Fatalf("ไม่ควร error: %v", err)
	}
	if s.IsDoubleParked {
		t.Fatal("จอดซ้อนคันมีความหมายเฉพาะคอนโด — mall ต้องถูกบังคับเป็น false")
	}
}

func TestSetParking_RejectsInvalidLocationType(t *testing.T) {
	svc := NewParkingService(newFakeRepo())
	_, err := svc.SetParking(context.Background(), "user-1", port.ParkingInput{
		LocationType: "office",
	})
	if err == nil {
		t.Fatal("ต้อง reject locationType ที่ไม่รู้จัก")
	}
}

func TestClearParking_RemovesActiveSession(t *testing.T) {
	repo := newFakeRepo()
	svc := NewParkingService(repo)
	_, _ = svc.SetParking(context.Background(), "user-1", port.ParkingInput{LocationType: "condo"})
	if err := svc.ClearParking(context.Background(), "user-1"); err != nil {
		t.Fatalf("ไม่ควร error: %v", err)
	}
	got, _ := svc.GetParking(context.Background(), "user-1")
	if got != nil {
		t.Fatal("เคลียร์แล้วต้องไม่มี active session เหลืออยู่")
	}
}
