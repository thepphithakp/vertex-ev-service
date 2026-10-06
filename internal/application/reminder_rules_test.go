package application

import (
	"testing"
	"time"
)

func bkk(y int, mo time.Month, d, h, m int) time.Time {
	loc, _ := time.LoadLocation("Asia/Bangkok")
	return time.Date(y, mo, d, h, m, 0, 0, loc)
}

func labels(rs []ReminderTime) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.Label()
	}
	return out
}

func TestDueReminders_ParkedMidMorning_SkipsEarlierSlots(t *testing.T) {
	parkedAt := bkk(2026, 10, 6, 11, 30)
	now := bkk(2026, 10, 6, 11, 30)
	due := DueReminders(parkedAt, now, map[string]bool{})
	// 9:00/10:00/11:00 อยู่ก่อน parkedAt — ไม่ควรติดมา
	if got := labels(due); len(got) != 0 {
		t.Fatalf("ตอนจอดใหม่ๆ ยังไม่ถึงรอบไหนเลย ได้ %v", got)
	}
}

func TestDueReminders_FiresWhenTimeReached(t *testing.T) {
	parkedAt := bkk(2026, 10, 6, 11, 30)
	now := bkk(2026, 10, 6, 12, 0)
	due := DueReminders(parkedAt, now, map[string]bool{})
	got := labels(due)
	if len(got) != 1 || got[0] != "12:00" {
		t.Fatalf("ตอน 12:00 ควรเตือนรอบ 12:00 พอดี ได้ %v", got)
	}
}

func TestDueReminders_ParkedAfterDeadline_NeverFires(t *testing.T) {
	parkedAt := bkk(2026, 10, 6, 13, 30)
	now := bkk(2026, 10, 6, 14, 0)
	due := DueReminders(parkedAt, now, map[string]bool{})
	if len(due) != 0 {
		t.Fatalf("จอดหลังเส้นตายไปแล้ว ไม่ควรเตือนเลย ได้ %v", labels(due))
	}
}

func TestDueReminders_AlreadySentIsExcluded(t *testing.T) {
	parkedAt := bkk(2026, 10, 6, 8, 0)
	now := bkk(2026, 10, 6, 12, 0)
	due := DueReminders(parkedAt, now, map[string]bool{
		"09:00": true, "10:00": true, "11:00": true,
	})
	got := labels(due)
	if len(got) != 1 || got[0] != "12:00" {
		t.Fatalf("ที่ยังไม่ส่งควรเหลือแค่ 12:00 ได้ %v", got)
	}
}

func TestDueReminders_WorkerDowntimeCatchesUpMultipleSlots(t *testing.T) {
	parkedAt := bkk(2026, 10, 6, 8, 0)
	now := bkk(2026, 10, 6, 11, 5) // worker เพิ่งกลับมาทำงาน พลาด 9:00/10:00/11:00 ไปพร้อมกัน
	due := DueReminders(parkedAt, now, map[string]bool{})
	got := labels(due)
	if len(got) != 3 {
		t.Fatalf("ต้องยิงย้อนหลังสามรอบที่ถึงเวลาแล้ว ได้ %v", got)
	}
}

func TestReminderBody_LastCallHasSpecialCopy(t *testing.T) {
	last := ReminderTime{12, 45}
	if got := ReminderBody(last); got == ReminderBody(ReminderTime{9, 0}) {
		t.Fatalf("ข้อความรอบ 12:45 ต้องต่างจากรอบอื่น ได้เหมือนกัน: %q", got)
	}
}
