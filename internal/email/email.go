package email

import (
	"fmt"
	"net/smtp"
	"strings"
	"time"
)

type Config struct {
	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	FromAddress  string
}

type Service struct {
	cfg Config
}

type Message struct {
	To       []string
	Subject  string
	HTMLBody string
}

func NewService(cfg Config) *Service {
	return &Service{cfg: cfg}
}

func (s *Service) IsConfigured() bool {
	return strings.TrimSpace(s.cfg.SMTPHost) != "" &&
		s.cfg.SMTPPort > 0 &&
		strings.TrimSpace(s.cfg.FromAddress) != ""
}

func (s *Service) SendMessage(msg Message) error {
	if !s.IsConfigured() {
		return fmt.Errorf("email service not configured")
	}
	if len(msg.To) == 0 {
		return fmt.Errorf("no recipients specified")
	}

	// Build MIME email with HTML
	header := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nDate: %s\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n",
		s.cfg.FromAddress,
		strings.Join(msg.To, ","),
		msg.Subject,
		time.Now().Format(time.RFC1123Z))

	fullBody := header + msg.HTMLBody

	// SMTP auth if credentials provided
	var auth smtp.Auth
	if strings.TrimSpace(s.cfg.SMTPUsername) != "" && strings.TrimSpace(s.cfg.SMTPPassword) != "" {
		auth = smtp.PlainAuth("", s.cfg.SMTPUsername, s.cfg.SMTPPassword, s.cfg.SMTPHost)
	}

	addr := fmt.Sprintf("%s:%d", s.cfg.SMTPHost, s.cfg.SMTPPort)
	return smtp.SendMail(addr, auth, s.cfg.FromAddress, msg.To, []byte(fullBody))
}

// Alert notification email
func (s *Service) SendAlertEmail(recipientEmail, deviceName, alertType, alertMessage string) error {
	subject := fmt.Sprintf("Alert: %s on %s", alertType, deviceName)
	html := fmt.Sprintf(`
<html><body style="font-family: Arial, sans-serif;">
  <h2 style="color: #d32f2f;">Alert Notification</h2>
  <p><strong>Device:</strong> %s</p>
  <p><strong>Type:</strong> %s</p>
  <p><strong>Message:</strong> %s</p>
  <p><small>Time: %s</small></p>
</body></html>
`, deviceName, alertType, alertMessage, time.Now().Format("2006-01-02 15:04:05"))

	return s.SendMessage(Message{
		To:       []string{recipientEmail},
		Subject:  subject,
		HTMLBody: html,
	})
}

// Ticket notification email
func (s *Service) SendTicketEmail(recipientEmail, ticketID, subject, clientName string) error {
	mailSubject := fmt.Sprintf("New Ticket #%s: %s", ticketID, subject)
	html := fmt.Sprintf(`
<html><body style="font-family: Arial, sans-serif;">
  <h2 style="color: #1976d2;">New Ticket</h2>
  <p><strong>Ticket ID:</strong> %s</p>
  <p><strong>Subject:</strong> %s</p>
  <p><strong>Client:</strong> %s</p>
  <p><small>Time: %s</small></p>
</body></html>
`, ticketID, subject, clientName, time.Now().Format("2006-01-02 15:04:05"))

	return s.SendMessage(Message{
		To:       []string{recipientEmail},
		Subject:  mailSubject,
		HTMLBody: html,
	})
}

// Device offline notification email
func (s *Service) SendDeviceOfflineEmail(recipientEmail, deviceName string, lastSeen time.Time) error {
	subject := fmt.Sprintf("Device Offline: %s", deviceName)
	html := fmt.Sprintf(`
<html><body style="font-family: Arial, sans-serif;">
  <h2 style="color: #d32f2f;">Device Offline</h2>
  <p><strong>Device:</strong> %s</p>
  <p><strong>Last Seen:</strong> %s</p>
  <p style="color: #d32f2f; font-weight: bold;">This device has not reported in recently.</p>
</body></html>
`, deviceName, lastSeen.Format("2006-01-02 15:04:05"))

	return s.SendMessage(Message{
		To:       []string{recipientEmail},
		Subject:  subject,
		HTMLBody: html,
	})
}

// Digest email for multiple alerts
func (s *Service) SendDigestEmail(recipientEmail string, events []map[string]string) error {
	eventHTML := ""
	for _, event := range events {
		eventHTML += fmt.Sprintf(`
  <div style="border-bottom: 1px solid #ddd; padding: 10px 0;">
    <strong>%s:</strong> %s<br/>
    <small>%s</small>
  </div>
`, event["title"], event["message"], event["time"])
	}

	html := fmt.Sprintf(`
<html><body style="font-family: Arial, sans-serif;">
  <h2 style="color: #1976d2;">GoMeshCentral Notification Summary</h2>
  <p>Here are the recent events on your systems:</p>
  %s
  <p style="margin-top: 20px; color: #666; font-size: 12px;">
    To manage your email preferences, log in to GoMeshCentral.
  </p>
</body></html>
`, eventHTML)

	return s.SendMessage(Message{
		To:       []string{recipientEmail},
		Subject:  "GoMeshCentral Summary",
		HTMLBody: html,
	})
}

// Invoice notification email with download link
func (s *Service) SendInvoiceEmail(recipientEmail, companyName, invoiceNumber, clientName string, total float64, downloadURL string) error {
	subject := fmt.Sprintf("Invoice %s from %s", invoiceNumber, companyName)
	formattedTotal := fmt.Sprintf("%.2f", total)
	html := fmt.Sprintf(`
<html><body style="font-family: Arial, sans-serif; color: #333;">
  <h2 style="color: #1976d2; border-bottom: 2px solid #1976d2; padding-bottom: 10px;">Invoice %s</h2>
  
  <p style="margin: 20px 0;"><strong>Dear %s,</strong></p>
  
  <p>Thank you for your business! Please find your invoice details below:</p>
  
  <table style="width: 100%%; border-collapse: collapse; margin: 20px 0; background: #f5f5f5;">
    <tr style="background: #1976d2; color: white;">
      <td style="padding: 12px; font-weight: bold;">Invoice #</td>
      <td style="padding: 12px;">%s</td>
    </tr>
    <tr>
      <td style="padding: 12px; font-weight: bold;">Company</td>
      <td style="padding: 12px;">%s</td>
    </tr>
    <tr style="background: #f9f9f9;">
      <td style="padding: 12px; font-weight: bold;">Amount Due</td>
      <td style="padding: 12px; font-size: 18px; font-weight: bold; color: #1976d2;">$%s</td>
    </tr>
  </table>
  
  <div style="background: #e3f2fd; border-left: 4px solid #1976d2; padding: 15px; margin: 20px 0;">
    <p style="margin: 0;"><strong>Download your invoice:</strong></p>
    <p style="margin: 10px 0;"><a href="%s" style="background-color: #1976d2; color: white; padding: 10px 20px; text-decoration: none; border-radius: 4px; display: inline-block;">Download Invoice PDF</a></p>
  </div>
  
  <p style="margin-top: 30px; color: #666; font-size: 12px;">
    If you have any questions about this invoice, please contact us.
  </p>
  
  <hr style="margin-top: 30px; border: none; border-top: 1px solid #ddd;">
  <p style="color: #999; font-size: 11px; margin: 10px 0;">
    This is an automated message. Please do not reply to this email.
  </p>
</body></html>
`, invoiceNumber, clientName, invoiceNumber, companyName, formattedTotal, downloadURL)

	return s.SendMessage(Message{
		To:       []string{recipientEmail},
		Subject:  subject,
		HTMLBody: html,
	})
}
