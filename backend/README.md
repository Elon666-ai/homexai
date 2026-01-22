# HomeX Backend API

Property management system API for HomeX platform.

## 🚀 Features

- ✅ Multi-language support (English, Chinese Simplified/Traditional, Tagalog)
- ✅ Multi-tenancy architecture (property-based isolation)
- ✅ JWT authentication
- ✅ Role-based access control (RBAC)
- ✅ Email/Phone verification
- ✅ Cloudflare Turnstile integration
- ✅ Rate limiting
- ✅ Redis caching
- ✅ Swagger API documentation
- ✅ Parking space management
- ✅ Bill management system

## 📋 Prerequisites

- Go 1.21 or higher
- MySQL 8.0 or higher
- Redis 6.0 or higher
- Docker & Docker Compose (optional)

## 🛠️ Installation

### 1. Clone the repository

```bash
git clone https://github.com/yourusername/homex-backend.git
cd homex-backend
```

### 2. Setup environment variables

```bash
cp .env.example .env
# Edit .env with your configuration
```

### 3. Install dependencies

```bash
go mod download
```

### 4. Start database services (using Docker)

```bash
make docker-up
```

Or manually start MySQL and Redis services.

### 5. Run database migrations

```bash
# Migrate master database
make migrate-master

# Migrate property database (replace 1001 with property ID)
make migrate-property PROPERTY_ID=1001
```

### 6. Seed initial data

```bash
make seed
```

This creates:
- Default roles
- Sample permissions
- Basic translations
- Super admin user (admin@homex.ph / Admin123!@#)

### 7. Run the server

```bash
# Development mode
make dev

# Production mode
make run
```

## 📚 API Documentation

After starting the server, access:
- Swagger UI: http://localhost:8080/swagger/index.html
- API Info: http://localhost:8080/docs

## 🔑 Default Credentials

**Super Admin:**
- Email: admin@homex.ph
- Password: Admin123!@#

⚠️ **IMPORTANT:** Change this password immediately after first login!

## 📁 Project Structure

```
backend/
├── cmd/
│   ├── api/          # Main API server
│   ├── migrate/      # Database migration tool
│   └── seed/         # Database seeding tool
├── internal/
│   ├── config/       # Configuration
│   ├── database/     # Database connections
│   ├── handler/      # HTTP handlers
│   ├── middleware/   # Middleware
│   ├── models/       # Data models
│   ├── repository/   # Data access layer
│   ├── routes/       # Route definitions
│   ├── service/      # Business logic
│   └── utils/        # Utilities
├── .env.example      # Environment template
├── docker-compose.yml
├── Dockerfile
├── Makefile
└── go.mod
```

## 🚦 API Endpoints

### Authentication
- `POST /api/v1/auth/login` - User login
- `POST /api/v1/auth/register` - User registration
- `POST /api/v1/auth/send-code` - Send verification code
- `POST /api/v1/auth/change-password` - Change password
- `POST /api/v1/auth/reset-password` - Reset password
- `POST /api/v1/auth/verify-email` - Verify email
- `POST /api/v1/auth/verify-phone` - Verify phone

### User Management
- `GET /api/v1/user/profile` - Get user profile
- `PUT /api/v1/user/profile` - Update user profile
- `PUT /api/v1/user/language` - Update language preference

### Units (Apartments/Parking)
- `GET /api/v1/units` - List units
- `GET /api/v1/units/:id` - Get unit details
- `POST /api/v1/admin/units` - Create unit (admin)
- `PUT /api/v1/admin/units/:id` - Update unit (admin)
- `DELETE /api/v1/admin/units/:id` - Delete unit (admin)

### Bills
- `GET /api/v1/bills/my` - Get my bills
- `GET /api/v1/bills/:id` - Get bill details
- `POST /api/v1/admin/bills` - Create bill (admin)
- `POST /api/v1/admin/bills/:id/pay` - Mark as paid (admin)

## 🔧 Development

### Run tests

```bash
make test
```

### Generate Swagger docs

```bash
make swagger
```

### Build

```bash
make build
```

### Clean

```bash
make clean
```

## 🐳 Docker

### Build image

```bash
docker build -t homex-backend .
```

### Run with docker-compose

```bash
docker-compose up -d
```

## 🌐 Multi-tenancy

HomeX uses subdomain-based multi-tenancy:

- Main domain: `homex.ph`
- Property 1: `property1.homex.ph`
- Property 2: `property2.homex.ph`

Each property has its own database for data isolation.

## 🔐 Security

- JWT tokens for authentication
- Bcrypt password hashing
- Rate limiting
- CORS configuration
- Cloudflare Turnstile for bot protection
- SQL injection prevention (using GORM)

## 📝 Environment Variables

See `.env.example` for all available configuration options.

Key variables:
- `APP_ENV` - Environment (development/production)
- `DB_MASTER_DSN` - Master database connection
- `REDIS_ADDR` - Redis server address
- `JWT_SECRET` - JWT signing secret
- `TURNSTILE_SECRET` - Cloudflare Turnstile secret

## 🤝 Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📄 License

This project is licensed under the MIT License.

## 👥 Support

For support, email support@homex.ph or join our Slack channel.

## 🗺️ Roadmap

- [ ] OAuth integration (Google, Facebook)
- [ ] Email/SMS notifications
- [ ] Payment gateway integration
- [ ] Visitor management
- [ ] Announcements system
- [ ] File uploads (S3/CloudFlare R2)
- [ ] Mobile app API
- [ ] Analytics dashboard

## 📊 Version

Current version: **1.5.6**

## 🙏 Acknowledgments

- Gin Web Framework
- GORM
- Redis
- Cloudflare Turnstile
