# HomeX AI - 多OAuth提供商登录系统

支持 Google 和 Facebook 账户登录的完整认证系统，基于 Gin 框架实现。

## 🚀 功能特性

- ✅ **多OAuth提供商支持**
  - Google OAuth2 登录
  - Facebook OAuth2 登录
- ✅ **统一的用户管理**
  - 自动用户注册和更新
  - 支持多种登录方式
  - 用户信息持久化
- ✅ **JWT Token 认证**
  - 7天有效期
  - 包含提供商信息
  - 安全可靠
- ✅ **精美的前端界面**
  - 现代化UI设计
  - 支持多个登录按钮
  - 响应式设计
- ✅ **完整的API接口**
  - RESTful 标准
  - 详细的文档
- ✅ **数据库支持**
  - SQLite 存储
  - 自动迁移

## 📋 系统要求

- Go 1.21 或更高版本
- Google 账号（配置 Google OAuth）
- Facebook 开发者账号（配置 Facebook OAuth）

## 🔧 配置步骤

### 1. 配置 Google OAuth（可选）

详见 [Google OAuth配置指南](GOOGLE_OAUTH_SETUP.md)

简要步骤：
1. 访问 https://console.cloud.google.com/
2. 创建 OAuth 2.0 客户端 ID
3. 获取客户端 ID 和密钥

### 2. 配置 Facebook OAuth（可选）

详见 [Facebook OAuth配置指南](FACEBOOK_OAUTH_SETUP.md)

简要步骤：
1. 访问 https://developers.facebook.com/
2. 创建应用
3. 配置 Facebook 登录
4. 获取应用编号和密钥

### 3. 配置项目

```bash
# 复制配置模板
cp .env.example .env

# 编辑配置文件
nano .env
```

填入你的凭据：

```env
# Google OAuth（如果使用）
GOOGLE_CLIENT_ID=你的Google客户端ID
GOOGLE_CLIENT_SECRET=你的Google客户端密钥
GOOGLE_REDIRECT_URL=http://localhost:8080/auth/google/callback

# Facebook OAuth（如果使用）
FACEBOOK_APP_ID=你的Facebook应用编号
FACEBOOK_APP_SECRET=你的Facebook应用密钥
FACEBOOK_REDIRECT_URL=http://localhost:8080/auth/facebook/callback

# JWT配置
JWT_SECRET=your-super-secret-jwt-key

# 服务器配置
SERVER_PORT=8080
```

**注意**: 至少需要配置一个OAuth提供商（Google或Facebook）。

### 4. 启动服务

```bash
# 下载依赖
go mod download

# 启动服务
go run main.go
```

### 5. 测试

访问 http://localhost:8080

## 📁 项目结构

```
homexai_oauth/
├── main.go                    # 主程序入口
├── go.mod                     # Go模块配置
├── .env.example               # 环境变量模板
├── config/                    # 配置模块
│   └── config.go
├── models/                    # 数据模型
│   └── user.go               # 支持多提供商
├── database/                  # 数据库模块
│   └── database.go
├── services/                  # 服务层
│   └── auth_service.go       # Google & Facebook OAuth
├── controllers/               # 控制器层
│   └── auth_controller.go    # 处理两种OAuth回调
├── middleware/                # 中间件
│   └── auth.go
├── routes/                    # 路由配置
│   └── routes.go
├── utils/                     # 工具函数
│   └── jwt.go
├── static/                    # 静态文件
│   ├── index.html            # 登录页面（两个按钮）
│   └── auth-success.html     # 登录成功页面
└── README.md                  # 本文档
```

## 🔌 API 接口

### Google OAuth

#### 获取 Google 登录链接
```
GET /auth/google/login
```

响应：
```json
{
  "auth_url": "https://accounts.google.com/o/oauth2/auth?...",
  "state": "random_state_string"
}
```

#### Google OAuth 回调
```
GET /auth/google/callback?code=xxx&state=xxx
```

成功后重定向到：`/auth-success.html?token=JWT_TOKEN&provider=google`

### Facebook OAuth

#### 获取 Facebook 登录链接
```
GET /auth/facebook/login
```

响应：
```json
{
  "auth_url": "https://www.facebook.com/v18.0/dialog/oauth?...",
  "state": "random_state_string"
}
```

#### Facebook OAuth 回调
```
GET /auth/facebook/callback?code=xxx&state=xxx
```

成功后重定向到：`/auth-success.html?token=JWT_TOKEN&provider=facebook`

### 通用接口

#### 验证 Token
```
POST /auth/verify
Content-Type: application/json

{
  "token": "your_jwt_token"
}
```

响应：
```json
{
  "valid": true,
  "user_id": 1,
  "email": "user@example.com",
  "provider": "google"
}
```

#### 获取用户信息（需要认证）
```
GET /api/profile
Authorization: Bearer YOUR_JWT_TOKEN
```

响应：
```json
{
  "user": {
    "id": 1,
    "provider": "google",
    "provider_id": "123456789",
    "email": "user@example.com",
    "name": "张三",
    "picture": "https://...",
    "email_verified": true,
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-01T00:00:00Z",
    "last_login_at": "2024-01-01T00:00:00Z"
  }
}
```

## 📊 数据库结构

### Users 表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER | 主键 |
| provider | VARCHAR(20) | OAuth提供商（google/facebook） |
| provider_id | TEXT | 提供商用户ID |
| email | TEXT | 用户邮箱 |
| name | TEXT | 用户姓名 |
| picture | TEXT | 用户头像URL |
| email_verified | BOOLEAN | 邮箱是否验证 |
| created_at | DATETIME | 创建时间 |
| updated_at | DATETIME | 更新时间 |
| last_login_at | DATETIME | 最后登录时间 |

**唯一索引**: (provider, provider_id)

## 🔒 安全特性

1. **OAuth2 标准流程**
   - 授权码模式
   - State 参数防CSRF

2. **JWT Token**
   - HMAC SHA-256 签名
   - 7天有效期
   - 包含提供商信息

3. **数据隔离**
   - 按提供商和ID区分用户
   - 同一邮箱可以有多个账户（不同提供商）

4. **HTTPS支持**
   - 生产环境使用HTTPS
   - 配置说明见文档

## 🧪 测试

### 使用浏览器测试

1. 启动服务：`go run main.go`
2. 访问：http://localhost:8080
3. 点击 Google 或 Facebook 登录按钮
4. 完成授权
5. 查看用户信息

### 使用 curl 测试

```bash
# 测试健康检查
curl http://localhost:8080/health

# 获取 Google 登录链接
curl http://localhost:8080/auth/google/login

# 获取 Facebook 登录链接
curl http://localhost:8080/auth/facebook/login

# 验证 Token
curl -X POST http://localhost:8080/auth/verify \
  -H "Content-Type: application/json" \
  -d '{"token":"YOUR_JWT_TOKEN"}'

# 获取用户信息
curl -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  http://localhost:8080/api/profile
```

## 📖 配置指南

- [Google OAuth 配置](GOOGLE_OAUTH_SETUP.md)
- [Facebook OAuth 配置](FACEBOOK_OAUTH_SETUP.md)

## 🚀 生产环境部署

### 配置清单

- [ ] 使用强随机 JWT_SECRET
- [ ] 配置 HTTPS
- [ ] 更新重定向 URL 为生产域名
- [ ] 使用生产级数据库（PostgreSQL/MySQL）
- [ ] 配置日志收集
- [ ] 实施 API 限流
- [ ] 设置监控和告警
- [ ] Google OAuth 应用审核（如需）
- [ ] Facebook 应用审核并上线

### 环境变量示例（生产）

```env
GOOGLE_CLIENT_ID=your-production-google-client-id
GOOGLE_CLIENT_SECRET=your-production-google-secret
GOOGLE_REDIRECT_URL=https://yourdomain.com/auth/google/callback

FACEBOOK_APP_ID=your-production-facebook-app-id
FACEBOOK_APP_SECRET=your-production-facebook-secret
FACEBOOK_REDIRECT_URL=https://yourdomain.com/auth/facebook/callback

JWT_SECRET=use-a-very-strong-random-secret-here
SERVER_PORT=443
SERVER_HOST=yourdomain.com
FRONTEND_URL=https://yourdomain.com
```

## 🐛 常见问题

### Google相关

详见 Google OAuth 配置文档

### Facebook相关

**Q: URL被阻止**
A: 检查 Facebook 应用设置中的重定向 URI

**Q: 无法获取邮箱**
A: 用户可以选择不分享，代码已处理此情况

**Q: 非管理员无法登录**
A: 开发模式需要添加测试用户，或将应用上线

详见 [Facebook OAuth配置指南](FACEBOOK_OAUTH_SETUP.md)

## 📄 License

MIT License

## 👥 贡献

欢迎提交 Issue 和 Pull Request！
