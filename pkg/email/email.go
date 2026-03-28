package email

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"html/template"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	logging "gitlab.com/lifegoeson-libs/pkg-logging"
	"gitlab.com/lifegoeson-libs/pkg-logging/logger"
)

type EmailService struct {
	SMTPHost     string
	SMTPPort     string
	SMTPUsername string
	SMTPPassword string
	FromEmail    string
	FromName     string
}

type EmailConfig struct {
	SMTPHost     string
	SMTPPort     string
	SMTPUsername string
	SMTPPassword string
	FromEmail    string
	FromName     string
}

type EmailData struct {
	Name         string
	Email        string
	ResetLink    string
	VerifyLink   string
	OTPCode      string
	CompanyName  string
	SupportEmail string
}

func NewEmailService(config *EmailConfig, ctx context.Context) *EmailService {
	password := strings.ReplaceAll(config.SMTPPassword, " ", "")
	password = strings.TrimSpace(password)
	logger.FromContext(ctx).Info("Email service initialized",
		logging.String("host", config.SMTPHost),
		logging.String("port", config.SMTPPort),
		logging.String("from_email", strings.TrimSpace(config.FromEmail)),
	)
	return &EmailService{
		SMTPHost:     config.SMTPHost,
		SMTPPort:     config.SMTPPort,
		SMTPUsername: strings.TrimSpace(config.SMTPUsername),
		SMTPPassword: password,
		FromEmail:    strings.TrimSpace(config.FromEmail),
		FromName:     config.FromName,
	}
}

func (e *EmailService) SendEmail(ctx context.Context, to, subject, htmlBody, textBody string) error {
	message := fmt.Sprintf("From: %s <%s>\r\n", e.FromName, e.FromEmail)
	message += fmt.Sprintf("To: %s\r\n", to)
	message += fmt.Sprintf("Subject: %s\r\n", subject)
	message += "MIME-Version: 1.0\r\n"
	message += "Content-Type: multipart/alternative; boundary=\"boundary\"\r\n\r\n"
	if textBody != "" {
		message += "--boundary\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n" + textBody + "\r\n\r\n"
	}
	if htmlBody != "" {
		message += "--boundary\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n" + htmlBody + "\r\n\r\n"
	}
	message += "--boundary--\r\n"

	port, err := strconv.Atoi(e.SMTPPort)
	if err != nil {
		return fmt.Errorf("invalid SMTP port: %w", err)
	}
	var client *smtp.Client
	if port == 465 {
		client, err = e.connectWithTLS(ctx, port)
	} else {
		client, err = e.connectWithSTARTTLS(ctx, port)
	}
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}
	defer client.Close()
	auth := smtp.PlainAuth("", e.SMTPUsername, e.SMTPPassword, e.SMTPHost)
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("SMTP authentication failed: %w", err)
	}
	if err := client.Mail(e.FromEmail); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("failed to set recipient: %w", err)
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to open data connection: %w", err)
	}
	if _, err := writer.Write([]byte(message)); err != nil {
		writer.Close()
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return nil
}

func (e *EmailService) connectWithTLS(ctx context.Context, port int) (*smtp.Client, error) {
	addr := net.JoinHostPort(e.SMTPHost, strconv.Itoa(port))
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 30 * time.Second}, "tcp", addr, &tls.Config{ServerName: e.SMTPHost})
	if err != nil {
		return nil, err
	}
	return smtp.NewClient(conn, e.SMTPHost)
}

func (e *EmailService) connectWithSTARTTLS(ctx context.Context, port int) (*smtp.Client, error) {
	addr := net.JoinHostPort(e.SMTPHost, strconv.Itoa(port))
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	client, err := smtp.NewClient(conn, e.SMTPHost)
	if err != nil {
		conn.Close()
		return nil, err
	}
	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(&tls.Config{ServerName: e.SMTPHost}); err != nil {
			client.Close()
			return nil, err
		}
	}
	return client, nil
}

func (e *EmailService) SendPasswordResetEmail(ctx context.Context, to, name, resetLink string) error {
	subject := "Reset your TechInsight password"
	textBody := fmt.Sprintf("Hi %s,\nClick to reset: %s\nIf you didn't request this, ignore this email.\n", name, resetLink)
	return e.SendEmail(ctx, to, subject, "", textBody)
}

func (e *EmailService) SendEmailVerificationEmail(ctx context.Context, to, name, verifyLink string) error {
	subject := "Verify your email address"
	textBody := fmt.Sprintf("Hi %s,\nVerify: %s\nThis link expires in 24 hours.\n", name, verifyLink)
	return e.SendEmail(ctx, to, subject, "", textBody)
}

func (e *EmailService) SendOTPEmail(ctx context.Context, to, name, otpCode string) error {
	data := EmailData{Name: name, OTPCode: otpCode, CompanyName: "TechInsight", SupportEmail: "support@techinsight.com"}
	subject := "Your TechInsight OTP Code"
	htmlBody, err := e.renderTemplate("otp", data)
	if err != nil {
		return err
	}
	textBody := fmt.Sprintf("Hi %s,\nYour OTP: %s\nExpires in 10 minutes.\n", name, otpCode)
	return e.SendEmail(ctx, to, subject, htmlBody, textBody)
}

func (e *EmailService) renderTemplate(name string, data EmailData) (string, error) {
	tmpl, err := template.New(name).Parse(otpTemplate)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

const otpTemplate = `
<!DOCTYPE html><html><head><meta charset="UTF-8"><title>OTP</title></head><body>
<p>Hi {{.Name}},</p><p>Your OTP: <strong>{{.OTPCode}}</strong></p>
<p>Expires in 10 minutes.</p><p>— {{.CompanyName}}</p></body></html>`
