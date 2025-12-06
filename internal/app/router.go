package app

import (
	"log"
	"os"
	"strings"
	"time"
	"yourapp/internal/config"
	"yourapp/internal/model"
	"yourapp/internal/repository"
	"yourapp/internal/service"
	"yourapp/internal/util"
	"yourapp/internal/websocket"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func NewRouter(cfg *config.Config) *gin.Engine {
	// Set Gin mode
	if cfg.ServerPort == "5000" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()

	// CORS middleware - allow multiple origins
	allowedOrigins := []string{
		cfg.ClientURL,
		"http://localhost:3000",
		"http://127.0.0.1:3000",
		"https://zoom.zacloth.com",
		"https://hackathon-zoom-ai-kolosal.vercel.app",
	}
	r.Use(corsMiddleware(allowedOrigins))

	// Add route logging middleware - ALWAYS ACTIVE for debugging
	r.Use(func(c *gin.Context) {
		log.Printf("[ROUTE] %s %s", c.Request.Method, c.Request.URL.Path)
		log.Printf("[ROUTE] Query: %s", c.Request.URL.RawQuery)
		log.Printf("[ROUTE] FullPath: %s", c.FullPath())
		c.Next()
		// Log 404s specifically
		if c.Writer.Status() == 404 {
			log.Printf("[404] Route not found: %s %s", c.Request.Method, c.Request.URL.Path)
			log.Printf("[404] FullPath was: %s", c.FullPath())
			log.Printf("[404] Available routes:")
			for _, route := range r.Routes() {
				if route.Method == c.Request.Method {
					log.Printf("[404]   - %s %s", route.Method, route.Path)
				}
			}
		}
	})

	// Initialize database
	db, err := initDB(cfg)
	if err != nil {
		panic("Failed to connect to database: " + err.Error())
	}

	// Auto migrate
	if err := db.AutoMigrate(&model.User{}, &model.Room{}, &model.RoomParticipant{}, &model.ChatMessage{}); err != nil {
		panic("Failed to migrate database: " + err.Error())
	}

	// Initialize repositories
	userRepo := repository.NewUserRepository(db)
	roomRepo := repository.NewRoomRepository(db)
	chatRepo := repository.NewChatRepository(db)

	// Initialize RabbitMQ with retry logic
	rabbitMQ := initRabbitMQWithRetry(cfg)

	// Initialize email service
	emailService := service.NewEmailService(cfg)

	// Initialize email worker if RabbitMQ is available
	var emailWorker *service.EmailWorker
	if rabbitMQ != nil {
		emailWorker = service.NewEmailWorker(emailService, rabbitMQ)
		if err := emailWorker.Start(); err != nil {
			log.Printf("Warning: Failed to start email worker: %v", err)
		} else {
			log.Println("Email worker started successfully")
		}
	} else {
		log.Println("Email worker not started - RabbitMQ connection failed. Will retry on first email send.")
		// Note: RabbitMQ will be reconnected automatically when email is sent via ensureRabbitMQ()
		// No need for continuous polling - lazy reconnection is more efficient
	}

	// Initialize services
	authService := service.NewAuthServiceWithConfig(userRepo, cfg.JWTSecret, rabbitMQ, cfg)
	roomService := service.NewRoomService(roomRepo, userRepo, cfg)
	chatService := service.NewChatService(chatRepo, roomRepo, userRepo)
	kolosalService := service.NewKolosalService(cfg.KolosalAPIURL, cfg.KolosalAPIKey)

	// Initialize WebSocket hub
	wsHub := websocket.NewHub()
	go wsHub.Run()

	// Initialize handlers
	authHandler := NewAuthHandler(authService, cfg.JWTSecret)
	roomHandler := NewRoomHandler(roomService)
	chatHandler := NewChatHandler(chatService, kolosalService, roomService, wsHub, cfg.JWTSecret)

	// API routes
	api := r.Group("/api/v1")
	{
		// Auth routes
		auth := api.Group("/auth")
		{
			auth.POST("/register", authHandler.Register)
			auth.POST("/login", authHandler.Login)
			auth.POST("/verify-otp", authHandler.VerifyOTP)
			auth.POST("/resend-otp", authHandler.ResendOTP)
			auth.POST("/google-oauth", authHandler.GoogleOAuth)
			auth.POST("/refresh-token", authHandler.RefreshToken)
			auth.POST("/forgot-password", authHandler.RequestResetPassword)
			auth.POST("/verify-reset-password", authHandler.VerifyResetPassword)
			auth.POST("/reset-password", authHandler.ResetPassword)
			auth.POST("/verify-email", authHandler.VerifyEmail)

			// Protected routes
			auth.GET("/me", authHandler.AuthMiddleware(), authHandler.GetMe)
		}

		// Room routes
		rooms := api.Group("/rooms")
		{
			// Public routes
			rooms.GET("", roomHandler.GetRooms)
			rooms.GET("/ids", roomHandler.GetRoomIDs)                              // Get all room IDs for AI
			rooms.GET("/my", authHandler.AuthMiddleware(), roomHandler.GetMyRooms) // Specific route before :id

			// Protected routes
			rooms.POST("", authHandler.AuthMiddleware(), roomHandler.CreateRoom)

			// Chat routes - specific routes before general :id routes
			// IMPORTANT: More specific routes must be defined before less specific ones
			// Register test-kolosal FIRST before all other :id routes to ensure it's matched
			log.Println("[ROUTER] Registering test-kolosal route in rooms group (FIRST)...")
			rooms.POST("/:id/test-kolosal", authHandler.AuthMiddleware(), chatHandler.TestKolosalAPI)
			log.Println("[ROUTER] ✓ test-kolosal route registered successfully in rooms group")

			// Other chat routes
			rooms.GET("/:id/messages", chatHandler.GetMessages)
			rooms.POST("/:id/messages", authHandler.AuthMiddleware(), chatHandler.CreateMessage)
			rooms.GET("/:id/chat/ws", chatHandler.ServeWebSocket)

			// Room action routes
			rooms.POST("/:id/join", authHandler.AuthMiddleware(), roomHandler.JoinRoom)
			rooms.POST("/:id/leave", authHandler.AuthMiddleware(), roomHandler.LeaveRoom)
			rooms.DELETE("/:id", authHandler.AuthMiddleware(), roomHandler.DeleteRoom)

			// General :id route - must be last
			rooms.GET("/:id", roomHandler.GetRoom)
		}
	}

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Register test-kolosal route directly at root level as final fallback
	// This MUST be before NoRoute handler to ensure it's checked first
	log.Println("[ROUTER] Registering test-kolosal route at root level...")
	r.POST("/api/v1/rooms/:id/test-kolosal", authHandler.AuthMiddleware(), chatHandler.TestKolosalAPI)
	log.Println("[ROUTER] ✓ test-kolosal route registered successfully at root level")

	// NoRoute handler - catch 404 and check if it's test-kolosal
	// This is a fallback in case the route is not registered properly
	log.Println("[ROUTER] Registering NoRoute handler...")
	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		method := c.Request.Method
		fullPath := c.FullPath()

		log.Printf("[NoRoute] ===== 404 DETECTED =====")
		log.Printf("[NoRoute] Method: %s", method)
		log.Printf("[NoRoute] Path: %s", path)
		log.Printf("[NoRoute] FullPath: %s", fullPath)
		log.Printf("[NoRoute] RequestURI: %s", c.Request.RequestURI)

		// Check if it's a test-kolosal request
		if method == "POST" && strings.Contains(path, "/test-kolosal") {
			log.Printf("[NoRoute] ✓ Detected test-kolosal request!")
			// Extract room ID from path: /api/v1/rooms/:id/test-kolosal
			parts := strings.Split(strings.Trim(path, "/"), "/")
			log.Printf("[NoRoute] Path parts: %v (length: %d)", parts, len(parts))

			// Path format: /api/v1/rooms/:id/test-kolosal
			// After split and trim: ["api", "v1", "rooms", ":id", "test-kolosal"]
			// We need to find "rooms" and get the next element as room ID
			roomID := ""
			for i, part := range parts {
				if part == "rooms" && i+1 < len(parts) {
					roomID = parts[i+1]
					break
				}
			}

			if roomID != "" {
				log.Printf("[NoRoute] ✓ Extracted room ID: %s", roomID)
				// Set the room ID as a param
				c.Params = []gin.Param{{Key: "id", Value: roomID}}

				// Run AuthMiddleware first to set userID in context
				log.Printf("[NoRoute] Running AuthMiddleware...")
				authHandler.AuthMiddleware()(c)

				// Check if middleware aborted (unauthorized)
				if c.IsAborted() {
					log.Printf("[NoRoute] AuthMiddleware aborted request")
					return
				}

				// Call the handler directly
				log.Printf("[NoRoute] Calling TestKolosalAPI handler...")
				chatHandler.TestKolosalAPI(c)
				return
			} else {
				log.Printf("[NoRoute] ✗ Could not extract room ID from path")
			}
		}

		log.Printf("[NoRoute] Returning 404 response")
		c.JSON(404, gin.H{"error": "Route not found", "path": path, "method": method, "fullPath": fullPath})
	})

	// Debug: Print all routes - ALWAYS ACTIVE for debugging
	log.Println("=== Registered Routes ===")
	testKolosalFound := false
	for _, route := range r.Routes() {
		if route.Path != "" {
			log.Printf("Route: %s %s", route.Method, route.Path)
			if strings.Contains(route.Path, "test-kolosal") {
				testKolosalFound = true
				log.Printf("  -> TEST-KOLOSAL ROUTE FOUND: %s %s", route.Method, route.Path)
			}
		}
	}
	if !testKolosalFound {
		log.Println("WARNING: test-kolosal route NOT found in registered routes!")
	} else {
		log.Println("SUCCESS: test-kolosal route is registered")
	}
	log.Println("========================")

	return r
}

func initDB(cfg *config.Config) (*gorm.DB, error) {
	dsn := cfg.DatabaseURL
	if dsn == "" {
		dsn = "host=" + cfg.PostgresHost +
			" port=" + cfg.PostgresPort +
			" user=" + cfg.PostgresUser +
			" password=" + cfg.PostgresPassword +
			" dbname=" + cfg.PostgresDB +
			" sslmode=" + cfg.PostgresSSLMode
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	return db, nil
}

// initRabbitMQWithRetry attempts to connect to RabbitMQ with exponential backoff retry
func initRabbitMQWithRetry(cfg *config.Config) *util.RabbitMQClient {
	maxRetries := 10
	initialDelay := 2 * time.Second
	maxDelay := 30 * time.Second

	for attempt := 1; attempt <= maxRetries; attempt++ {
		rabbitMQ, err := util.NewRabbitMQClient(cfg)
		if err == nil {
			log.Printf("RabbitMQ connected successfully on attempt %d", attempt)
			return rabbitMQ
		}

		if attempt < maxRetries {
			// Calculate delay with exponential backoff
			delay := initialDelay * time.Duration(1<<uint(attempt-1))
			if delay > maxDelay {
				delay = maxDelay
			}

			log.Printf("Failed to connect to RabbitMQ (attempt %d/%d): %v. Retrying in %v...", attempt, maxRetries, err, delay)
			time.Sleep(delay)
		} else {
			log.Printf("Warning: Failed to connect to RabbitMQ after %d attempts: %v. Email sending will be disabled.", maxRetries, err)
			log.Println("Note: RabbitMQ will be retried automatically when email is sent (if connection is restored)")
		}
	}

	return nil
}

func corsMiddleware(allowedOrigins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// In development mode, be more permissive
		isDevelopment := len(os.Getenv("PORT")) > 0 && os.Getenv("PORT") == "5000"

		// Check if origin is in allowed list
		allowed := false
		if origin == "" {
			// Server-to-server request (e.g., from Next.js API route)
			// Allow it but don't set CORS headers (not needed for server-to-server)
			allowed = true
		} else {
			// Check if origin is in allowed list
			for _, allowedOrigin := range allowedOrigins {
				if origin == allowedOrigin {
					allowed = true
					c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
					c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
					break
				}
			}

			// In development, also allow localhost origins even if not in list
			if !allowed && isDevelopment {
				if strings.HasPrefix(origin, "http://localhost:") ||
					strings.HasPrefix(origin, "http://127.0.0.1:") ||
					strings.HasPrefix(origin, "http://[::1]:") {
					allowed = true
					c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
					c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
					log.Printf("[CORS] Allowing development origin: %s", origin)
				}
			}
		}

		// Set CORS headers for allowed origins
		if allowed && origin != "" {
			c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
			c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")
		} else if origin != "" {
			// Log blocked origin for debugging
			log.Printf("[CORS] Blocked origin: %s (not in allowed list)", origin)
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
