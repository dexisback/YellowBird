package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/dexisback/YellowBird/internal/config"
	"github.com/dexisback/YellowBird/internal/queue"
	"github.com/dexisback/YellowBird/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Server struct {
	engine *gin.Engine
	config *config.Config
	db     *gorm.DB
	redis  *queue.RedisQueue
}

func New(cfg *config.Config, db *gorm.DB) *Server {
	redisQueue := queue.NewRedisQueue(
		cfg.RedisAddr,
		cfg.RedisPassword,
		cfg.RedisDB,
		"api",
	)
	engine := gin.New() //because private , nobody outside the server package should be able to access this

	engine.Use(
		middleware.Recovery(),
		middleware.RequestID(),
		middleware.Logging(),
	)

	server := &Server{
		engine: engine,
		config: cfg,
		redis:  redisQueue,
		db:     db,
	}

	server.registerRoutes()

	return server
}

//instead of engine.Run() , main.go will only do srv.Run(). this hides gin from rest of the application

func (s *Server) Run(ctx context.Context) error {
	address := fmt.Sprintf(":%s", s.config.Port)
	httpServer := &http.Server{
		Addr:    address,
		Handler: s.engine,
	}

	serverErr := make(chan error, 1)

	go func() {
		serverErr <- httpServer.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("server failed: %w", err)
		}
		return nil
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("server shutdown failed: %w", err)
		}

		err := <-serverErr
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("server failed while shutting down: %w", err)
		}

		return nil
	}
}
