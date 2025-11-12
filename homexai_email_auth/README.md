# HomeX AI - 邮箱认证系统

完整的邮箱注册登录系统，支持验证码注册、登录和密码找回功能。

## 🚀 功能特性

- ✅ **邮箱注册** - 验证码验证，安全可靠
- ✅ **用户登录** - 邮箱密码登录
- ✅ **找回密码** - 邮件链接重置密码
- ✅ **JWT认证** - 7天有效期
- ✅ **精美邮件模板** - HTML格式，专业美观
- ✅ **完整前端** - 注册、登录、重置密码等页面
- ✅ **密码加密** - bcrypt哈希存储

## 📋 系统要求

- Go 1.21+
- 邮箱账号（Gmail/Outlook/QQ/163等）

## 🔧 快速开始

### 1. 配置邮箱服务（3分钟）

#### Gmail配置（推荐）

1. 启用Google两步验证
2. 生成应用专用密码：https://myaccount.google.com/apppasswords
3. 复制16位密码

详见：`EMAIL_SETUP.md`

### 2. 配置项目

```bash
cp .env.example .env
nano .env
```

填入：
```env
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USERNAME=你的邮箱@gmail.com
SMTP_PASSWORD=应用专用密码
SMTP_FROM_NAME=HomeX AI
SMTP_FROM_EMAIL=你的邮箱@gmail.com

JWT_SECRET=your-secret-key
```

### 3. 启动服务

```bash
go mod download
go run main.go
```

### 4. 访问测试

- 注册：http://localhost:8080/register
- 登录：http://localhost:8080/login
- 忘记密码：http://localhost:8080/forgot-password

## 📖 API文档

### 发送验证码
```
POST /auth/send-code
Content-Type: application/json

{
  "email": "user@example.com",
  "type": "register"  // 或 "reset_password"
}
```

### 注册
```
POST /auth/register
Content-Type: application/json

{
  "email": "user@example.com",
  "password": "password123",
  "name": "张三",
  "verification_code": "123456"
}
```

### 登录
```
POST /auth/login
Content-Type: application/json

{
  "email": "user@example.com",
  "password": "password123"
}
```

### 请求重置密码
```
POST /auth/request-reset
Content-Type: application/json

{
  "email": "user@example.com"
}
```

### 重置密码
```
POST /auth/reset-password
Content-Type: application/json

{
  "token": "reset_token",
  "new_password": "newpassword123"
}
```

### 获取用户信息
```
GET /api/profile
Authorization: Bearer YOUR_JWT_TOKEN
```

## 📁 项目结构

```
homexai_email_auth/
├── main.go                    # 主程序
├── config/config.go           # 配置加载
├── models/user.go             # 数据模型
├── database/database.go       # 数据库
├── services/
│   ├── email_service.go      # 邮件发送
│   └── auth_service.go       # 认证服务
├── controllers/
│   └── auth_controller.go    # 控制器
├── middleware/auth.go         # 认证中间件
├── routes/routes.go           # 路由
├── static/                    # 前端页面
│   ├── index.html
│   ├── register.html
│   ├── login.html
│   ├── forgot-password.html
│   ├── reset-password.html
│   └── dashboard.html
├── .env.example               # 配置模板
├── EMAIL_SETUP.md            # 邮件配置指南
└── README.md                  # 本文档
```

## 🔒 安全特性

1. **密码加密** - bcrypt哈希
2. **验证码** - 10分钟有效期
3. **重置令牌** - 30分钟有效期
4. **一次性使用** - 验证码和令牌用后失效
5. **JWT认证** - 7天有效期

## 📧 邮件模板

系统包含三种邮件模板：

1. **验证码邮件** - 注册时发送
2. **密码重置邮件** - 包含重置链接
3. **欢迎邮件** - 注册成功后发送

所有邮件均采用HTML格式，美观专业。

## 🧪 测试流程

### 注册测试

1. 访问 http://localhost:8080/register
2. 输入邮箱，点击发送验证码
3. 查收邮件，输入验证码
4. 填写姓名和密码
5. 点击注册
6. 跳转到控制台

### 登录测试

1. 访问 http://localhost:8080/login
2. 输入邮箱和密码
3. 点击登录
4. 跳转到控制台

### 忘记密码测试

1. 访问 http://localhost:8080/forgot-password
2. 输入邮箱
3. 点击发送重置链接
4. 查收邮件，点击链接
5. 设置新密码
6. 返回登录页面

## 💡 生产环境建议

- [ ] 使用专业邮件服务（SendGrid, Mailgun等）
- [ ] 使用PostgreSQL或MySQL
- [ ] 添加速率限制
- [ ] 启用HTTPS
- [ ] 使用强JWT密钥
- [ ] 添加日志监控
- [ ] 实施备份策略

## 📝 License

MIT License
