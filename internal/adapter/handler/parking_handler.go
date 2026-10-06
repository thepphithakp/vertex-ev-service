package handler

import (
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/vertex/ev-service/internal/application"
	"github.com/vertex/ev-service/internal/port"
	"github.com/vertex/ev-service/pkg/middleware"
)

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
	return c.JSON(session)
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
	return c.JSON(session)
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
