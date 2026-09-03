package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lk/zoek/backend/internal/config"
	"github.com/lk/zoek/backend/internal/handler"
	"github.com/lk/zoek/backend/internal/logger"
	"github.com/lk/zoek/backend/internal/middleware"
	"github.com/lk/zoek/backend/internal/store"
	"github.com/lk/zoek/backend/pkg/wechat"
)

func main() {
	// Parse flags
	configPath := flag.String("config", "", "path to YAML config file (default: use built-in defaults + env)")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	log, err := logger.New(cfg.Log.Level, cfg.Log.Encoding, cfg.Log.Output)
	if err != nil {
		fmt.Fprintf(os.Stderr, "初始化日志失败: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync()

	log.Info("zoek 后端服务启动",
		logger.String("mode", cfg.Server.Mode),
		logger.String("port", cfg.Server.Port),
		logger.String("db_driver", cfg.Database.Driver),
		logger.String("db_dsn", cfg.Database.DSNString()),
		logger.String("log_level", cfg.Log.Level),
	)

	// Database
	s, err := store.NewFromConfig(cfg.Database.Driver, cfg.Database.DSNString(), cfg.Database.LogLevel, log)
	if err != nil {
		log.Fatal("数据库连接失败", logger.ErrorField(err))
	}
	defer s.Close()

	if err := s.AutoMigrate(); err != nil {
		log.Fatal("数据库迁移失败", logger.ErrorField(err))
	}

	// Expire old forming games and adjustments on startup
	if err := s.ExpireOldFormingGames(); err != nil {
		log.Warn("清理过期牌桌失败", logger.ErrorField(err))
	}
	if err := s.ExpireOldAdjustments(); err != nil {
		log.Warn("清理过期调整失败", logger.ErrorField(err))
	}

	// JWT
	jwtManager := middleware.NewJWTManager(cfg.JWT.Secret, cfg.JWT.TokenExpiry)

	// Gin
	gin.SetMode(cfg.Server.Mode)
	r := gin.New()
	r.Use(middleware.RequestID())
	r.Use(middleware.Logging(log))
	r.Use(middleware.ErrorHandler(log))
	r.Use(middleware.CORS(cfg.CORS.AllowOrigins))

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":     "ok",
			"request_id": middleware.GetRequestID(c),
		})
	})

	// WeChat
	wxClient := wechat.NewClient(cfg.Wechat.AppID, cfg.Wechat.AppSecret)

	// Handlers
	authHandler := handler.NewAuthHandler(s, jwtManager, wxClient)
	gameHandler := handler.NewGameHandler(s, jwtManager, wxClient)
	roundHandler := handler.NewRoundHandler(s)
	adjHandler := handler.NewAdjustmentHandler(s)
	settlementHandler := handler.NewSettlementHandler(s)

	// API v1
	v1 := r.Group("/api/v1")
	{
		// Public: login
		v1.POST("/auth/login", authHandler.Login)

		// Authenticated
		auth := v1.Group("")
		auth.Use(jwtManager.Auth())
		{
			// User
			auth.GET("/user/profile", authHandler.GetProfile)
			auth.PUT("/user/profile", authHandler.UpdateProfile)

			// Games
			auth.POST("/games", gameHandler.CreateGame)
			auth.GET("/games/active", gameHandler.GetActiveGames)
			auth.GET("/games/history", gameHandler.GetHistoryGames)
			auth.GET("/games/:game_id", gameHandler.GetGame)
			auth.POST("/games/join", gameHandler.JoinGame)
			auth.POST("/games/:game_id/start", gameHandler.StartGame)
			auth.POST("/games/:game_id/cancel", gameHandler.CancelGame)
			auth.POST("/games/:game_id/end", gameHandler.EndGame)
			auth.GET("/games/:game_id/qrcode", gameHandler.GetGameQRCode)

			// Rounds
			auth.POST("/games/:game_id/rounds", roundHandler.CreateNextRound)
			auth.GET("/games/:game_id/rounds/current", roundHandler.GetCurrentRound)
			auth.PUT("/games/:game_id/rounds/:round_id/submission", roundHandler.SubmitScore)
			auth.POST("/games/:game_id/rounds/:round_id/lock", roundHandler.LockRound)
			auth.POST("/games/:game_id/rounds/:round_id/next", roundHandler.CreateNextRound)
			auth.GET("/games/:game_id/rounds/:round_id", roundHandler.GetRoundDetail)

			// Adjustments
			auth.POST("/games/:game_id/rounds/:round_id/adjustments", adjHandler.CreateAdjustment)
			auth.GET("/games/:game_id/adjustments", adjHandler.ListAdjustments)
			auth.POST("/games/:game_id/adjustments/:adjustment_id/accept", adjHandler.AcceptAdjustment)
			auth.POST("/games/:game_id/adjustments/:adjustment_id/reject", adjHandler.RejectAdjustment)
			auth.POST("/games/:game_id/adjustments/:adjustment_id/cancel", adjHandler.CancelAdjustment)

			// Settlement and History
			auth.GET("/games/:game_id/settlement", settlementHandler.GetSettlement)
			auth.GET("/games/:game_id/history", settlementHandler.GetHistoryDetail)
		}
	}

	// HTTP Server with graceful shutdown
	addr := fmt.Sprintf(":%s", cfg.Server.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	// Start server in goroutine
	go func() {
		log.Info("HTTP 服务监听", logger.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("服务启动失败", logger.ErrorField(err))
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Info("收到关闭信号", logger.String("signal", sig.String()))

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Server.ShutdownTimeout)*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("服务关闭失败", logger.ErrorField(err))
	}

	log.Info("zoek 后端服务已关闭")
}
