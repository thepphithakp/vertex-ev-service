package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/vertex/ev-service/internal/application"
	"github.com/vertex/ev-service/internal/port"
)

// defaultInterval คือความถี่ที่ worker ตรวจว่ามี reminder ถึงเวลาส่งหรือยัง
//
// รอบเตือนห่างกันอย่างน้อย 45 นาที (12:00 → 12:45 ถี่สุด) การตรวจทุก 1 นาที
// จึงคลาดเคลื่อนได้สูงสุดแค่ ~1 นาทีจากเวลาที่ตั้งใจ ซึ่งยอมรับได้
const defaultInterval = 1 * time.Minute

// ReminderWorker สแกนหา parking session ที่จอดซ้อนคันอยู่เป็นระยะ แล้วยิง
// push ไปตามรอบเวลาที่ถึงกำหนด — พอร์ตแนวคิดมาจาก OutboxWorker ของ
// pet-service (background loop แยกจาก HTTP request path) แต่เรียบง่ายกว่า
// เพราะไม่ต้องมี outbox table แยก ใช้ parking_sessions.reminders_sent ตรงๆ
type ReminderWorker struct {
	repo     port.ParkingRepository
	notifier port.Notifier
	interval time.Duration
	now      func() time.Time
}

func NewReminderWorker(repo port.ParkingRepository, notifier port.Notifier) *ReminderWorker {
	return &ReminderWorker{repo: repo, notifier: notifier, interval: defaultInterval, now: time.Now}
}

// Run บล็อกจนกว่า ctx จะถูกยกเลิก — เรียกใน goroutine แยกจาก main
func (w *ReminderWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	w.tick(ctx) // เช็คทันทีตอน start ไม่ต้องรอ interval แรก

	for {
		select {
		case <-ctx.Done():
			slog.Info("reminder worker หยุดทำงาน")
			return
		case <-ticker.C:
			w.tick(ctx)
		}
	}
}

func (w *ReminderWorker) tick(ctx context.Context) {
	sessions, err := w.repo.ListDoubleParkedActive(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "reminder worker: ดึง session ไม่สำเร็จ", "error", err)
		return
	}

	now := w.now()
	for _, s := range sessions {
		sentSet := make(map[string]bool, len(s.RemindersSent))
		for _, l := range s.RemindersSent {
			sentSet[l] = true
		}

		due := application.DueReminders(s.ParkedAt, now, sentSet)
		if len(due) == 0 {
			continue
		}

		var fired []string
		for _, r := range due {
			err := w.notifier.Notify(ctx, s.UserID, application.ReminderTitle, application.ReminderBody(r))
			if err != nil {
				// ไม่ mark ว่าส่งแล้ว — รอบถัดไปของ worker จะลองใหม่
				slog.ErrorContext(ctx, "reminder worker: ส่ง push ไม่สำเร็จ",
					"session_id", s.ID, "user_id", s.UserID, "label", r.Label(), "error", err)
				continue
			}
			fired = append(fired, r.Label())
		}

		if len(fired) == 0 {
			continue
		}
		if err := w.repo.MarkRemindersSent(ctx, s.ID, fired); err != nil {
			slog.ErrorContext(ctx, "reminder worker: บันทึก reminders_sent ไม่สำเร็จ "+
				"(ส่ง push ไปแล้วแต่ครั้งหน้าอาจส่งซ้ำ)",
				"session_id", s.ID, "labels", fired, "error", err)
			continue
		}
		slog.InfoContext(ctx, "reminder worker: ส่ง push สำเร็จ",
			"session_id", s.ID, "user_id", s.UserID, "labels", fired)
	}
}
