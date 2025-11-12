# 邮件服务配置指南

## Gmail 配置（推荐）

### 步骤1：启用两步验证

1. 访问 Google 账户：https://myaccount.google.com/
2. 左侧菜单选择 **安全性**
3. 找到 **登录 Google** 部分
4. 点击 **两步验证** 并启用

### 步骤2：生成应用专用密码

1. 返回 **安全性** 页面
2. 找到 **应用专用密码**
3. 如果找不到，搜索"应用专用密码"或访问：
   https://myaccount.google.com/apppasswords
4. 选择应用：**邮件**
5. 选择设备：**其他（自定义名称）**
6. 输入名称：**HomeX AI**
7. 点击 **生成**
8. 复制生成的16位密码（如：`abcd efgh ijkl mnop`）

### 步骤3：配置.env文件

```env
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USERNAME=你的Gmail邮箱@gmail.com
SMTP_PASSWORD=应用专用密码（16位，去掉空格）
SMTP_FROM_NAME=HomeX AI
SMTP_FROM_EMAIL=你的Gmail邮箱@gmail.com
```

## 其他邮箱服务

### Outlook/Hotmail

```env
SMTP_HOST=smtp-mail.outlook.com
SMTP_PORT=587
SMTP_USERNAME=你的邮箱@outlook.com
SMTP_PASSWORD=你的密码
```

### QQ邮箱

1. 登录QQ邮箱网页版
2. 设置 → 账户
3. 开启 **POP3/SMTP服务**
4. 生成授权码

```env
SMTP_HOST=smtp.qq.com
SMTP_PORT=587
SMTP_USERNAME=你的QQ号@qq.com
SMTP_PASSWORD=生成的授权码
```

### 163邮箱

1. 登录163邮箱
2. 设置 → POP3/SMTP/IMAP
3. 开启服务并设置授权码

```env
SMTP_HOST=smtp.163.com
SMTP_PORT=465
SMTP_USERNAME=你的邮箱@163.com
SMTP_PASSWORD=授权码
```

## 测试配置

启动服务后，尝试发送验证码。如果收到邮件，说明配置成功！

## 常见问题

**Q: 535 Authentication failed**
A: 检查用户名和密码是否正确，Gmail需使用应用专用密码

**Q: 连接超时**
A: 检查防火墙设置，确保允许SMTP端口

**Q: TLS错误**
A: 尝试使用端口465（SSL）而不是587（TLS）
