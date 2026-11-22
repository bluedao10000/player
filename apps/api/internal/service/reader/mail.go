package reader

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/http"
	"net/smtp"
	"net/textproto"
	"os"
	"time"
)

func MailConfigured() bool {
	return os.Getenv("READER_LOCAL_MAIL") == "mailpit" || (os.Getenv("AUTH_RESEND_API_KEY") != "" && os.Getenv("AUTH_EMAIL_FROM") != "")
}

func SendLoginCode(ctx context.Context, email, code string, workbench bool) error {
	subject, plain, rich := loginCodeEmail(code, workbench)
	if os.Getenv("READER_LOCAL_MAIL") == "mailpit" {
		address := os.Getenv("READER_LOCAL_SMTP_ADDR")
		if address == "" {
			address = "mailpit:1025"
		}
		message, err := loginCodeMIME(email, subject, plain, rich)
		if err != nil {
			return err
		}
		return smtp.SendMail(address, nil, "reader@folio.test", []string{email}, message)
	}
	body, err := json.Marshal(map[string]any{"from": os.Getenv("AUTH_EMAIL_FROM"), "to": []string{email}, "subject": subject, "text": plain, "html": rich})
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

func loginCodeEmail(code string, workbench bool) (subject, plain, rich string) {
	title := "登录验证码"
	destination := "Folio 时光登录页"
	if workbench {
		title = "工作台验证码"
		destination = "Folio 时光工作台登录页"
	}
	subject = "Folio 时光 · " + title
	instruction := "请将下方验证码填入" + destination + "。"
	reminder := "验证码 5 分钟内有效。如非本人操作，请忽略此邮件。"
	plain = subject + "\r\n\r\n" + instruction + "\r\n\r\n" + code + "\r\n\r\n" + reminder + "\r\n"
	rich = fmt.Sprintf(`<!doctype html>
<html lang="zh-CN">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"></head>
<body style="margin:0;background:#f3f5f8;color:#17233d;font-family:Arial,'PingFang SC','Microsoft YaHei',sans-serif">
<table role="presentation" width="100%%" cellspacing="0" cellpadding="0" style="background:#f3f5f8">
<tr><td align="center" style="padding:32px 16px">
<table role="presentation" width="100%%" cellspacing="0" cellpadding="0" style="max-width:480px;background:#ffffff;border:1px solid #dce3ee;border-radius:18px">
<tr><td align="center" style="padding:32px 24px">
<p style="margin:0 0 16px;color:#176855;font-size:16px;font-weight:bold">Folio 时光</p>
<h1 style="margin:0 0 12px;font-size:24px;line-height:1.4">%s</h1>
<p style="margin:0;color:#475570;font-size:15px;line-height:1.6">%s</p>
<div style="margin:24px 0;padding:16px 0;background:#edf4f2;border-radius:10px">
<span style="display:inline-block;white-space:nowrap;color:#143b32;font-family:Consolas,Monaco,monospace;font-size:36px;font-weight:bold;line-height:1.2;letter-spacing:3px">%s</span>
</div>
<p style="margin:0;color:#475570;font-size:13px;line-height:1.6">%s</p>
</td></tr></table>
</td></tr></table>
</body></html>`, title, instruction, html.EscapeString(code), reminder)
	return subject, plain, rich
}

func loginCodeMIME(email, subject, plain, rich string) ([]byte, error) {
	var body bytes.Buffer
	parts := multipart.NewWriter(&body)
	for _, format := range []struct{ contentType, content string }{
		{"text/plain", plain},
		{"text/html", rich},
	} {
		part, err := parts.CreatePart(textproto.MIMEHeader{
			"Content-Type":              {format.contentType + "; charset=UTF-8"},
			"Content-Transfer-Encoding": {"quoted-printable"},
		})
		if err != nil {
			return nil, err
		}
		encoded := quotedprintable.NewWriter(part)
		if _, err := encoded.Write([]byte(format.content)); err != nil {
			return nil, err
		}
		if err := encoded.Close(); err != nil {
			return nil, err
		}
	}
	if err := parts.Close(); err != nil {
		return nil, err
	}
	var message bytes.Buffer
	fmt.Fprintf(&message, "From: reader@folio.test\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: multipart/alternative; boundary=%q\r\n\r\n", email, mime.BEncoding.Encode("UTF-8", subject), parts.Boundary())
	message.Write(body.Bytes())
	return message.Bytes(), nil
}
