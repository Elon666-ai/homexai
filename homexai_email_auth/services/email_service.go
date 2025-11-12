package services

import (
	"fmt"
	"homexai_email_auth/config"

	"gopkg.in/gomail.v2"
)

type EmailService struct {
	config *config.Config
}

func NewEmailService(cfg *config.Config) *EmailService {
	return &EmailService{
		config: cfg,
	}
}

// SendVerificationCode 发送验证码邮件
func (s *EmailService) SendVerificationCode(to, code string) error {
	subject := "HomeX AI - 验证码"
	body := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%); color: white; padding: 30px; text-align: center; border-radius: 10px 10px 0 0; }
        .content { background: #f9f9f9; padding: 30px; border-radius: 0 0 10px 10px; }
        .code { background: #667eea; color: white; font-size: 32px; font-weight: bold; padding: 20px; text-align: center; border-radius: 8px; letter-spacing: 8px; margin: 20px 0; }
        .footer { text-align: center; margin-top: 20px; color: #999; font-size: 12px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🏠 HomeX AI</h1>
            <p>智能家居管理平台</p>
        </div>
        <div class="content">
            <h2>您的验证码</h2>
            <p>您好！</p>
            <p>您正在注册 HomeX AI 账户，请使用以下验证码完成注册：</p>
            <div class="code">%s</div>
            <p>此验证码将在 <strong>10分钟</strong> 后失效。</p>
            <p>如果这不是您本人的操作，请忽略此邮件。</p>
        </div>
        <div class="footer">
            <p>© 2024 HomeX AI. All rights reserved.</p>
        </div>
    </div>
</body>
</html>
	`, code)

	return s.sendEmail(to, subject, body)
}

// SendPasswordResetEmail 发送密码重置邮件
func (s *EmailService) SendPasswordResetEmail(to, resetLink string) error {
	subject := "HomeX AI - 重置密码"
	body := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%); color: white; padding: 30px; text-align: center; border-radius: 10px 10px 0 0; }
        .content { background: #f9f9f9; padding: 30px; border-radius: 0 0 10px 10px; }
        .button { display: inline-block; background: #667eea; color: white; padding: 15px 30px; text-decoration: none; border-radius: 8px; margin: 20px 0; font-weight: bold; }
        .button:hover { background: #5568d3; }
        .footer { text-align: center; margin-top: 20px; color: #999; font-size: 12px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🏠 HomeX AI</h1>
            <p>智能家居管理平台</p>
        </div>
        <div class="content">
            <h2>重置密码</h2>
            <p>您好！</p>
            <p>我们收到了您重置密码的请求。请点击下面的按钮重置您的密码：</p>
            <p style="text-align: center;">
                <a href="%s" class="button">重置密码</a>
            </p>
            <p>或者复制以下链接到浏览器：</p>
            <p style="word-break: break-all; background: #fff; padding: 10px; border-radius: 4px;">%s</p>
            <p>此链接将在 <strong>30分钟</strong> 后失效。</p>
            <p>如果这不是您本人的操作，请忽略此邮件，您的密码不会被更改。</p>
        </div>
        <div class="footer">
            <p>© 2024 HomeX AI. All rights reserved.</p>
        </div>
    </div>
</body>
</html>
	`, resetLink, resetLink)

	return s.sendEmail(to, subject, body)
}

// SendWelcomeEmail 发送欢迎邮件
func (s *EmailService) SendWelcomeEmail(to, name string) error {
	subject := "欢迎加入 HomeX AI！"
	body := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%); color: white; padding: 30px; text-align: center; border-radius: 10px 10px 0 0; }
        .content { background: #f9f9f9; padding: 30px; border-radius: 0 0 10px 10px; }
        .footer { text-align: center; margin-top: 20px; color: #999; font-size: 12px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🏠 HomeX AI</h1>
            <p>智能家居管理平台</p>
        </div>
        <div class="content">
            <h2>欢迎，%s！</h2>
            <p>恭喜您成功注册 HomeX AI 账户！</p>
            <p>现在您可以：</p>
            <ul>
                <li>✅ 管理您的智能家居设备</li>
                <li>✅ 创建自动化场景</li>
                <li>✅ 远程控制家中设备</li>
                <li>✅ 查看能源使用报告</li>
            </ul>
            <p>感谢您选择 HomeX AI，让我们一起打造智能生活！</p>
        </div>
        <div class="footer">
            <p>© 2024 HomeX AI. All rights reserved.</p>
        </div>
    </div>
</body>
</html>
	`, name)

	return s.sendEmail(to, subject, body)
}

// sendEmail 发送邮件的底层方法
func (s *EmailService) sendEmail(to, subject, body string) error {
	m := gomail.NewMessage()
	m.SetHeader("From", m.FormatAddress(s.config.SMTPFromEmail, s.config.SMTPFromName))
	m.SetHeader("To", to)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", body)

	d := gomail.NewDialer(
		s.config.SMTPHost,
		s.config.SMTPPort,
		s.config.SMTPUsername,
		s.config.SMTPPassword,
	)

	if err := d.DialAndSend(m); err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}
