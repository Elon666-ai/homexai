package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"time"

	"homexai/internal/config"
)

// EmailService handles email sending operations
type EmailService struct {
	config *config.EmailConfig2
	client *http.Client
}

// NewEmailService creates a new email service
func NewEmailService(cfg *config.EmailConfig2) *EmailService {
	return &EmailService{
		config: cfg,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SendGridEmail represents SendGrid email structure
type SendGridEmail struct {
	Personalizations []Personalization `json:"personalizations"`
	From             EmailAddress      `json:"from"`
	Subject          string            `json:"subject"`
	Content          []Content         `json:"content"`
}

type Personalization struct {
	To      []EmailAddress `json:"to"`
	Subject string         `json:"subject,omitempty"`
}

type EmailAddress struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

type Content struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// SendVerificationCode sends a verification code via email
func (s *EmailService) SendVerificationCode(email, code, codeType string) error {
	if s.config.SendGridAPIKey == "" {
		// Fallback: log to console for development
		fmt.Printf("📧 Verification code for %s (%s): %s\n", email, codeType, code)
		return nil
	}

	subject := s.getSubjectByType(codeType)
	htmlContent := s.generateVerificationHTML(code, codeType)
	textContent := s.generateVerificationText(code, codeType)

	return s.sendEmail(email, subject, htmlContent, textContent)
}

// SendWelcomeEmail sends a welcome email to new users
func (s *EmailService) SendWelcomeEmail(email, fullName string) error {
	if s.config.SendGridAPIKey == "" {
		fmt.Printf("📧 Welcome email would be sent to: %s\n", email)
		return nil
	}

	subject := "Welcome to HomeX!"
	htmlContent := s.generateWelcomeHTML(fullName)
	textContent := s.generateWelcomeText(fullName)

	return s.sendEmail(email, subject, htmlContent, textContent)
}

// SendPasswordResetEmail sends password reset notification
func (s *EmailService) SendPasswordResetEmail(email string) error {
	if s.config.SendGridAPIKey == "" {
		fmt.Printf("📧 Password reset notification would be sent to: %s\n", email)
		return nil
	}

	subject := "Your Password Has Been Reset"
	htmlContent := s.generatePasswordResetHTML()
	textContent := s.generatePasswordResetText()

	return s.sendEmail(email, subject, htmlContent, textContent)
}

// sendEmail sends an email via SendGrid API
func (s *EmailService) sendEmail(toEmail, subject, htmlContent, textContent string) error {
	sgEmail := SendGridEmail{
		Personalizations: []Personalization{
			{
				To: []EmailAddress{
					{Email: toEmail},
				},
			},
		},
		From: EmailAddress{
			Email: s.config.FromEmail,
			Name:  s.config.FromName,
		},
		Subject: subject,
		Content: []Content{
			{Type: "text/plain", Value: textContent},
			{Type: "text/html", Value: htmlContent},
		},
	}

	jsonData, err := json.Marshal(sgEmail)
	if err != nil {
		return fmt.Errorf("failed to marshal email data: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.sendgrid.com/v3/mail/send", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.config.SendGridAPIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("sendgrid API error: status %d", resp.StatusCode)
	}

	return nil
}

// getSubjectByType returns email subject based on code type
func (s *EmailService) getSubjectByType(codeType string) string {
	subjects := map[string]string{
		CodeTypeLogin:         "Your Login Code - HomeX",
		CodeTypeRegister:      "Verify Your Email - HomeX",
		CodeTypeResetPassword: "Reset Your Password - HomeX",
		CodeTypeVerifyEmail:   "Verify Your Email - HomeX",
		CodeTypeVerifyPhone:   "Verify Your Phone - HomeX",
	}

	if subject, ok := subjects[codeType]; ok {
		return subject
	}
	return "Your Verification Code - HomeX"
}

// generateVerificationHTML generates HTML content for verification email
func (s *EmailService) generateVerificationHTML(code, codeType string) string {
	tmpl := template.Must(template.New("verification").Parse(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Verification Code</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; margin: 0; padding: 0; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; padding: 30px; text-align: center; border-radius: 10px 10px 0 0; }
        .content { background: #f9f9f9; padding: 30px; border-radius: 0 0 10px 10px; }
        .code-box { background: white; border: 2px dashed #667eea; border-radius: 8px; padding: 20px; text-align: center; margin: 20px 0; }
        .code { font-size: 32px; font-weight: bold; color: #667eea; letter-spacing: 5px; }
        .footer { text-align: center; padding: 20px; color: #888; font-size: 12px; }
        .warning { background: #fff3cd; border-left: 4px solid #ffc107; padding: 15px; margin: 20px 0; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🏠 HomeX</h1>
            <p>Your Property Management Platform</p>
        </div>
        <div class="content">
            <h2>Your Verification Code</h2>
            <p>Hello,</p>
            <p>You requested a verification code for {{.Action}}. Please use the code below:</p>
            <div class="code-box">
                <div class="code">{{.Code}}</div>
            </div>
            <p>This code will expire in <strong>5 minutes</strong>.</p>
            <div class="warning">
                <strong>⚠️ Security Notice:</strong> Never share this code with anyone. HomeX staff will never ask for your verification code.
            </div>
            <p>If you didn't request this code, please ignore this email or contact support if you have concerns.</p>
        </div>
        <div class="footer">
            <p>© 2024 HomeX. All rights reserved.</p>
            <p>This is an automated email. Please do not reply.</p>
        </div>
    </div>
</body>
</html>
`))

	var buf bytes.Buffer
	data := map[string]string{
		"Code":   code,
		"Action": s.getActionText(codeType),
	}
	tmpl.Execute(&buf, data)
	return buf.String()
}

// generateVerificationText generates plain text content for verification email
func (s *EmailService) generateVerificationText(code, codeType string) string {
	return fmt.Sprintf(`HomeX - Your Verification Code

Hello,

You requested a verification code for %s. Please use the code below:

%s

This code will expire in 5 minutes.

Security Notice: Never share this code with anyone. HomeX staff will never ask for your verification code.

If you didn't request this code, please ignore this email or contact support if you have concerns.

© 2024 HomeX. All rights reserved.
This is an automated email. Please do not reply.
`, s.getActionText(codeType), code)
}

// generateWelcomeHTML generates HTML content for welcome email
func (s *EmailService) generateWelcomeHTML(fullName string) string {
	tmpl := template.Must(template.New("welcome").Parse(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; margin: 0; padding: 0; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; padding: 30px; text-align: center; border-radius: 10px 10px 0 0; }
        .content { background: #f9f9f9; padding: 30px; border-radius: 0 0 10px 10px; }
        .feature { background: white; padding: 15px; margin: 10px 0; border-radius: 8px; border-left: 4px solid #667eea; }
        .cta-button { display: inline-block; background: #667eea; color: white; padding: 12px 30px; text-decoration: none; border-radius: 5px; margin: 20px 0; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🏠 Welcome to HomeX!</h1>
        </div>
        <div class="content">
            <h2>Hi {{.Name}}!</h2>
            <p>Thank you for joining HomeX! We're excited to have you as part of our community.</p>
            <p>With HomeX, you can:</p>
            <div class="feature">✨ <strong>Manage Properties</strong> - Efficiently handle all your property management needs</div>
            <div class="feature">💰 <strong>Track Bills & Payments</strong> - Never miss a payment with our tracking system</div>
            <div class="feature">📝 <strong>Submit Requests</strong> - Easy maintenance and service request management</div>
            <div class="feature">👥 <strong>Connect with Community</strong> - Stay connected with your property community</div>
            <p>Ready to get started?</p>
            <a href="https://homex.ph" class="cta-button">Explore HomeX</a>
            <p>If you have any questions, feel free to reach out to our support team.</p>
        </div>
        <div style="text-align: center; padding: 20px; color: #888; font-size: 12px;">
            <p>© 2024 HomeX. All rights reserved.</p>
        </div>
    </div>
</body>
</html>
`))

	var buf bytes.Buffer
	tmpl.Execute(&buf, map[string]string{"Name": fullName})
	return buf.String()
}

// generateWelcomeText generates plain text content for welcome email
func (s *EmailService) generateWelcomeText(fullName string) string {
	return fmt.Sprintf(`Welcome to HomeX!

Hi %s!

Thank you for joining HomeX! We're excited to have you as part of our community.

With HomeX, you can:
✨ Manage Properties - Efficiently handle all your property management needs
💰 Track Bills & Payments - Never miss a payment with our tracking system
📝 Submit Requests - Easy maintenance and service request management
👥 Connect with Community - Stay connected with your property community

Ready to get started? Visit https://homex.ph

If you have any questions, feel free to reach out to our support team.

© 2024 HomeX. All rights reserved.
`, fullName)
}

// generatePasswordResetHTML generates HTML content for password reset notification
func (s *EmailService) generatePasswordResetHTML() string {
	return `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; margin: 0; padding: 0; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; padding: 30px; text-align: center; border-radius: 10px 10px 0 0; }
        .content { background: #f9f9f9; padding: 30px; border-radius: 0 0 10px 10px; }
        .success-box { background: #d4edda; border-left: 4px solid #28a745; padding: 15px; margin: 20px 0; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🔐 Password Reset Successful</h1>
        </div>
        <div class="content">
            <div class="success-box">
                <p><strong>Your password has been successfully reset.</strong></p>
            </div>
            <p>Your HomeX account password was recently changed. You can now log in with your new password.</p>
            <p>If you didn't make this change, please contact our support team immediately.</p>
        </div>
        <div style="text-align: center; padding: 20px; color: #888; font-size: 12px;">
            <p>© 2024 HomeX. All rights reserved.</p>
        </div>
    </div>
</body>
</html>
`
}

// generatePasswordResetText generates plain text content for password reset notification
func (s *EmailService) generatePasswordResetText() string {
	return `HomeX - Password Reset Successful

Your password has been successfully reset.

Your HomeX account password was recently changed. You can now log in with your new password.

If you didn't make this change, please contact our support team immediately.

© 2024 HomeX. All rights reserved.
`
}

// getActionText returns action text based on code type
func (s *EmailService) getActionText(codeType string) string {
	actions := map[string]string{
		CodeTypeLogin:         "login",
		CodeTypeRegister:      "account registration",
		CodeTypeResetPassword: "password reset",
		CodeTypeVerifyEmail:   "email verification",
		CodeTypeVerifyPhone:   "phone verification",
	}

	if action, ok := actions[codeType]; ok {
		return action
	}
	return "verification"
}

// SendWelcomeEmailWithPassword sends a welcome email with initial password to new tenant
func (s *EmailService) SendWelcomeEmailWithPassword(email, fullName, initialPassword string) error {
	if s.config == nil || s.config.SendGridAPIKey == "" {
		fmt.Printf("📧 Welcome email with password would be sent to: %s (Password: %s)\n", email, initialPassword)
		return nil
	}

	subject := "Welcome to HomeX - Your Account Information"
	htmlContent := s.generateWelcomeWithPasswordHTML(fullName, email, initialPassword)
	textContent := s.generateWelcomeWithPasswordText(fullName, email, initialPassword)

	return s.sendEmail(email, subject, htmlContent, textContent)
}

// generateWelcomeWithPasswordHTML generates HTML content for welcome email with password
func (s *EmailService) generateWelcomeWithPasswordHTML(fullName, email, password string) string {
	tmpl := template.Must(template.New("welcome-password").Parse(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body {
            font-family: Arial, sans-serif;
            line-height: 1.6;
            color: #333;
        }
        .container {
            max-width: 600px;
            margin: 0 auto;
            padding: 20px;
        }
        .header {
            background-color: #4F46E5;
            color: white;
            padding: 20px;
            text-align: center;
            border-radius: 5px 5px 0 0;
        }
        .content {
            background-color: #f9f9f9;
            padding: 30px;
            border-radius: 0 0 5px 5px;
        }
        .credentials {
            background-color: #fff;
            border: 1px solid #e0e0e0;
            border-radius: 5px;
            padding: 15px;
            margin: 20px 0;
        }
        .credential-row {
            display: flex;
            margin: 10px 0;
        }
        .credential-label {
            font-weight: bold;
            width: 120px;
        }
        .credential-value {
            color: #4F46E5;
            font-family: monospace;
        }
        .button {
            display: inline-block;
            background-color: #4F46E5;
            color: white;
            padding: 12px 30px;
            text-decoration: none;
            border-radius: 5px;
            margin: 20px 0;
        }
        .footer {
            margin-top: 30px;
            text-align: center;
            font-size: 12px;
            color: #666;
        }
        .warning {
            background-color: #FEF3C7;
            border-left: 4px solid #F59E0B;
            padding: 15px;
            margin: 20px 0;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Welcome to HomeX</h1>
        </div>
        <div class="content">
            <h2>Hello {{.FullName}},</h2>
            
            <p>Welcome to HomeX property management system! Your tenant account has been created successfully.</p>
            
            <div class="credentials">
                <h3>Your Login Credentials</h3>
                <div class="credential-row">
                    <span class="credential-label">Email:</span>
                    <span class="credential-value">{{.Email}}</span>
                </div>
                <div class="credential-row">
                    <span class="credential-label">Password:</span>
                    <span class="credential-value">{{.Password}}</span>
                </div>
            </div>

            <div class="warning">
                <strong>⚠️ Important Security Notice:</strong>
                <p>For your security, you will be required to change your password upon first login. Please keep your login credentials confidential.</p>
            </div>

            <p>You can now access the system to:</p>
            <ul>
                <li>View your lease information</li>
                <li>Check and pay bills</li>
                <li>Submit maintenance requests</li>
                <li>Register visitors</li>
                <li>Participate in community forums</li>
            </ul>

            <div style="text-align: center;">
                <a href="https://homex.ph/login" class="button">Login to HomeX</a>
            </div>

            <p>If you have any questions or need assistance, please don't hesitate to contact property management.</p>

            <p>Best regards,<br>
            <strong>HomeX Team</strong></p>
        </div>
        <div class="footer">
            <p>This is an automated email. Please do not reply to this message.</p>
            <p>&copy; 2025 HomeX. All rights reserved.</p>
        </div>
    </div>
</body>
</html>
`))

	var buf bytes.Buffer
	tmpl.Execute(&buf, map[string]string{
		"FullName": fullName,
		"Email":    email,
		"Password": password,
	})
	return buf.String()
}

// generateWelcomeWithPasswordText generates plain text content for welcome email with password
func (s *EmailService) generateWelcomeWithPasswordText(fullName, email, password string) string {
	return fmt.Sprintf(`Welcome to HomeX - Your Account Information

Hello %s,

Welcome to HomeX property management system! Your tenant account has been created successfully.

Your Login Credentials:
-----------------------
Email: %s
Password: %s

⚠️ IMPORTANT SECURITY NOTICE:
For your security, you will be required to change your password upon first login. Please keep your login credentials confidential.

You can now access the system to:
- View your lease information
- Check and pay bills
- Submit maintenance requests
- Register visitors
- Participate in community forums

Login at: https://homex.ph/login

If you have any questions or need assistance, please don't hesitate to contact property management.

Best regards,
HomeX Team

---
This is an automated email. Please do not reply to this message.
© 2025 HomeX. All rights reserved.
`, fullName, email, password)
}
