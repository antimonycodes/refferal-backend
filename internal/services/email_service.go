package services

import (
	"bytes"
	"fmt"
	"html/template"
	"net/smtp"

	"github.com/cirvee/referral-backend/internal/config"
)

type EmailService struct {
	cfg *config.SMTPConfig
}

func NewEmailService(cfg *config.SMTPConfig) *EmailService {
	return &EmailService{cfg: cfg}
}

// SendEmail sends an email using SMTP
func (s *EmailService) SendEmail(to, subject, htmlBody string) error {
	if s.cfg.User == "" || s.cfg.Password == "" {
		// Skip sending if SMTP not configured
		return nil
	}

	from := s.cfg.FromEmail
	auth := smtp.PlainAuth("", s.cfg.User, s.cfg.Password, s.cfg.Host)

	// Build email message
	mime := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"
	msg := fmt.Sprintf("From: %s <%s>\r\nTo: %s\r\nSubject: %s\r\n%s\r\n%s",
		s.cfg.FromName, from, to, subject, mime, htmlBody)

	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	if err := smtp.SendMail(addr, auth, from, []string{to}, []byte(msg)); err != nil {
		fmt.Printf("Failed to send email to %s: %v\n", to, err)
		fmt.Printf("=== MOCK EMAIL CONTENT ===\nTo: %s\nSubject: %s\nBody:\n%s\n========================\n", to, subject, htmlBody)
		return nil // Return nil so the request doesn't fail in frontend
	}
	fmt.Printf("Email sent successfully to %s\n", to)
	return nil
}

// SendWelcomeEmail sends a welcome email after registration
func (s *EmailService) SendWelcomeEmail(email, name string) error {
	subject := "Welcome to Cirvee! 🎉"
	body := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #1F2937; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: #6D00E7; color: white; padding: 30px; text-align: center; border-radius: 10px 10px 0 0; }
        .content { background: #EFF4FE; padding: 30px; border-radius: 0 0 10px 10px; }
        .button { display: inline-block; background: #6D00E7; color: white; padding: 12px 30px; text-decoration: none; border-radius: 5px; margin-top: 20px; }
        .footer { text-align: center; margin-top: 20px; color: #808080; font-size: 12px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Welcome to Cirvee!</h1>
        </div>
        <div class="content">
            <h2>Hi %s,</h2>
            <p>Thank you for joining Cirvee! We're excited to have you on board.</p>
            <p>You can now start referring students and earn commissions. Share your unique referral code with friends and colleagues to start earning!</p>
            <p>Login to your dashboard to get your referral code and track your earnings.</p>
            <a href="%s/login" class="button">Go to Dashboard</a>
        </div>
        <div class="footer">
            <p>© 2024 Cirvee. All rights reserved.</p>
        </div>
    </div>
</body>
</html>
`, name, s.cfg.FrontendURL)
	return s.SendEmail(email, subject, body)
}

// SendPasswordResetEmail sends a password reset link
func (s *EmailService) SendPasswordResetEmail(email, resetToken string) error {
	subject := "Reset Your Password - Cirvee"
	resetLink := fmt.Sprintf("%s/reset-password?token=%s", s.cfg.FrontendURL, resetToken)
	body := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #1F2937; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: #6D00E7; color: white; padding: 30px; text-align: center; border-radius: 10px 10px 0 0; }
        .content { background: #EFF4FE; padding: 30px; border-radius: 0 0 10px 10px; }
        .button { display: inline-block; background: #6D00E7; color: white; padding: 12px 30px; text-decoration: none; border-radius: 5px; margin-top: 20px; }
        .footer { text-align: center; margin-top: 20px; color: #808080; font-size: 12px; }
        .warning { background: #FFCA9E; border: 1px solid #ffc107; padding: 15px; border-radius: 5px; margin-top: 20px; color: #1F2937; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Password Reset</h1>
        </div>
        <div class="content">
            <p>You requested to reset your password. Click the button below to set a new password:</p>
            <a href="%s" class="button">Reset Password</a>
            <div class="warning">
                <strong>⚠️ Important:</strong> This link will expire in 1 hour. If you didn't request this, please ignore this email.
            </div>
        </div>
        <div class="footer">
            <p>© 2024 Cirvee. All rights reserved.</p>
        </div>
    </div>
</body>
</html>
`, resetLink)
	return s.SendEmail(email, subject, body)
}

// SendStudentConfirmation sends confirmation email to student
func (s *EmailService) SendStudentConfirmation(email, name, course string) error {
	subject := "Registration Confirmed - Cirvee"
	body := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #1F2937; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: #008000; color: white; padding: 30px; text-align: center; border-radius: 10px 10px 0 0; }
        .content { background: #EFF4FE; padding: 30px; border-radius: 0 0 10px 10px; }
        .course-box { background: white; border: 2px solid #008000; padding: 20px; border-radius: 10px; margin: 20px 0; text-align: center; }
        .footer { text-align: center; margin-top: 20px; color: #808080; font-size: 12px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🎓 Registration Confirmed!</h1>
        </div>
        <div class="content">
            <h2>Hi %s,</h2>
            <p>Thank you for registering with Cirvee! Your application has been received.</p>
            <div class="course-box">
                <h3>Course Selected</h3>
                <p style="font-size: 18px; font-weight: bold; color: #008000;">%s</p>
            </div>
            <p>Our team will be in touch with you shortly with the next steps and payment details.</p>
            <p>If you have any questions, feel free to reach out to us.</p>
        </div>
        <div class="footer">
            <p>© 2024 Cirvee. All rights reserved.</p>
        </div>
    </div>
</body>
</html>
`, name, course)
	return s.SendEmail(email, subject, body)
}

// SendReferralNotification notifies user when someone uses their referral code
func (s *EmailService) SendReferralNotification(email, name, studentName, course string, earnings int64) error {
	subject := "🎉 New Referral - You Earned a Commission!"
	body := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #1F2937; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: #6D00E7; color: white; padding: 30px; text-align: center; border-radius: 10px 10px 0 0; }
        .content { background: #EFF4FE; padding: 30px; border-radius: 0 0 10px 10px; }
        .earnings-box { background: #008000; color: white; padding: 30px; border-radius: 10px; margin: 20px 0; text-align: center; }
        .earnings-amount { font-size: 36px; font-weight: bold; }
        .button { display: inline-block; background: #6D00E7; color: white; padding: 12px 30px; text-decoration: none; border-radius: 5px; margin-top: 20px; }
        .footer { text-align: center; margin-top: 20px; color: #808080; font-size: 12px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🎉 New Referral!</h1>
        </div>
        <div class="content">
            <h2>Congratulations %s!</h2>
            <p>Great news! Someone just used your referral code to register.</p>
            <p><strong>Student:</strong> %s<br><strong>Course:</strong> %s</p>
            <div class="earnings-box">
                <p style="margin: 0;">You earned</p>
                <p class="earnings-amount">₦%d</p>
            </div>
            <p>Keep sharing your referral code to earn more!</p>
            <a href="%s/dashboard" class="button">View Dashboard</a>
        </div>
        <div class="footer">
            <p>© 2024 Cirvee. All rights reserved.</p>
        </div>
    </div>
</body>
</html>
`, name, studentName, course, earnings, s.cfg.FrontendURL)
	return s.SendEmail(email, subject, body)
}

// SendAdminNewStudentAlert notifies admin of new student signup
func (s *EmailService) SendAdminNewStudentAlert(adminEmail, studentName, studentEmail, course, referrer string) error {
	subject := "New Student Registration - Cirvee Admin"
	referrerInfo := "Direct (No referral)"
	if referrer != "" {
		referrerInfo = referrer
	}
	body := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #1F2937; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: #1F2937; color: white; padding: 30px; text-align: center; border-radius: 10px 10px 0 0; }
        .content { background: #EFF4FE; padding: 30px; border-radius: 0 0 10px 10px; }
        .info-table { width: 100%%; border-collapse: collapse; margin: 20px 0; }
        .info-table td { padding: 10px; border-bottom: 1px solid #e5e7eb; }
        .info-table td:first-child { font-weight: bold; width: 40%%; }
        .button { display: inline-block; background: #6D00E7; color: white; padding: 12px 30px; text-decoration: none; border-radius: 5px; margin-top: 20px; }
        .footer { text-align: center; margin-top: 20px; color: #808080; font-size: 12px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>📋 New Student Registration</h1>
        </div>
        <div class="content">
            <p>A new student has registered on the platform:</p>
            <table class="info-table">
                <tr><td>Name</td><td>%s</td></tr>
                <tr><td>Email</td><td>%s</td></tr>
                <tr><td>Course</td><td>%s</td></tr>
                <tr><td>Referred By</td><td>%s</td></tr>
            </table>
            <a href="%s/admin/students" class="button">View in Admin Panel</a>
        </div>
        <div class="footer">
            <p>© 2024 Cirvee. All rights reserved.</p>
        </div>
    </div>
</body>
</html>
`, studentName, studentEmail, course, referrerInfo, s.cfg.FrontendURL)
	return s.SendEmail(adminEmail, subject, body)
}

// Helper to render templates
func renderTemplate(tmpl string, data interface{}) (string, error) {
	t, err := template.New("email").Parse(tmpl)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
