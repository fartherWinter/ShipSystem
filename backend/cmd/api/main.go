package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata"

	"github.com/gin-gonic/gin"

	"shipsystem/backend/internal/config"
	"shipsystem/backend/internal/database"
	"shipsystem/backend/internal/handlers"
	"shipsystem/backend/internal/middleware"
	"shipsystem/backend/internal/repositories"
	"shipsystem/backend/internal/services"
	"shipsystem/backend/internal/ws"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		log.Fatalf("invalid config: %v", err)
	}

	db, err := database.Connect(cfg.DatabaseDSN)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	if err := database.MigrateAndSeed(db, cfg); err != nil {
		log.Fatalf("migrate database: %v", err)
	}

	hub := ws.NewHub()
	go hub.Run()

	store := repositories.NewStore(db)
	authSvc := services.NewAuthService(store, cfg)
	appSvc := services.NewAppService(store, hub)
	analyticsSvc := services.NewAnalyticsService(cfg)
	handler := handlers.NewHandler(authSvc, appSvc, analyticsSvc, hub, int(cfg.JWTTTL.Seconds()), cfg.IsProduction(), cfg.CORSOrigins)

	router := gin.New()
	router.Use(middleware.RequestID())
	router.Use(middleware.Recovery(log.Default()))
	router.Use(middleware.RequestLogger(log.Default()))
	router.Use(middleware.SecurityHeaders())
	router.Use(middleware.RequestBodyLimit(cfg.RequestBodyLimit))
	router.Use(cors(cfg.CORSOrigins))
	router.GET("/health", health)
	router.GET("/ready", readiness(db))

	api := router.Group("/api/v1")
	api.POST("/auth/login", handler.Login)
	api.POST("/auth/logout", handler.Logout)

	protected := api.Group("")
	protected.Use(middleware.JWT(authSvc, cfg.GoAPIToken))
	handler.RegisterRoutes(protected)

	router.GET("/ws/monitor", handler.MonitorWS)

	if err := serveHTTP(cfg, router); err != nil {
		log.Fatalf("run server: %v", err)
	}
}

func serveHTTP(cfg config.Config, handler http.Handler) error {
	server := newHTTPServer(cfg, handler)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		log.Printf("Go API listening on %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return <-errCh
	}
}

func newHTTPServer(cfg config.Config, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: cfg.HTTPHeaderTimeout,
		ReadTimeout:       cfg.HTTPReadTimeout,
		WriteTimeout:      cfg.HTTPWriteTimeout,
		IdleTimeout:       cfg.HTTPIdleTimeout,
	}
}

func health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func readiness(db databasePinger) gin.HandlerFunc {
	return func(c *gin.Context) {
		if db == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready", "database": "unavailable"})
			return
		}
		sqlDB, err := db.DB()
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready", "database": "unavailable"})
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := sqlDB.PingContext(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready", "database": "unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready", "database": "ok"})
	}
}

type databasePinger interface {
	DB() (*sql.DB, error)
}

func cors(origins []string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(origins))
	for _, origin := range origins {
		allowed[strings.TrimSpace(origin)] = struct{}{}
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if _, ok := allowed["*"]; ok {
			c.Header("Access-Control-Allow-Origin", origin)
		} else if _, ok := allowed[origin]; ok {
			c.Header("Access-Control-Allow-Origin", origin)
		}
		c.Header("Vary", "Origin")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
