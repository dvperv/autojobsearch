package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"

	"autojobsearch/backend/internal/api/handlers"
	authmiddleware "autojobsearch/backend/internal/api/middleware"
	"autojobsearch/backend/internal/services"
	"autojobsearch/backend/internal/storage"
	"autojobsearch/backend/pkg/utils"
)

func main() {
	// Инициализация логгера
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatal("Failed to create logger:", err)
	}
	defer logger.Sync()

	logger.Info("🚀 Starting AutoJobSearch backend...")

	// Загрузка конфигурации
	cfg := loadConfig()

	// Инициализация базы данных
	db, err := storage.NewDatabase(cfg.DatabaseURL, logger)
	if err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer db.Close()

	// Инициализация Redis
	redisClient, err := storage.NewRedisClient(
		cfg.RedisAddress,
		cfg.RedisPassword,
		cfg.RedisDB,
		logger,
	)
	if err != nil {
		logger.Fatal("Failed to connect to Redis", zap.Error(err))
	}
	defer redisClient.Close()

	// Инициализация сервисов
	hhService := services.NewHHService(&cfg.HHConfig, db, redisClient, logger)
	notificationService := services.NewNotificationService(db, redisClient, logger)
	matcher := services.NewSmartMatcher(logger)
	automationEngine := services.NewAutomationEngine(
		db, redisClient, hhService, matcher, notificationService, logger,
	)

	// Инициализация хендлеров
	authHandler := handlers.NewAuthHandler(db, redisClient, logger)
	hhAuthHandler := handlers.NewHHAuthHandler(hhService, db, redisClient, logger)
	automationHandler := handlers.NewAutomationHandler(automationEngine, db, logger)
	resumeHandler := handlers.NewResumeHandler(db, logger)
	applicationHandler := handlers.NewApplicationHandler(db, logger)

	// Создание роутера
	r := chi.NewRouter()

	// Middleware
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(authmiddleware.CORSMiddleware)
	r.Use(chimiddleware.Timeout(30 * time.Second))
	r.Use(chimiddleware.Compress(5))

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		// Проверка всех зависимостей
		servicesStatus := make(map[string]string)

		// Проверка базы данных
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		if dbErr := db.HealthCheck(ctx); dbErr == nil {
			servicesStatus["database"] = "healthy"
		} else {
			servicesStatus["database"] = "unhealthy"
			logger.Error("Database health check failed", zap.Error(dbErr))
		}

		// Проверка Redis
		if redisErr := redisClient.HealthCheck(ctx); redisErr == nil {
			servicesStatus["redis"] = "healthy"
		} else {
			servicesStatus["redis"] = "unhealthy"
			logger.Error("Redis health check failed", zap.Error(redisErr))
		}

		// Проверка HH.ru API (базовая проверка конфигурации)
		if cfg.HHConfig.ClientID != "" && cfg.HHConfig.ClientSecret != "" {
			servicesStatus["hh_api"] = "configured"
		} else {
			servicesStatus["hh_api"] = "not_configured"
		}

		// Добавляем информацию о сервере
		servicesStatus["server"] = "running"
		servicesStatus["version"] = "1.0.0"

		utils.WriteHealthCheck(w, "healthy", servicesStatus)
	})

	// API routes
	r.Route("/api", func(r chi.Router) {
		// Public routes (не требуют аутентификации)
		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", authHandler.Register)
			r.Post("/login", authHandler.Login)
			r.Post("/refresh", authHandler.RefreshToken)
			r.Post("/forgot-password", authHandler.ForgotPassword)
			r.Post("/reset-password", authHandler.ResetPassword)
		})

		// Protected routes (требуют аутентификации)
		r.Group(func(r chi.Router) {
			r.Use(authmiddleware.AuthMiddleware)

			// User profile routes
			r.Route("/user", func(r chi.Router) {
				r.Get("/profile", authHandler.GetProfile)
				r.Put("/profile", authHandler.UpdateProfile)
				r.Put("/password", authHandler.ChangePassword)
				r.Post("/logout", authHandler.Logout)
			})

			// HH.ru OAuth routes
			r.Mount("/hh", hhAuthHandler.Routes())

			// Automation routes
			r.Mount("/automation", automationHandler.Routes())

			// Resume routes
			r.Mount("/resumes", resumeHandler.Routes())

			// Application routes
			r.Mount("/applications", applicationHandler.Routes())

			// Settings routes
			r.Route("/settings", func(r chi.Router) {
				r.Get("/search", automationHandler.GetSearchSettings)
				r.Put("/search", automationHandler.UpdateSearchSettings)
				r.Get("/notifications", authHandler.GetNotificationSettings)
				r.Put("/notifications", authHandler.UpdateNotificationSettings)
			})

			// Statistics routes
			r.Route("/stats", func(r chi.Router) {
				r.Get("/overview", automationHandler.GetAutomationStats)
				r.Get("/daily", automationHandler.GetDailyStats)
				r.Get("/applications", applicationHandler.GetApplicationStats)
			})
		})
	})

	// Documentation and info routes (public)
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
			"name":        "AutoJobSearch API",
			"version":     "1.0.0",
			"description": "Automatic job search and application system",
			"docs":        "/docs",
			"health":      "/health",
			"endpoints": map[string]string{
				"api":        "/api",
				"auth":       "/api/auth",
				"automation": "/api/automation",
				"hh":         "/api/hh",
			},
		})
	})

	r.Get("/docs", func(w http.ResponseWriter, r *http.Request) {
		utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
			"openapi": "3.0.0",
			"info": map[string]string{
				"title":   "AutoJobSearch API",
				"version": "1.0.0",
			},
			"paths": map[string]interface{}{
				// Здесь можно добавить документацию OpenAPI
			},
		})
	})

	// Статика для документации (опционально)
	r.Handle("/docs/*", http.StripPrefix("/docs/", http.FileServer(http.Dir("./docs"))))

	// 404 handler
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		utils.WriteError(w, http.StatusNotFound, "Route not found")
	})

	// Method not allowed handler
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		utils.WriteError(w, http.StatusMethodNotAllowed, "Method not allowed")
	})

	// Настройка сервера
	server := &http.Server{
		Addr:         cfg.ServerAddress,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
		ErrorLog:     log.New(os.Stderr, "http: ", log.LstdFlags),
	}

	// Запуск сервера в горутине
	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("🌐 Server starting",
			zap.String("address", cfg.ServerAddress),
			zap.String("env", cfg.Environment))

		if cfg.Environment == "development" {
			serverErrors <- server.ListenAndServe()
		} else {
			// В продакшене используем HTTPS
			serverErrors <- server.ListenAndServeTLS(
				cfg.TLSCertPath,
				cfg.TLSKeyPath,
			)
		}
	}()

	// Graceful shutdown
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		logger.Fatal("❌ Server failed to start", zap.Error(err))

	case sig := <-shutdown:
		logger.Info("🛑 Shutdown signal received",
			zap.String("signal", sig.String()))

		// Даем время на завершение текущих запросов
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Останавливаем сервер
		if err := server.Shutdown(ctx); err != nil {
			logger.Error("⚠️ Graceful shutdown failed", zap.Error(err))
			if err := server.Close(); err != nil {
				logger.Fatal("💥 Force shutdown failed", zap.Error(err))
			}
		}

		// Останавливаем сервисы
		logger.Info("👋 Stopping services...")
		automationEngine.StopAllJobs()

		logger.Info("✅ Server stopped gracefully")
	}
}

// Config конфигурация приложения
type Config struct {
	Environment   string
	ServerAddress string
	DatabaseURL   string
	RedisAddress  string
	RedisPassword string
	RedisDB       int
	JWTSecret     string
	TLSCertPath   string
	TLSKeyPath    string
	HHConfig      services.HHServiceConfig
}

func loadConfig() *Config {
	return &Config{
		Environment:   getEnv("ENVIRONMENT", "development"),
		ServerAddress: getEnv("SERVER_ADDRESS", ":8080"),
		DatabaseURL:   getEnv("DATABASE_URL", "postgres://postgres:password@localhost:5432/autojobsearch?sslmode=disable"),
		RedisAddress:  getEnv("REDIS_ADDRESS", "localhost:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       getEnvAsInt("REDIS_DB", 0),
		JWTSecret:     getEnv("JWT_SECRET", "your-super-secret-jwt-key-change-in-production"),
		TLSCertPath:   getEnv("TLS_CERT_PATH", ""),
		TLSKeyPath:    getEnv("TLS_KEY_PATH", ""),
		HHConfig: services.HHServiceConfig{
			ClientID:     getEnv("HH_CLIENT_ID", ""),
			ClientSecret: getEnv("HH_CLIENT_SECRET", ""),
			RedirectURL:  getEnv("HH_REDIRECT_URL", "http://localhost:8080/api/hh/callback"),
			AuthURL:      getEnv("HH_AUTH_URL", "https://hh.ru/oauth/authorize"),
			TokenURL:     getEnv("HH_TOKEN_URL", "https://hh.ru/oauth/token"),
			APIBaseURL:   getEnv("HH_API_URL", "https://api.hh.ru"),
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
