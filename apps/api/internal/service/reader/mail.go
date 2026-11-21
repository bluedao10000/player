package reader

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/smtp"
	"os"
	"time"
)

func MailConfigured() bool {
	return os.Getenv("READER_LOCAL_MAIL") == "mailpit" || (os.Getenv("AUTH_RESEND_API_KEY") != "" && os.Getenv("AUTH_EMAIL_FROM") != "")
}

func SendCode(ctx context.Context, email, code string) error {
	subject := "Folio 时光 · 登录验证码"
	message := "你的读者登录验证码为 " + code + "，5 分钟内有效。如非本人操作，请忽略这封邮件。"
	if os.Getenv("READER_LOCAL_MAIL") == "mailpit" {
		return smtp.SendMail("mailpit:1025", nil, "reader@folio.test", []string{email}, []byte("From: reader@folio.test\r\nTo: "+email+"\r\nSubject: "+subject+"\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n"+message))
	}
	body, err := json.Marshal(map[string]any{"from": os.Getenv("AUTH_EMAIL_FROM"), "to": []string{email}, "subject": subject, "text": message})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+os.Getenv("AUTH_RESEND_API_KEY"))
	req.Header.Set("Content-Type", "application/json")
	res, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode/100 != 2 {
		return errors.New("mail delivery failed")
	}
	return nil
}
