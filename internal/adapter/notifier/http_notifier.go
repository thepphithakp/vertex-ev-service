package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// serviceTokenHeader ต้องตรงกับที่ notification-service อ่าน
const serviceTokenHeader = "X-Service-Token"

// sendTimeout จำกัดเวลาที่ยอมรอ notification-service — พอร์ตค่าจาก
// vertex-pet-service/internal/adapter/event/http_publisher.go
const sendTimeout = 5 * time.Second

// HTTPNotifier ยิง push ไปที่ notification-service ผ่าน cluster-internal DNS
// (ไม่ใช่ fire-and-forget แบบ pet-service → event-service เพราะ worker ต้องรู้
// ผลลัพธ์แน่ชัดก่อนจะ mark ว่า reminder รอบนี้ส่งแล้ว — ส่งไม่สำเร็จต้อง retry
// รอบถัดไปของ worker ไม่ใช่เข้าใจผิดว่าส่งแล้ว)
type HTTPNotifier struct {
	baseURL string
	token   string
	client  *http.Client
}

func NewHTTPNotifier(baseURL, token string) *HTTPNotifier {
	return &HTTPNotifier{
		baseURL: baseURL,
		token:   token,
		client:  &http.Client{Timeout: sendTimeout},
	}
}

func (n *HTTPNotifier) Notify(ctx context.Context, userID, title, body string) error {
	payload, err := json.Marshal(map[string]string{
		"userId": userID,
		"title":  title,
		"body":   body,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		n.baseURL+"/api/v1/internal/push/send", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(serviceTokenHeader, n.token)

	resp, err := n.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("notification-service ตอบ status %d", resp.StatusCode)
	}
	return nil
}
