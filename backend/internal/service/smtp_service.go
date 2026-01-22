package service

import (
	"bytes"
	"fmt"
	"html/template"
	"net/smtp"

	"homexai/internal/config"
)

// SmtpService handles email sending operations using Gmail SMTP
type SmtpService struct {
	cfg *config.EmailConf
}

// NewSmtpService creates a new email service
func NewSmtpService(cfg *config.EmailConf) *SmtpService {
	return &SmtpService{
		cfg: cfg,
	}
}

// getFrontendURL returns the frontend URL from config
func (s *SmtpService) getFrontendURL() string {
	url := config.Yaml.Server.FrontendURL
	if url == "" {
		return "https://homex.ph"
	}
	return url
}

// getPropertyURL returns the frontend URL with subdomain for a specific property
func (s *SmtpService) getPropertyURL(subdomain string) string {
	if subdomain == "" {
		return s.getFrontendURL()
	}
	// Generate URL like: https://demo.homex.ph
	return fmt.Sprintf("https://%s.homex.ph", subdomain)
}

// getEmailSignatureHTML returns the standard email signature/footer HTML
func (s *SmtpService) getEmailSignatureHTML() string {
	frontendURL := s.getFrontendURL()
	return fmt.Sprintf(`
        <div class="footer" style="background: #f8f9fa; padding: 30px 20px; text-align: center; border-top: 1px solid #e9ecef;">
            <!-- Login Button -->
            <div style="margin-bottom: 20px;">
                <a href="%s" style="display: inline-block; background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%); color: white; padding: 12px 30px; border-radius: 25px; text-decoration: none; font-weight: bold; font-size: 14px;">
                    🔗 Login to HomeX
                </a>
            </div>
            
            <!-- Divider -->
            <div style="border-top: 1px solid #e0e0e0; margin: 20px 40px;"></div>
            
            <!-- Brand -->
            <div style="margin-bottom: 15px;">
                <span style="font-size: 20px; font-weight: bold; color: #667eea;">🏠 HomeX</span>
                <p style="margin: 5px 0 0 0; color: #666; font-size: 12px;">Your Property Management Platform</p>
            </div>
            
            <!-- Contact Info -->
            <div style="margin-bottom: 15px; color: #666; font-size: 12px;">
                <p style="margin: 5px 0;">📧 Support: <a href="mailto:support@homex.ph" style="color: #667eea; text-decoration: none;">support@homex.ph</a></p>
                <p style="margin: 5px 0;">🌐 Website: <a href="%s" style="color: #667eea; text-decoration: none;">%s</a></p>
            </div>
            
            <!-- Copyright -->
            <div style="color: #999; font-size: 11px; margin-top: 20px;">
                <p style="margin: 5px 0;">© 2024 HomeX. All rights reserved.</p>
                <p style="margin: 5px 0;">This is an automated message. Please do not reply directly to this email.</p>
            </div>
        </div>
    `, frontendURL, frontendURL, frontendURL)
}

// SendVerificationCode sends a verification code via email
func (s *SmtpService) SendVerificationCode(email, code, codeType string) error {
	if s.cfg.SMTPHost == "" || s.cfg.Username == "" {
		// Fallback: log to console for development
		fmt.Printf("📧 Verification code for %s (%s): %s\n", email, codeType, code)
		return nil
	}

	subject := s.getSubjectByType(codeType)
	htmlContent := s.generateVerificationHTML(code, codeType)

	return s.sendEmail(email, subject, htmlContent)
}

// SendWelcomeEmail sends a welcome email to new users
func (s *SmtpService) SendWelcomeEmail(email, fullName string) error {
	if s.cfg.SMTPHost == "" || s.cfg.Username == "" {
		fmt.Printf("📧 Welcome email would be sent to: %s\n", email)
		return nil
	}

	subject := "Welcome to HomeX!"
	htmlContent := s.generateWelcomeHTML(fullName)

	return s.sendEmail(email, subject, htmlContent)
}

// SendPasswordResetEmail sends password reset notification
func (s *SmtpService) SendPasswordResetEmail(email string) error {
	if s.cfg.SMTPHost == "" || s.cfg.Username == "" {
		fmt.Printf("📧 Password reset notification would be sent to: %s\n", email)
		return nil
	}

	subject := "Your Password Has Been Reset"
	htmlContent := s.generatePasswordResetHTML()

	return s.sendEmail(email, subject, htmlContent)
}

// SendInvitationEmail sends an invitation email with verification code
func (s *SmtpService) SendInvitationEmail(email, fullName, code, role string) error {
	if s.cfg.SMTPHost == "" || s.cfg.Username == "" {
		fmt.Printf("📧 Invitation email for %s (%s) [%s]: Code=%s\n", email, fullName, role, code)
		return nil
	}

	subject := "You're Invited to HomeX!"
	htmlContent := s.generateInvitationHTML(fullName, code, role)

	return s.sendEmail(email, subject, htmlContent)
}

// SendStaffCredentialsEmail sends staff account credentials email with temporary password
func (s *SmtpService) SendStaffCredentialsEmail(email, fullName, tempPassword, role, propertyName string) error {
	if s.cfg.SMTPHost == "" || s.cfg.Username == "" {
		fmt.Printf("📧 Staff credentials for %s (%s) [%s] at %s: TempPassword=%s\n", email, fullName, role, propertyName, tempPassword)
		return nil
	}

	subject := "Your HomeX Staff Account Has Been Created"
	htmlContent := s.generateStaffCredentialsHTML(fullName, email, tempPassword, role, propertyName)

	return s.sendEmail(email, subject, htmlContent)
}

// sendEmail sends an email via SMTP - SIMPLIFIED VERSION THAT WORKS WITH YAHOO
// ✅ 修复：采用最简单可靠的方式，兼容Gmail、Yahoo、Outlook等所有主流邮箱
func (s *SmtpService) sendEmail(toEmail, subject, htmlContent string) error {
	// 使用SMTP用户名作为发件人（必须与认证用户一致）
	from := s.cfg.Username

	// 构建简单的邮件消息（与你的成功示例保持一致）
	// ⚠️ 关键：Yahoo需要非常简单的邮件格式
	message := []byte(
		"From: " + from + "\r\n" +
			"To: " + toEmail + "\r\n" +
			"Subject: " + subject + "\r\n" +
			"MIME-Version: 1.0\r\n" +
			"Content-Type: text/html; charset=utf-8\r\n" +
			"\r\n" +
			htmlContent,
	)

	// 设置SMTP认证
	auth := smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.SMTPHost)

	// SMTP服务器地址
	addr := fmt.Sprintf("%s:%d", s.cfg.SMTPHost, s.cfg.SMTPPort)

	// ✅ 使用最简单的smtp.SendMail（与你的成功示例完全一致）
	// 这种方式会自动处理STARTTLS，兼容性最好
	err := smtp.SendMail(addr, auth, from, []string{toEmail}, message)
	if err != nil {
		return fmt.Errorf("failed to send email to %s: %w", toEmail, err)
	}

	fmt.Printf("✅ Email sent successfully to %s\n", toEmail)
	return nil
}

// getSubjectByType returns email subject based on code type
func (s *SmtpService) getSubjectByType(codeType string) string {
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
func (s *SmtpService) generateVerificationHTML(code, codeType string) string {
	frontendURL := s.getFrontendURL()

	tmpl := template.Must(template.New("verification").Parse(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Verification Code</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; margin: 0; padding: 0; background-color: #f4f4f4; }
        .container { max-width: 600px; margin: 20px auto; background: white; border-radius: 10px; overflow: hidden; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
        .header { background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; padding: 30px; text-align: center; }
        .header h1 { margin: 0; font-size: 28px; }
        .header p { margin: 10px 0 0 0; font-size: 14px; opacity: 0.9; }
        .content { padding: 30px; }
        .content h2 { color: #333; margin-top: 0; }
        .code-box { background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); border-radius: 8px; padding: 25px; text-align: center; margin: 25px 0; }
        .code { font-size: 36px; font-weight: bold; color: red; letter-spacing: 8px; text-shadow: 2px 2px 4px rgba(0,0,0,0.2); }
        .info-box { background: #f8f9fa; border-left: 4px solid #667eea; padding: 15px; margin: 20px 0; border-radius: 4px; }
        .warning { background: #fff3cd; border-left: 4px solid #ffc107; padding: 15px; margin: 20px 0; border-radius: 4px; }
        .warning strong { color: #856404; }
        .emoji { font-size: 24px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1><span class="emoji">🏠</span> HomeX</h1>
            <p>Your Property Management Platform</p>
        </div>
        <div class="content">
            <h2>Your Verification Code</h2>
            <p>Hello,</p>
            <p>You requested a verification code for <strong>{{.Action}}</strong>. Please use the code below:</p>
            <div class="code-box">
                <div class="code">{{.Code}}</div>
            </div>
            <div class="info-box">
                <p style="margin: 0;"><strong>⏰ This code will expire in 5 minutes.</strong></p>
            </div>
            <div class="warning">
                <p style="margin: 0;"><strong>⚠️ Security Notice:</strong> Never share this code with anyone. HomeX staff will never ask for your verification code.</p>
            </div>
            <p>If you didn't request this code, please ignore this email or contact support if you have concerns.</p>
        </div>
        
        <!-- Email Signature -->
        <div style="background: #f8f9fa; padding: 30px 20px; text-align: center; border-top: 1px solid #e9ecef;">
            <div style="margin-bottom: 20px;">
                <a href="{{.FrontendURL}}" style="display: inline-block; background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; padding: 12px 30px; border-radius: 25px; text-decoration: none; font-weight: bold; font-size: 14px;">
                    🔗 Go to HomeX
                </a>
            </div>
            <div style="border-top: 1px solid #e0e0e0; margin: 20px 40px;"></div>
            <div style="margin-bottom: 15px;">
                <span style="font-size: 20px; font-weight: bold; color: #667eea;">🏠 HomeX</span>
                <p style="margin: 5px 0 0 0; color: #666; font-size: 12px;">Your Property Management Platform</p>
            </div>
            <div style="margin-bottom: 15px; color: #666; font-size: 12px;">
                <p style="margin: 5px 0;">📧 Support: <a href="mailto:support@homex.ph" style="color: #667eea; text-decoration: none;">support@homex.ph</a></p>
                <p style="margin: 5px 0;">🌐 Website: <a href="{{.FrontendURL}}" style="color: #667eea; text-decoration: none;">{{.FrontendURL}}</a></p>
            </div>
            <div style="color: #999; font-size: 11px; margin-top: 20px;">
                <p style="margin: 5px 0;">© 2024 HomeX. All rights reserved.</p>
                <p style="margin: 5px 0;">This is an automated message. Please do not reply directly to this email.</p>
            </div>
        </div>
    </div>
</body>
</html>
`))

	var buf bytes.Buffer
	data := map[string]string{
		"Code":        code,
		"Action":      s.getActionText(codeType),
		"FrontendURL": frontendURL,
	}
	tmpl.Execute(&buf, data)
	return buf.String()
}

// generateWelcomeHTML generates HTML content for welcome email
func (s *SmtpService) generateWelcomeHTML(fullName string) string {
	frontendURL := s.getFrontendURL()

	tmpl := template.Must(template.New("welcome").Parse(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; margin: 0; padding: 0; background-color: #f4f4f4; }
        .container { max-width: 600px; margin: 20px auto; background: white; border-radius: 10px; overflow: hidden; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
        .header { background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; padding: 40px 30px; text-align: center; }
        .header h1 { margin: 0; font-size: 32px; }
        .content { padding: 30px; }
        .content h2 { color: #333; margin-top: 0; }
        .feature { background: #f8f9fa; padding: 15px; margin: 15px 0; border-radius: 8px; border-left: 4px solid #667eea; }
        .feature strong { color: #667eea; }
        .emoji { font-size: 20px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🏠 Welcome to HomeX!</h1>
        </div>
        <div class="content">
            <h2>Hi {{.Name}}! 👋</h2>
            <p>Thank you for joining <strong>HomeX</strong>! We're excited to have you as part of our community.</p>
            <p>With HomeX, you can:</p>
            <div class="feature"><span class="emoji">✨</span> <strong>Manage Properties</strong> - Efficiently handle all your property management needs</div>
            <div class="feature"><span class="emoji">💰</span> <strong>Track Bills & Payments</strong> - Never miss a payment with our tracking system</div>
            <div class="feature"><span class="emoji">📝</span> <strong>Submit Requests</strong> - Easy maintenance and service request management</div>
            <div class="feature"><span class="emoji">👥</span> <strong>Connect with Community</strong> - Stay connected with your property community</div>
            <p>If you have any questions, feel free to reach out to our support team.</p>
        </div>
        
        <!-- Email Signature -->
        <div style="background: #f8f9fa; padding: 30px 20px; text-align: center; border-top: 1px solid #e9ecef;">
            <div style="margin-bottom: 20px;">
                <a href="{{.FrontendURL}}" style="display: inline-block; background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; padding: 14px 35px; border-radius: 25px; text-decoration: none; font-weight: bold; font-size: 14px; box-shadow: 0 4px 15px rgba(102, 126, 234, 0.4);">
                    🚀 Get Started with HomeX
                </a>
            </div>
            <div style="border-top: 1px solid #e0e0e0; margin: 20px 40px;"></div>
            <div style="margin-bottom: 15px;">
                <span style="font-size: 20px; font-weight: bold; color: #667eea;">🏠 HomeX</span>
                <p style="margin: 5px 0 0 0; color: #666; font-size: 12px;">Your Property Management Platform</p>
            </div>
            <div style="margin-bottom: 15px; color: #666; font-size: 12px;">
                <p style="margin: 5px 0;">📧 Support: <a href="mailto:support@homex.ph" style="color: #667eea; text-decoration: none;">support@homex.ph</a></p>
                <p style="margin: 5px 0;">🌐 Website: <a href="{{.FrontendURL}}" style="color: #667eea; text-decoration: none;">{{.FrontendURL}}</a></p>
            </div>
            <div style="color: #999; font-size: 11px; margin-top: 20px;">
                <p style="margin: 5px 0;">© 2024 HomeX. All rights reserved.</p>
                <p style="margin: 5px 0;">This is an automated message. Please do not reply directly to this email.</p>
            </div>
        </div>
    </div>
</body>
</html>
`))

	var buf bytes.Buffer
	tmpl.Execute(&buf, map[string]string{
		"Name":        fullName,
		"FrontendURL": frontendURL,
	})
	return buf.String()
}

// generatePasswordResetHTML generates HTML content for password reset notification
func (s *SmtpService) generatePasswordResetHTML() string {
	frontendURL := s.getFrontendURL()

	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; margin: 0; padding: 0; background-color: #f4f4f4; }
        .container { max-width: 600px; margin: 20px auto; background: white; border-radius: 10px; overflow: hidden; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
        .header { background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%); color: white; padding: 30px; text-align: center; }
        .header h1 { margin: 0; font-size: 28px; }
        .content { padding: 30px; }
        .success-box { background: #d4edda; border-left: 4px solid #28a745; padding: 20px; margin: 20px 0; border-radius: 4px; }
        .success-box strong { color: #155724; }
        .warning { background: #fff3cd; border-left: 4px solid #ffc107; padding: 15px; margin: 20px 0; border-radius: 4px; }
        .emoji { font-size: 24px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1><span class="emoji">🔐</span> Password Reset Successful</h1>
        </div>
        <div class="content">
            <div class="success-box">
                <p style="margin: 0;"><strong>✅ Your password has been successfully reset.</strong></p>
            </div>
            <p>Your HomeX account password was recently changed. You can now log in with your new password.</p>
            <div class="warning">
                <p style="margin: 0;"><strong>⚠️ If you didn't make this change,</strong> please contact our support team immediately.</p>
            </div>
        </div>
        
        <!-- Email Signature -->
        <div style="background: #f8f9fa; padding: 30px 20px; text-align: center; border-top: 1px solid #e9ecef;">
            <div style="margin-bottom: 20px;">
                <a href="%s" style="display: inline-block; background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%); color: white; padding: 12px 30px; border-radius: 25px; text-decoration: none; font-weight: bold; font-size: 14px;">
                    🔗 Login to HomeX
                </a>
            </div>
            <div style="border-top: 1px solid #e0e0e0; margin: 20px 40px;"></div>
            <div style="margin-bottom: 15px;">
                <span style="font-size: 20px; font-weight: bold; color: #667eea;">🏠 HomeX</span>
                <p style="margin: 5px 0 0 0; color: #666; font-size: 12px;">Your Property Management Platform</p>
            </div>
            <div style="margin-bottom: 15px; color: #666; font-size: 12px;">
                <p style="margin: 5px 0;">📧 Support: <a href="mailto:support@homex.ph" style="color: #667eea; text-decoration: none;">support@homex.ph</a></p>
                <p style="margin: 5px 0;">🌐 Website: <a href="%s" style="color: #667eea; text-decoration: none;">%s</a></p>
            </div>
            <div style="color: #999; font-size: 11px; margin-top: 20px;">
                <p style="margin: 5px 0;">© 2024 HomeX. All rights reserved.</p>
                <p style="margin: 5px 0;">This is an automated message. Please do not reply directly to this email.</p>
            </div>
        </div>
    </div>
</body>
</html>
`, frontendURL, frontendURL, frontendURL)
}

// getActionText returns action text based on code type
func (s *SmtpService) getActionText(codeType string) string {
	actions := map[string]string{
		CodeTypeLogin:         "login",
		CodeTypeRegister:      "account registration",
		CodeTypeResetPassword: "password reset",
		CodeTypeVerifyEmail:   "email verification",
		CodeTypeVerifyPhone:   "phone verification",
		CodeTypeInvitation:    "invitation login",
	}

	if action, ok := actions[codeType]; ok {
		return action
	}
	return "verification"
}

// generateInvitationHTML generates HTML content for invitation email
func (s *SmtpService) generateInvitationHTML(fullName, code, role string) string {
	frontendURL := s.getFrontendURL()

	tmpl := template.Must(template.New("invitation").Parse(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>HomeX Invitation</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; margin: 0; padding: 0; background-color: #f4f4f4; }
        .container { max-width: 600px; margin: 20px auto; background: white; border-radius: 10px; overflow: hidden; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
        .header { background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; padding: 40px 30px; text-align: center; }
        .header h1 { margin: 0; font-size: 32px; }
        .header p { margin: 10px 0 0 0; font-size: 16px; opacity: 0.9; }
        .content { padding: 30px; }
        .content h2 { color: #333; margin-top: 0; }
        .role-badge { display: inline-block; background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; padding: 8px 20px; border-radius: 20px; font-weight: bold; margin: 10px 0; }
        .code-box { background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); border-radius: 8px; padding: 25px; text-align: center; margin: 25px 0; }
        .code { font-size: 42px; font-weight: bold; color: #FFD700; letter-spacing: 10px; text-shadow: 2px 2px 4px rgba(0,0,0,0.3); }
        .info-box { background: #e8f4fd; border-left: 4px solid #667eea; padding: 15px; margin: 20px 0; border-radius: 4px; }
        .steps { background: #f8f9fa; padding: 20px; margin: 20px 0; border-radius: 8px; }
        .steps h3 { margin-top: 0; color: #667eea; }
        .steps ol { margin: 0; padding-left: 20px; }
        .steps li { margin: 10px 0; }
        .warning { background: #fff3cd; border-left: 4px solid #ffc107; padding: 15px; margin: 20px 0; border-radius: 4px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🏠 Welcome to HomeX!</h1>
            <p>Your Property Management Platform</p>
        </div>
        <div class="content">
            <h2>Hello {{.Name}}! 👋</h2>
            <p>You have been invited to join HomeX as:</p>
            <p><span class="role-badge">{{.Role}}</span></p>
            <p>Use the verification code below to complete your login:</p>
            <div class="code-box">
                <div class="code">{{.Code}}</div>
            </div>
            <div class="info-box">
                <p style="margin: 0;"><strong>⏰ This code is valid for 24 hours.</strong></p>
            </div>
            <div class="steps">
                <h3>📋 How to Login:</h3>
                <ol>
                    <li>Go to the HomeX login page</li>
                    <li>Enter your email address</li>
                    <li>Enter the verification code above</li>
                    <li>Set your new password on first login</li>
                </ol>
            </div>
            <div class="warning">
                <p style="margin: 0;"><strong>⚠️ Security Notice:</strong> Never share this code with anyone. HomeX staff will never ask for your verification code.</p>
            </div>
            <p>If you didn't expect this invitation, please ignore this email.</p>
        </div>
        
        <!-- Email Signature -->
        <div style="background: #f8f9fa; padding: 30px 20px; text-align: center; border-top: 1px solid #e9ecef;">
            <div style="margin-bottom: 20px;">
                <a href="{{.FrontendURL}}" style="display: inline-block; background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; padding: 12px 30px; border-radius: 25px; text-decoration: none; font-weight: bold; font-size: 14px;">
                    🔗 Login to HomeX
                </a>
            </div>
            <div style="border-top: 1px solid #e0e0e0; margin: 20px 40px;"></div>
            <div style="margin-bottom: 15px;">
                <span style="font-size: 20px; font-weight: bold; color: #667eea;">🏠 HomeX</span>
                <p style="margin: 5px 0 0 0; color: #666; font-size: 12px;">Your Property Management Platform</p>
            </div>
            <div style="margin-bottom: 15px; color: #666; font-size: 12px;">
                <p style="margin: 5px 0;">📧 Support: <a href="mailto:support@homex.ph" style="color: #667eea; text-decoration: none;">support@homex.ph</a></p>
                <p style="margin: 5px 0;">🌐 Website: <a href="{{.FrontendURL}}" style="color: #667eea; text-decoration: none;">{{.FrontendURL}}</a></p>
            </div>
            <div style="color: #999; font-size: 11px; margin-top: 20px;">
                <p style="margin: 5px 0;">© 2024 HomeX. All rights reserved.</p>
                <p style="margin: 5px 0;">This is an automated message. Please do not reply directly to this email.</p>
            </div>
        </div>
    </div>
</body>
</html>
`))

	var buf bytes.Buffer
	data := map[string]string{
		"Name":        fullName,
		"Code":        code,
		"Role":        role,
		"FrontendURL": frontendURL,
	}
	tmpl.Execute(&buf, data)
	return buf.String()
}

// generateStaffCredentialsHTML generates HTML content for staff credentials email
func (s *SmtpService) generateStaffCredentialsHTML(fullName, email, tempPassword, role, propertyName string) string {
	frontendURL := s.getFrontendURL()

	tmpl := template.Must(template.New("staffcredentials").Parse(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Your HomeX Account</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; margin: 0; padding: 0; background-color: #f4f4f4; }
        .container { max-width: 600px; margin: 20px auto; background: white; border-radius: 10px; overflow: hidden; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
        .header { background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; padding: 40px 30px; text-align: center; }
        .header h1 { margin: 0; font-size: 32px; }
        .header p { margin: 10px 0 0 0; font-size: 16px; opacity: 0.9; }
        .content { padding: 30px; }
        .content h2 { color: #333; margin-top: 0; }
        .role-badge { display: inline-block; background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; padding: 8px 20px; border-radius: 20px; font-weight: bold; margin: 10px 0; }
        .credentials-box { background: #f8f9fa; border-radius: 8px; padding: 20px; margin: 25px 0; border-left: 4px solid #667eea; }
        .credentials-box p { margin: 10px 0; }
        .credentials-box strong { color: #333; }
        .password-box { background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); border-radius: 8px; padding: 20px; text-align: center; margin: 15px 0; }
        .password { font-size: 24px; font-weight: bold; color: #FFD700; letter-spacing: 3px; font-family: monospace; }
        .steps { background: #e8f4fd; padding: 20px; margin: 20px 0; border-radius: 8px; }
        .steps h3 { margin-top: 0; color: #667eea; }
        .steps ol { margin: 0; padding-left: 20px; }
        .steps li { margin: 10px 0; }
        .warning { background: #fff3cd; border-left: 4px solid #ffc107; padding: 15px; margin: 20px 0; border-radius: 4px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🏠 Welcome to HomeX!</h1>
            <p>Your Staff Account Has Been Created</p>
        </div>
        <div class="content">
            <h2>Hello {{.Name}}! 👋</h2>
            <p>You have been added as a staff member at <strong>{{.PropertyName}}</strong>.</p>
            <p>Your role: <span class="role-badge">{{.Role}}</span></p>
            
            <div class="credentials-box">
                <h3 style="margin-top: 0; color: #667eea;">🔐 Your Login Credentials</h3>
                <p><strong>Email:</strong> {{.Email}}</p>
                <p><strong>Temporary Password:</strong></p>
                <div class="password-box">
                    <span class="password">{{.Password}}</span>
                </div>
            </div>
            
            <div class="steps">
                <h3>📋 How to Login:</h3>
                <ol>
                    <li>Go to the HomeX login page</li>
                    <li>Enter your email address: <strong>{{.Email}}</strong></li>
                    <li>Enter the temporary password shown above</li>
                    <li><strong>You will be prompted to change your password on first login</strong></li>
                </ol>
            </div>
            
            <div class="warning">
                <p style="margin: 0;"><strong>⚠️ Important:</strong> For security reasons, please change your password immediately after your first login. Do not share these credentials with anyone.</p>
            </div>
            
            <p>If you have any questions, please contact your property administrator.</p>
        </div>
        
        <!-- Email Signature -->
        <div style="background: #f8f9fa; padding: 30px 20px; text-align: center; border-top: 1px solid #e9ecef;">
            <div style="margin-bottom: 20px;">
                <a href="{{.FrontendURL}}" style="display: inline-block; background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; padding: 12px 30px; border-radius: 25px; text-decoration: none; font-weight: bold; font-size: 14px;">
                    🔗 Login to HomeX
                </a>
            </div>
            <div style="border-top: 1px solid #e0e0e0; margin: 20px 40px;"></div>
            <div style="margin-bottom: 15px;">
                <span style="font-size: 20px; font-weight: bold; color: #667eea;">🏠 HomeX</span>
                <p style="margin: 5px 0 0 0; color: #666; font-size: 12px;">Your Property Management Platform</p>
            </div>
            <div style="margin-bottom: 15px; color: #666; font-size: 12px;">
                <p style="margin: 5px 0;">📧 Support: <a href="mailto:support@homex.ph" style="color: #667eea; text-decoration: none;">support@homex.ph</a></p>
                <p style="margin: 5px 0;">🌐 Website: <a href="{{.FrontendURL}}" style="color: #667eea; text-decoration: none;">{{.FrontendURL}}</a></p>
            </div>
            <div style="color: #999; font-size: 11px; margin-top: 20px;">
                <p style="margin: 5px 0;">© 2024 HomeX. All rights reserved.</p>
                <p style="margin: 5px 0;">This is an automated message. Please do not reply directly to this email.</p>
            </div>
        </div>
    </div>
</body>
</html>
`))

	var buf bytes.Buffer
	data := map[string]string{
		"Name":         fullName,
		"Email":        email,
		"Password":     tempPassword,
		"Role":         s.getRoleDisplayName(role),
		"PropertyName": propertyName,
		"FrontendURL":  frontendURL,
	}
	tmpl.Execute(&buf, data)
	return buf.String()
}

// getRoleDisplayName returns the display name for a role
func (s *SmtpService) getRoleDisplayName(roleCode string) string {
	switch roleCode {
	case "property_account":
		return "Property Accountant"
	case "property_staff":
		return "Property Staff"
	case "property_admin":
		return "Property Admin"
	default:
		return roleCode
	}
}

// SendNotificationEmail sends a general notification email to a user
func (s *SmtpService) SendNotificationEmail(email, subject, content, propertyName, subdomain string) error {
	if s.cfg.SMTPHost == "" || s.cfg.Username == "" {
		fmt.Printf("📧 Notification email would be sent to: %s\n", email)
		return nil
	}

	htmlContent := s.generateNotificationHTML(subject, content, propertyName, subdomain)

	return s.sendEmail(email, subject, htmlContent)
}

// SendAnnouncementEmail sends announcement notification to a user
func (s *SmtpService) SendAnnouncementEmail(email, title, content, priority, propertyName, subdomain string) error {
	if s.cfg.SMTPHost == "" || s.cfg.Username == "" {
		fmt.Printf("📢 Announcement notification would be sent to: %s\n", email)
		return nil
	}

	subject := fmt.Sprintf("[%s] New Announcement: %s", propertyName, title)
	htmlContent := s.generateAnnouncementHTML(title, content, priority, propertyName, subdomain)

	return s.sendEmail(email, subject, htmlContent)
}

// generateNotificationHTML generates HTML content for general notification email
func (s *SmtpService) generateNotificationHTML(subject, content, propertyName, subdomain string) string {
	propertyURL := s.getPropertyURL(subdomain)

	tmpl := template.Must(template.New("notification").Parse(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Notification</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; margin: 0; padding: 0; background-color: #f4f4f4; }
        .container { max-width: 600px; margin: 20px auto; background: white; border-radius: 10px; overflow: hidden; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
        .header { background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; padding: 30px; text-align: center; }
        .header h1 { margin: 0; font-size: 24px; }
        .content { padding: 30px; }
        .notification-content { background: #f8f9fa; border-left: 4px solid #667eea; padding: 20px; border-radius: 4px; white-space: pre-line; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🏠 {{.PropertyName}}</h1>
            <p>Notification</p>
        </div>
        <div class="content">
            <div class="notification-content">{{.Content}}</div>
        </div>
        
        <!-- Email Signature -->
        <div style="background: #f8f9fa; padding: 30px 20px; text-align: center; border-top: 1px solid #e9ecef;">
            <div style="margin-bottom: 20px;">
                <a href="{{.PropertyURL}}" style="display: inline-block; background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; padding: 12px 30px; border-radius: 25px; text-decoration: none; font-weight: bold; font-size: 14px;">
                    🔗 Login to View Details
                </a>
            </div>
            <div style="border-top: 1px solid #e0e0e0; margin: 20px 40px;"></div>
            <div style="margin-bottom: 15px;">
                <span style="font-size: 20px; font-weight: bold; color: #667eea;">🏠 HomeX</span>
                <p style="margin: 5px 0 0 0; color: #666; font-size: 12px;">Your Property Management Platform</p>
            </div>
            <div style="margin-bottom: 15px; color: #666; font-size: 12px;">
                <p style="margin: 5px 0;">📧 Support: <a href="mailto:support@homex.ph" style="color: #667eea; text-decoration: none;">support@homex.ph</a></p>
                <p style="margin: 5px 0;">🌐 Website: <a href="{{.PropertyURL}}" style="color: #667eea; text-decoration: none;">{{.PropertyURL}}</a></p>
            </div>
            <div style="color: #999; font-size: 11px; margin-top: 20px;">
                <p style="margin: 5px 0;">© 2024 HomeX. All rights reserved.</p>
                <p style="margin: 5px 0;">This is an automated message. Please do not reply directly to this email.</p>
            </div>
        </div>
    </div>
</body>
</html>
`))

	var buf bytes.Buffer
	data := map[string]string{
		"Content":      content,
		"PropertyName": propertyName,
		"PropertyURL":  propertyURL,
	}
	tmpl.Execute(&buf, data)
	return buf.String()
}

// generateAnnouncementHTML generates HTML content for announcement email
func (s *SmtpService) generateAnnouncementHTML(title, content, priority, propertyName, subdomain string) string {
	priorityColor := "#6b7280" // gray
	priorityLabel := "Normal"
	switch priority {
	case "high":
		priorityColor = "#dc2626"
		priorityLabel = "High Priority"
	case "urgent":
		priorityColor = "#ea580c"
		priorityLabel = "Urgent"
	}

	// Use property-specific URL with subdomain
	propertyURL := s.getPropertyURL(subdomain)

	tmpl := template.Must(template.New("announcement").Parse(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>New Announcement</title>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; margin: 0; padding: 0; background-color: #f4f4f4; }
        .container { max-width: 600px; margin: 20px auto; background: white; border-radius: 10px; overflow: hidden; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
        .header { background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; padding: 30px; text-align: center; }
        .header h1 { margin: 0; font-size: 24px; }
        .header p { margin: 10px 0 0 0; font-size: 14px; opacity: 0.9; }
        .content { padding: 30px; }
        .priority-badge { display: inline-block; background: {{.PriorityColor}}; color: white; padding: 4px 12px; border-radius: 12px; font-size: 12px; font-weight: bold; margin-bottom: 15px; }
        .announcement-title { font-size: 20px; font-weight: bold; color: #333; margin-bottom: 15px; }
        .announcement-content { background: #f8f9fa; border-left: 4px solid #667eea; padding: 20px; border-radius: 4px; white-space: pre-line; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🏠 {{.PropertyName}}</h1>
            <p>New Announcement</p>
        </div>
        <div class="content">
            <span class="priority-badge">{{.PriorityLabel}}</span>
            <div class="announcement-title">{{.Title}}</div>
            <div class="announcement-content">{{.Content}}</div>
        </div>
        
        <!-- Email Signature -->
        <div style="background: #f8f9fa; padding: 30px 20px; text-align: center; border-top: 1px solid #e9ecef;">
            <!-- Login Button -->
            <div style="margin-bottom: 20px;">
                <a href="{{.PropertyURL}}" style="display: inline-block; background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; padding: 12px 30px; border-radius: 25px; text-decoration: none; font-weight: bold; font-size: 14px;">
                    🔗 Login to View Details
                </a>
            </div>
            
            <!-- Divider -->
            <div style="border-top: 1px solid #e0e0e0; margin: 20px 40px;"></div>
            
            <!-- Brand -->
            <div style="margin-bottom: 15px;">
                <span style="font-size: 20px; font-weight: bold; color: #667eea;">🏠 HomeX</span>
                <p style="margin: 5px 0 0 0; color: #666; font-size: 12px;">Your Property Management Platform</p>
            </div>
            
            <!-- Contact Info -->
            <div style="margin-bottom: 15px; color: #666; font-size: 12px;">
                <p style="margin: 5px 0;">📧 Support: <a href="mailto:support@homex.ph" style="color: #667eea; text-decoration: none;">support@homex.ph</a></p>
                <p style="margin: 5px 0;">🌐 Website: <a href="{{.PropertyURL}}" style="color: #667eea; text-decoration: none;">{{.PropertyURL}}</a></p>
            </div>
            
            <!-- Copyright -->
            <div style="color: #999; font-size: 11px; margin-top: 20px;">
                <p style="margin: 5px 0;">© 2024 HomeX. All rights reserved.</p>
                <p style="margin: 5px 0;">This is an automated message. Please do not reply directly to this email.</p>
            </div>
        </div>
    </div>
</body>
</html>
`))

	var buf bytes.Buffer
	data := map[string]string{
		"Title":         title,
		"Content":       content,
		"PriorityColor": priorityColor,
		"PriorityLabel": priorityLabel,
		"PropertyName":  propertyName,
		"PropertyURL":   propertyURL,
	}
	tmpl.Execute(&buf, data)
	return buf.String()
}
