# Facebook OAuth 配置指南

## 第一步：创建 Facebook 应用

1. 访问 [Facebook开发者平台](https://developers.facebook.com/)
2. 点击右上角 **我的应用** → **创建应用**
3. 选择应用类型：**消费者** 或 **无**
4. 填写应用信息：
   - **应用名称**：HomeX AI
   - **应用联系邮箱**：你的邮箱
5. 点击 **创建应用**

## 第二步：配置 Facebook 登录

1. 在左侧菜单中，找到 **添加产品**
2. 找到 **Facebook 登录**，点击 **设置**
3. 选择平台：**网站**
4. 填写网站 URL：`http://localhost:8080`
5. 点击 **保存**

## 第三步：配置 OAuth 重定向 URI

1. 左侧菜单：**Facebook 登录** → **设置**
2. 找到 **有效 OAuth 重定向 URI**
3. 添加：`http://localhost:8080/auth/facebook/callback`
4. 点击 **保存更改**

## 第四步：获取应用凭据

1. 左侧菜单：**设置** → **基本**
2. 找到：
   - **应用编号 (App ID)**
   - **应用密钥 (App Secret)**（点击"显示"查看）
3. 复制这两个值

## 第五步：配置应用

将获取的凭据填入项目的 `.env` 文件：

```env
FACEBOOK_APP_ID=你的应用编号
FACEBOOK_APP_SECRET=你的应用密钥
FACEBOOK_REDIRECT_URL=http://localhost:8080/auth/facebook/callback
```

## 第六步：设置应用模式

### 开发模式（测试用）

1. 默认创建的应用处于 **开发模式**
2. 在开发模式下：
   - 只有应用管理员、开发者和测试人员可以登录
   - 需要添加测试用户

### 添加测试用户

1. 左侧菜单：**角色** → **测试用户**
2. 点击 **添加测试用户**
3. 或在 **角色** → **角色** 中添加真实用户为测试人员

### 上线模式（生产环境）

要让所有人都能使用：

1. 完成应用审核要求
2. 左侧菜单：**应用审核**
3. 提交审核所需权限：
   - `email`
   - `public_profile`
4. 点击顶部开关，将应用设为 **上线**

## 第七步：测试

```bash
# 启动服务
./start.sh

# 访问
http://localhost:8080

# 点击 "使用 Facebook 账户登录"
```

---

## 常见问题

### Q: URL被阻止: 重定向 URI

**问题**: 登录时提示 "URL被阻止"

**解决方案**:
1. 检查 Facebook 应用设置中的 **有效 OAuth 重定向 URI**
2. 确保包含：`http://localhost:8080/auth/facebook/callback`
3. 注意：必须完全匹配，包括协议、域名、端口、路径

### Q: 应用未公开

**问题**: 非管理员无法登录

**解决方案**:
- **开发阶段**: 在"角色"中添加测试用户
- **生产环境**: 完成应用审核，将应用设为上线状态

### Q: 无法获取邮箱

**问题**: 用户信息中 email 为空

**原因**: 
- 用户可以选择不分享邮箱
- 应用没有请求 `email` 权限

**解决方案**:
- 确保在 OAuth 作用域中包含 `email`
- 代码已处理：如果没有email，会生成一个临时邮箱

### Q: API版本问题

**问题**: Graph API 调用失败

**解决方案**:
- 使用最新的 Graph API 版本
- 当前代码使用: `https://graph.facebook.com/me`
- 可以指定版本: `https://graph.facebook.com/v18.0/me`

---

## 生产环境配置

### 1. 更新重定向 URI

```env
FACEBOOK_REDIRECT_URL=https://yourdomain.com/auth/facebook/callback
```

在 Facebook 应用设置中添加：
```
https://yourdomain.com/auth/facebook/callback
```

### 2. 配置应用域名

1. 左侧菜单：**设置** → **基本**
2. **应用域名**: 添加你的域名（不含协议）
   - 例如：`yourdomain.com`

### 3. 配置隐私政策 URL

生产环境必须提供：
- **隐私政策 URL**
- **服务条款 URL**（可选）
- **数据删除说明 URL**

### 4. 提交应用审核

如果使用高级权限（如 `user_friends`），需要提交审核：

1. 左侧菜单：**应用审核** → **权限和功能**
2. 请求高级访问权限
3. 提供：
   - 应用截图
   - 使用说明
   - 测试账号

### 5. 启用应用

完成所有配置后：
1. 顶部切换开关
2. 将应用从开发模式切换到上线模式

---

## 安全建议

1. **应用密钥保护**
   - 永远不要在前端代码中暴露 App Secret
   - 使用环境变量存储
   - 添加 `.env` 到 `.gitignore`

2. **HTTPS**
   - 生产环境必须使用 HTTPS
   - HTTP 仅用于本地开发

3. **权限最小化**
   - 只请求必要的权限
   - 当前只需要：`email`, `public_profile`

4. **State 参数**
   - 用于防止 CSRF 攻击
   - 代码已实现

---

## 获取更多帮助

- [Facebook 登录文档](https://developers.facebook.com/docs/facebook-login/)
- [Graph API 参考](https://developers.facebook.com/docs/graph-api/)
- [应用审核指南](https://developers.facebook.com/docs/app-review/)

---

## 测试清单

- [ ] 应用已创建
- [ ] Facebook 登录已配置
- [ ] OAuth 重定向 URI 已添加
- [ ] 应用编号和密钥已复制
- [ ] `.env` 文件已配置
- [ ] 测试用户已添加（开发模式）
- [ ] 服务成功启动
- [ ] 可以正常登录
- [ ] 用户信息正确显示

**完成以上步骤，Facebook 登录即可使用！** 🎉
