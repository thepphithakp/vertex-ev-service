package handler

import (
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/vertex/ev-service/internal/application"
	"github.com/vertex/ev-service/internal/domain"
	"github.com/vertex/ev-service/internal/port"
	"github.com/vertex/ev-service/pkg/middleware"
)

// parkingSessionResponse คือรูปแบบ JSON ที่ตอบให้ client — เดิมใช้
// domain.ParkingSession ตรงๆ (มี json tag ติดอยู่กับ domain type เอง) ย้าย
// มาที่นี่เพื่อไม่ให้ domain ต้องรู้จัก wire format ของ API คงชื่อ field
// และรูปแบบเดิมทุกตัวอักษรไว้ ไม่ให้ PWA client ที่ใช้อยู่พัง
type parkingSessionResponse struct {
	ID             uuid.UUID  `json:"id"`
	UserID         string     `json:"userId"`
	Floor          string     `json:"floor"`
	Zone           string     `json:"zone"`
	Notes          string     `json:"notes"`
	LocationType   string     `json:"locationType"`
	IsDoubleParked bool       `json:"isDoubleParked"`
	ParkedAt       time.Time  `json:"parkedAt"`
	RemindersSent  []string   `json:"remindersSent"`
	EndedAt        *time.Time `json:"endedAt,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

func toParkingSessionResponse(s *domain.ParkingSession) parkingSessionResponse {
	return parkingSessionResponse{
		ID:             s.ID,
		UserID:         s.UserID,
		Floor:          s.Floor,
		Zone:           s.Zone,
		Notes:          s.Notes,
		LocationType:   s.LocationType,
		IsDoubleParked: s.IsDoubleParked,
		ParkedAt:       s.ParkedAt,
		RemindersSent:  s.RemindersSent,
		EndedAt:        s.EndedAt,
		CreatedAt:      s.CreatedAt,
		UpdatedAt:      s.UpdatedAt,
	}
}

type ParkingHandler struct {
	useCase port.ParkingUseCase
}

func NewParkingHandler(useCase port.ParkingUseCase) *ParkingHandler {
	return &ParkingHandler{useCase: useCase}
}

func (h *ParkingHandler) RegisterRoutes(r fiber.Router) {
	r.Get("", h.Get)
	r.Put("", h.Set)
	r.Delete("", h.Clear)
}

func (h *ParkingHandler) Get(c *fiber.Ctx) error {
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		return unauthorized(c)
	}
	session, err := h.useCase.GetParking(c.UserContext(), actor.UserID)
	if err != nil {
		return handleUseCaseError(c, err)
	}
	if session == nil {
		return c.Status(fiber.StatusNoContent).Send(nil)
	}
	return c.JSON(toParkingSessionResponse(session))
}

type setParkingRequest struct {
	Floor          string `json:"floor"`
	Zone           string `json:"zone"`
	Notes          string `json:"notes"`
	LocationType   string `json:"locationType"`
	IsDoubleParked bool   `json:"isDoubleParked"`
	ParkedAt       string `json:"parkedAt"` // RFC3339 — ว่างได้ แปลว่าใช้เวลาปัจจุบัน
}

func (h *ParkingHandler) Set(c *fiber.Ctx) error {
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		return unauthorized(c)
	}

	var req setParkingRequest
	if err := c.BodyParser(&req); err != nil {
		return badRequest(c, "อ่าน request body ไม่ได้")
	}

	var parkedAt time.Time
	if req.ParkedAt != "" {
		t, err := time.Parse(time.RFC3339, req.ParkedAt)
		if err != nil {
			return badRequest(c, "parkedAt ต้องเป็นรูปแบบ RFC3339")
		}
		parkedAt = t
	}

	session, err := h.useCase.SetParking(c.UserContext(), actor.UserID, port.ParkingInput{
		Floor:          req.Floor,
		Zone:           req.Zone,
		Notes:          req.Notes,
		LocationType:   req.LocationType,
		IsDoubleParked: req.IsDoubleParked,
		ParkedAt:       parkedAt,
	})
	if err != nil {
		return handleUseCaseError(c, err)
	}
	return c.JSON(toParkingSessionResponse(session))
}

func (h *ParkingHandler) Clear(c *fiber.Ctx) error {
	actor, ok := middleware.ActorFrom(c)
	if !ok {
		return unauthorized(c)
	}
	if err := h.useCase.ClearParking(c.UserContext(), actor.UserID); err != nil {
		return handleUseCaseError(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func handleUseCaseError(c *fiber.Ctx, err error) error {
	var ve *application.ValidationError
	if errors.As(err, &ve) {
		return badRequest(c, ve.Error())
	}
	return fiber.NewError(fiber.StatusInternalServerError, err.Error())
}

func badRequest(c *fiber.Ctx, msg string) error {
	return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
		"error":     msg,
		"requestId": c.Get(middleware.HeaderRequestID),
	})
}

func unauthorized(c *fiber.Ctx) error {
	return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
		"error":     "ยังไม่ได้ยืนยันตัวตน",
		"requestId": c.Get(middleware.HeaderRequestID),
	})
}
