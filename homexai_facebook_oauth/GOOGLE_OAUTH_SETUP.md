# Google OAuth2 配置指南

## 第一步：创建 Google Cloud 项目

1. 访问 [Google Cloud Console](https://console.cloud.google.com/)
2. 如果还没有项目，点击"创建项目"
3. 输入项目名称（例如：HomeX AI）
4. 点击"创建"

## 第二步：启用 Google+ API

1. 在左侧菜单中，选择"API 和服务" > "库"
2. 搜索 "Google+ API"
3. 点击 "Google+ API"
4. 点击"启用"按钮

或者启用更通用的 Google People API：
1. 搜索 "Google People API"
2. 点击并启用

## 第三步：创建 OAuth 2.0 凭据

1. 在左侧菜单中，选择"API 和服务" > "凭据"
2. 点击顶部的"+ 创建凭据"按钮
3. 选择"OAuth 客户端 ID"

### 配置 OAuth 同意屏幕（首次使用需要）

如果是第一次创建 OAuth 客户端 ID，需要先配置 OAuth 同意屏幕：

1. 选择用户类型：
   - **外部**：适用于测试，任何 Google 账户都可以使用
   - **内部**：仅限组织内部用户（需要 Google Workspace）
   
   建议选择"外部"进行测试

2. 填写应用信息：
   - **应用名称**：HomeX AI
   - **用户支持电子邮件**：你的邮箱
   - **应用首页**：http://localhost:8080（测试用）
   - **应用隐私政策链接**：可以暂时留空（生产环境必填）
   - **应用服务条款链接**：可以暂时留空（生产环境必填）
   - **开发者联系信息**：你的邮箱

3. 配置作用域（Scopes）：
   - 点击"添加或移除作用域"
   - 搜索并添加以下作用域：
     - `.../auth/userinfo.email`
     - `.../auth/userinfo.profile`
   - 点击"更新"

4. 测试用户（外部应用需要）：
   - 点击"+ ADD USERS"
   - 添加你的 Google 账户邮箱
   - 只有这些测试用户可以登录（发布前）

5. 点击"保存并继续"，完成配置

### 创建 OAuth 客户端 ID

1. 应用类型：选择"Web 应用"
2. 名称：输入名称（例如：HomeX Web Client）
3. 授权的重定向 URI：
   - 点击"+ 添加 URI"
   - 输入：`http://localhost:8080/auth/google/callback`
   - **重要**：URI 必须完全匹配，包括协议、域名、端口和路径

4. 点击"创建"

## 第四步：获取凭据

创建成功后，会显示一个对话框：

1. **客户端 ID**：类似 `123456789-abcdefg.apps.googleusercontent.com`
2. **客户端密钥**：类似 `GOCSPX-abcd1234efgh5678`

将这两个值复制下来！

## 第五步：配置应用

将获取的凭据填入项目的 `.env` 文件：

```env
GOOGLE_CLIENT_ID=你的客户端ID.apps.googleusercontent.com
GOOGLE_CLIENT_SECRET=你的客户端密钥
GOOGLE_REDIRECT_URL=http://localhost:8080/auth/google/callback
```

## 第六步：测试

1. 启动应用：`./start.sh` 或 `go run main.go`
2. 访问：http://localhost:8080
3. 点击"使用 Google 账户登录"
4. 使用你添加的测试账户登录

## 常见问题

### Q1: redirect_uri_mismatch 错误

**原因**：重定向 URI 不匹配

**解决方案**：
- 检查 Google Cloud Console 中配置的重定向 URI
- 确保与 `.env` 中的 `GOOGLE_REDIRECT_URL` 完全一致
- 注意检查：
  - http vs https
  - 端口号（:8080）
  - 路径（/auth/google/callback）
  - 不要有多余的斜杠

### Q2: Access blocked: This app's request is invalid

**原因**：OAuth 同意屏幕配置不完整

**解决方案**：
- 检查 OAuth 同意屏幕是否配置完成
- 确保添加了必要的作用域
- 如果是外部应用，确保添加了测试用户

### Q3: Access blocked: Authorization Error

**原因**：
- 应用未验证
- 使用了非测试用户账户

**解决方案**：
- 在"OAuth 同意屏幕"中添加测试用户
- 或者将应用设置为内部使用
- 生产环境需要提交应用审核

### Q4: 登录成功但获取不到用户信息

**原因**：API 未启用或作用域配置错误

**解决方案**：
- 确保启用了 Google+ API 或 Google People API
- 检查 OAuth 同意屏幕中是否添加了正确的作用域
- 查看服务器日志了解详细错误

## 生产环境注意事项

### 1. 发布应用

测试完成后，需要发布应用才能让所有用户使用：

1. 在"OAuth 同意屏幕"页面
2. 点击"发布应用"
3. 可能需要提交应用审核（取决于使用的作用域）

### 2. 更新重定向 URI

生产环境使用 HTTPS：

```env
GOOGLE_REDIRECT_URL=https://yourdomain.com/auth/google/callback
```

在 Google Cloud Console 中添加新的重定向 URI：
- `https://yourdomain.com/auth/google/callback`

### 3. 域名验证

某些情况下需要验证域名所有权：

1. 在 Google Cloud Console 中
2. 选择"API 和服务" > "凭据"
3. 在"OAuth 2.0 客户端 ID"部分找到你的客户端
4. 点击"域名验证"并按照指示操作

### 4. 安全建议

- 不要在代码中硬编码客户端密钥
- 使用环境变量或密钥管理服务
- 定期轮换客户端密钥
- 启用 2FA 保护 Google Cloud 账户
- 监控 API 使用情况

## 费用说明

Google OAuth2 服务本身是免费的，但请注意：

- 有每日请求配额限制
- 如果超过配额，需要申请提高限制（可能收费）
- 相关 API（如 People API）也有使用限制

查看配额：
1. Google Cloud Console
2. "API 和服务" > "配额"

## 有用的链接

- [Google OAuth2 文档](https://developers.google.com/identity/protocols/oauth2)
- [OAuth 2.0 Playground](https://developers.google.com/oauthplayground/)
- [Google Cloud Console](https://console.cloud.google.com/)
- [Google API 库](https://console.cloud.google.com/apis/library)

## 获取帮助

如遇到问题：

1. 查看服务器日志：`go run main.go`
2. 检查浏览器控制台（F12）
3. 参考 README.md 中的故障排除部分
4. 访问 Google OAuth2 文档
