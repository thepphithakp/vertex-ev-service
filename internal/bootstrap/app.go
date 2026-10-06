package bootstrap

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"gorm.io/gorm"

	"github.com/vertex/ev-service/internal/adapter/handler"
	"github.com/vertex/ev-service/internal/adapter/notifier"
	"github.com/vertex/ev-service/internal/adapter/repository"
	"github.com/vertex/ev-service/internal/application"
	"github.com/vertex/ev-service/internal/config"
	"github.com/vertex/ev-service/internal/worker"
	"github.com/vertex/ev-service/pkg/middleware"
)

// bodyLimit จำกัดขนาด request — parking session เล็กมาก (floor/zone/notes สั้นๆ)
const bodyLimit = 64 << 10 // 64KB

// NewApp ประกอบ HTTP layer + reminder worker
//
// worker ไม่ได้ผูกกับ HTTP request ใดๆ เลย คืนแยกต่างหากให้ main.go สั่ง
// Run(ctx) ใน goroutine ของตัวเอง — เหมือนที่ pet-service แยก OutboxWorker
// ออกจาก HTTP layer
func NewApp(db *gorm.DB, cfg config.Config, auth middleware.AuthConfig) (*fiber.App, *Health, *worker.ReminderWorker) {
	app := fiber.New(fiber.Config{
		BodyLimit:             bodyLimit,
		ErrorHandler:          middleware.ErrorHandler,
		DisableStartupMessage: true,
	})

	app.Use(recover.New())
	app.Use(middleware.NewRequestID())
	app.Use(middleware.NewMetrics())
	app.Use(middleware.NewAccessLog())
	app.Use(cors.New())

	health := NewHealth(db)
	app.Get("/livez", health.Liveness)
	app.Get("/readyz", health.Readiness)
	app.Get("/health", health.Liveness)
	app.Get("/metrics", middleware.MetricsHandler())

	repo := repository.NewGORMParkingRepository(db)
	svc := application.NewParkingService(repo)
	h := handler.NewParkingHandler(svc)

	h.RegisterRoutes(app.Group("/api/v1/ev/parking", middleware.NewAuth(auth)))

	httpNotifier := notifier.NewHTTPNotifier(cfg.Notification.ServiceURL, cfg.Notification.Token)
	reminderWorker := worker.NewReminderWorker(repo, httpNotifier)

	return app, health, reminderWorker
}
