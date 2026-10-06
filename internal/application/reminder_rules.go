package application

import (
	"fmt"
	"time"
)

// กฎจอดซ้อนคัน — พอร์ต 1:1 จาก vertex-pwa/src/lib/parking.ts ซึ่งพอร์ตมาจาก
// iOS อีกที (ดู NotificationManager.scheduleDoubleParkingReminder) เส้นตาย
// คือเวลาคงที่ 13:00 ไม่ใช่การนับถอยหลังจากเวลาที่จอด
const (
	DeadlineHour = 13
	FineBaht     = 1000
)

// ReminderTime คือเวลาคงที่ในหนึ่งวันที่ต้องเตือน — ตรงกับ REMINDER_TIMES ของ PWA
type ReminderTime struct {
	Hour   int
	Minute int
}

// Label คืนรูปแบบ "HH:MM" ใช้เป็น key ใน RemindersSent — ต้องตรงกับที่ worker
// ใช้ค้นหาว่ายิงไปแล้วหรือยัง
func (r ReminderTime) Label() string {
	return fmt.Sprintf("%02d:%02d", r.Hour, r.Minute)
}

var ReminderTimes = []ReminderTime{
	{9, 0}, {10, 0}, {11, 0}, {12, 0}, {12, 45},
}

// bangkok ใช้คำนวณเวลาเตือนเสมอ ไม่พึ่ง timezone ของเครื่องที่รัน container
// เพราะ pod อาจรันด้วย TZ=UTC และกฎนี้ผูกกับเวลาไทยโดยเฉพาะ
var bangkok = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Bangkok")
	if err != nil {
		// ไม่ควรเกิดขึ้นเลยถ้า image มี tzdata (ดู Dockerfile) แต่ถ้าพลาดจริง
		// fallback เป็น offset คงที่ UTC+7 ดีกว่า panic ทั้ง process
		return time.FixedZone("Asia/Bangkok", 7*60*60)
	}
	return loc
}()

// ReminderBody คืนข้อความแจ้งเตือน — ตรงกับ reminderBody() ของ PWA ทุกตัวอักษร
// เพื่อให้ผู้ใช้เห็นข้อความเดียวกันไม่ว่าจะมาจาก local Notification() เดิม
// (ตอนเปิดแอปค้างไว้) หรือ server push ตัวใหม่นี้
func ReminderBody(r ReminderTime) string {
	if r.Hour == 12 && r.Minute == 45 {
		return fmt.Sprintf(
			"เหลือเวลาอีก 15 นาทีถึง %d:00 น.! รีบไปเลื่อนรถด่วนเพื่อเลี่ยงค่าปรับ %d บาท",
			DeadlineHour, FineBaht)
	}
	return fmt.Sprintf(
		"คุณจอดรถซ้อนคันอยู่ อย่าลืมไปเลื่อนรถเข้าช่องจอดก่อน %d:00 น. นะครับ (ค่าปรับ %d บาท)",
		DeadlineHour, FineBaht)
}

const ReminderTitle = "⚠️ แจ้งเตือนจอดซ้อนคัน!"

// timeToday คืนเวลา r ของ "วันเดียวกับ ref" ตามเขตเวลาไทย
func timeToday(r ReminderTime, ref time.Time) time.Time {
	t := ref.In(bangkok)
	return time.Date(t.Year(), t.Month(), t.Day(), r.Hour, r.Minute, 0, 0, bangkok)
}

// DueReminders คืนรายการ ReminderTime ที่ "ถึงเวลาแล้วและควรยิง" สำหรับ session
// หนึ่งตัว ณ เวลา now
//
// แต่ละรอบยิงได้อย่างมากครั้งเดียวต่อ session ตลอดชีพ (ไม่ใช่เตือนซ้ำทุกวัน
// ถ้าจอดซ้อนคันข้ามวัน) เพราะ label ใน RemindersSent ไม่ผูกกับวันที่ — จงใจ
// เลือกแบบง่ายนี้สำหรับ v1 ดีกว่าเสี่ยงเตือนรัวถ้า session เก่าค้างอยู่หลายวัน
//
// เงื่อนไข:
//  1. จอดวันนี้ตอน 11:30 → ไม่เตือนรอบ 9:00/10:00/11:00 ย้อนหลัง (อยู่ก่อน parkedAt)
//  2. จอดวันนี้ตอน 13:30 (เลยเส้นตายแล้ว) → ไม่เตือนเลยสักรอบของวันนี้
//  3. จอดมาจากเมื่อวาน ยังไม่จบ session → รอบของ "วันนี้" ถือว่าเตือนได้ปกติ
//     (เงื่อนไขข้อ 1-2 มีผลเฉพาะวันที่จอดจริงเท่านั้น)
//
// ⚠️ ถ้า worker หยุดทำงานไปนาน (เช่น pod restart ช่วง 9:00-11:00) แล้วกลับมา
// ทำงานตอน 11:05 จะเห็นว่า 9:00/10:00/11:00 ทั้งสามรอบ "ถึงเวลาแล้วและยังไม่ส่ง"
// พร้อมกัน — ยิงทั้งสามรอบรวดเดียว ถือเป็นพฤติกรรมที่ยอมรับได้สำหรับ v1
// (ดีกว่าไม่เตือนเลย) ไม่ได้ทำ grace window มาจำกัดเพิ่ม
func DueReminders(parkedAt, now time.Time, alreadySent map[string]bool) []ReminderTime {
	nowBkk := now.In(bangkok)
	parkedBkk := parkedAt.In(bangkok)
	parkedToday := nowBkk.Year() == parkedBkk.Year() && nowBkk.YearDay() == parkedBkk.YearDay()

	var due []ReminderTime
	for _, r := range ReminderTimes {
		if alreadySent[r.Label()] {
			continue
		}
		at := timeToday(r, now)
		if at.After(now) {
			continue
		}
		if parkedToday && at.Before(parkedAt) {
			continue
		}
		due = append(due, r)
	}
	return due
}
