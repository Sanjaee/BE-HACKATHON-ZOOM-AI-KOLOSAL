# BeRealTime Go Backend

Backend server untuk aplikasi BeRealTime menggunakan Go dengan arsitektur clean architecture.

## Struktur Proyek

```
/yourapp
│
├── cmd/
│   └── server/
│       └── main.go
│
├── internal/
│   ├── config/          # load env, config global
│   │   └── config.go
│   │
│   ├── app/             # HTTP handler + routing (Gin)
│   │   ├── router.go
│   │   ├── auth_handler.go
│   │   ├── chat_handler.go
│   │   └── ...
│   │
│   ├── service/         # business logic / usecase
│   │   ├── auth_service.go
│   │   ├── chat_service.go
│   │   └── ...
│   │
│   ├── repository/      # DB access (gorm / raw SQL)
│   │   ├── user_repo.go
│   │   ├── chat_repo.go
│   │   └── ...
│   │
│   ├── model/           # struct model untuk DB
│   │   ├── user.go
│   │   ├── chat.go
│   │   └── ...
│   │
│   ├── websocket/       # ws hub, manager, client
│   │   ├── hub.go
│   │   ├── client.go
│   │   ├── ws_handler.go
│   │   └── ...
│   │
│   └── util/            # helper: jwt, hash, error, response
│       ├── jwt.go
│       ├── hash.go
│       └── response.go
│
├── pkg/                 # library reusable (optional)
│   └── logger/
│       └── logger.go
│
├── go.mod
├── .env
├── Dockerfile
└── docker-compose.yml
```

## Deskripsi Folder

### `cmd/server/`
Entry point aplikasi. Berisi `main.go` yang menginisialisasi dan menjalankan server.

### `internal/config/`
Konfigurasi aplikasi, termasuk loading environment variables dan setup global config.

### `internal/app/`
Layer HTTP handler dan routing menggunakan Gin framework.
- `router.go`: Setup routing dan middleware
- `*_handler.go`: HTTP handlers untuk setiap endpoint

### `internal/service/`
Business logic layer (use case layer). Berisi logika bisnis aplikasi.

### `internal/repository/`
Data access layer. Interface dan implementasi untuk akses database (GORM atau raw SQL).

### `internal/model/`
Struct model untuk database. Definisi struct yang digunakan untuk mapping database.

### `internal/websocket/`
WebSocket implementation untuk real-time communication.
- `hub.go`: WebSocket hub untuk manage connections
- `client.go`: WebSocket client implementation
- `ws_handler.go`: WebSocket handler

### `internal/util/`
Utility functions dan helpers:
- `jwt.go`: JWT token generation dan validation
- `hash.go`: Password hashing utilities
- `response.go`: Standard response formatter

### `pkg/logger/`
Reusable logger library yang bisa digunakan di seluruh aplikasi.

## 🚀 Cara Menjalankan Aplikasi

### Prerequisites
Sebelum menjalankan aplikasi, pastikan Anda telah menginstall:
- **Go 1.21+** - [Download Go](https://golang.org/dl/)
- **Docker & Docker Compose** - [Download Docker](https://www.docker.com/get-started)
- **PostgreSQL** (jika menjalankan secara lokal tanpa Docker)
- **Redis** (jika menjalankan secara lokal tanpa Docker)
- **RabbitMQ** (jika menjalankan secara lokal tanpa Docker)

### 📦 Installation

#### 1. Clone Repository
```bash
git clone <repository-url>
cd monorepo/backend
```

#### 2. Setup Environment Variables
Buat file `.env` di root folder `backend/` dengan konfigurasi berikut:

```bash
# Copy dari .env.example jika ada
cp .env.example .env
```

Atau buat file `.env` baru dan isi dengan variabel yang diperlukan (lihat bagian Environment Variables di bawah).

#### 3. Install Dependencies (Untuk Development Lokal)
```bash
cd be
go mod download
cd ..
```

### 🐳 Menjalankan dengan Docker Compose (Recommended)

Cara termudah untuk menjalankan aplikasi adalah menggunakan Docker Compose. Ini akan menjalankan semua services yang diperlukan (Backend, PostgreSQL, Redis, RabbitMQ, LiveKit) dalam satu perintah.

#### Langkah-langkah:

1. **Pastikan Docker dan Docker Compose sudah terinstall dan berjalan**

2. **Jalankan semua services:**
```bash
docker-compose up -d
```

3. **Cek status services:**
```bash
docker-compose ps
```

4. **Lihat logs:**
```bash
# Semua services
docker-compose logs -f

# Hanya backend
docker-compose logs -f backend
```

5. **Stop services:**
```bash
docker-compose down
```

6. **Stop dan hapus volumes (data akan terhapus):**
```bash
docker-compose down -v
```

#### Services yang akan berjalan:
- **Backend API**: http://localhost:5000
- **PostgreSQL**: localhost:5432
- **Redis**: localhost:6379
- **RabbitMQ Management UI**: http://localhost:15672
  - Username: `yourapp` (default)
  - Password: `password123` (default)
- **LiveKit Server**: localhost:7880

### 💻 Menjalankan Secara Lokal (Tanpa Docker)

Jika Anda ingin menjalankan backend secara lokal tanpa Docker, pastikan semua dependencies (PostgreSQL, Redis, RabbitMQ) sudah terinstall dan berjalan.

#### Langkah-langkah:

1. **Setup Database PostgreSQL:**
```bash
# Buat database
createdb yourapp

# Atau menggunakan psql
psql -U postgres
CREATE DATABASE yourapp;
```

2. **Jalankan Redis:**
```bash
redis-server
```

3. **Jalankan RabbitMQ:**
```bash
# Install RabbitMQ terlebih dahulu, lalu jalankan
rabbitmq-server
```

4. **Update file `.env` dengan konfigurasi lokal:**
```env
POSTGRES_HOST=localhost
REDIS_HOST=localhost
RABBITMQ_HOST=localhost
```

5. **Jalankan aplikasi:**
```bash
cd be
go run cmd/server/main.go
```

Aplikasi akan berjalan di **http://localhost:5000**

### 🔄 Development Mode

Untuk development dengan hot reload, gunakan:

```bash
# Install air (hot reload tool untuk Go)
go install github.com/cosmtrek/air@latest

# Jalankan dengan air
cd be
air
```

Atau gunakan `go run` dengan watch mode:
```bash
cd be
go run cmd/server/main.go
```

## Environment Variables

Buat file `.env` di root folder `backend/` dengan variabel berikut:

```env
# Domain & URLs
DOMAIN=https://your-domain.com
CLIENT_URL=https://your-domain.com
FRONTEND_URL=https://your-domain.com
BACKEND_URL=https://your-domain.com

# Server
PORT=5000
SERVER_HOST=0.0.0.0

# Database
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_USER=your_user
POSTGRES_PASSWORD=your_password
POSTGRES_DB=your_database
POSTGRES_SSLMODE=disable

# JWT & Authentication
JWT_SECRET=your_jwt_secret_key
NEXTAUTH_SECRET=your_nextauth_secret_key
NEXTAUTH_URL=https://your-domain.com

# Google OAuth
GOOGLE_CLIENT_ID=your_google_client_id
GOOGLE_CLIENT_SECRET=your_google_client_secret
NEXT_PUBLIC_GOOGLE_CLIENT_ID=your_google_client_id

# Kolosal AI
KOLOSAL_API_URL=https://api.kolosal.ai
KOLOSAL_API_KEY=your_kolosal_api_key

# Cloudinary (optional)
NEXT_PUBLIC_CLOUDINARY_CLOUD_NAME=your_cloud_name
NEXT_PUBLIC_CLOUDINARY_API_KEY=your_cloudinary_api_key
NEXT_PUBLIC_CLOUDINARY_API_SECRET=your_cloudinary_api_secret

# Redis
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=

# RabbitMQ
RABBITMQ_HOST=localhost
RABBITMQ_PORT=5672
RABBITMQ_USER=your_user
RABBITMQ_PASSWORD=your_password

# Email Configuration
EMAIL_FROM=your_email@gmail.com
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USERNAME=your_email@gmail.com
SMTP_PASSWORD=your_smtp_app_password

# LiveKit Configuration
LIVEKIT_URL=wss://your-domain.com/rtc
LIVEKIT_API_KEY=your_livekit_api_key
LIVEKIT_API_SECRET=your_livekit_api_secret
```

> **⚠️ PENTING:** Jangan commit file `.env` ke repository! Pastikan file `.env` sudah ada di `.gitignore`.

## 🛠️ Development Commands

### Build Aplikasi
```bash
cd be
go build -o bin/server cmd/server/main.go
```

### Run Binary yang sudah di-build
```bash
cd be
./bin/server
```

### Run Tests
```bash
cd be
go test ./...
```

### Run Tests dengan Coverage
```bash
cd be
go test -cover ./...
```

### Format Code
```bash
cd be
go fmt ./...
```

### Lint Code
```bash
cd be
golangci-lint run
```

## 🐳 Docker Commands

### Build Docker Image
```bash
docker build -t yourapp-backend ./be
```

### Run Container Manual
```bash
docker run -p 5000:5000 --env-file .env yourapp-backend
```

### Rebuild Container (setelah perubahan code)
```bash
docker-compose up -d --build backend
```

### Restart Service Tertentu
```bash
docker-compose restart backend
```

### View Logs Real-time
```bash
docker-compose logs -f backend
```

### Execute Command di Container
```bash
docker-compose exec backend sh
```

## Services & Ports

Setelah menjalankan `docker-compose up -d`, services berikut akan tersedia:

- **API Server**: http://localhost:5000
- **PostgreSQL**: localhost:5432
- **Redis**: localhost:6379
- **RabbitMQ Management UI**: http://localhost:15672
  - Username: `yourapp` (default)
  - Password: `password123` (default)
- **pgweb (Database UI)**: http://localhost:8081
  - Web-based PostgreSQL client untuk melihat dan mengelola database
  - Otomatis terhubung ke database yang dikonfigurasi

## Architecture

Aplikasi ini menggunakan **Clean Architecture** dengan layer separation:

1. **Handler Layer** (`internal/app/`): HTTP handlers dan routing
2. **Service Layer** (`internal/service/`): Business logic
3. **Repository Layer** (`internal/repository/`): Data access
4. **Model Layer** (`internal/model/`): Domain models

## License

MIT

