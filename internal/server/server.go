package server

import (
	"fmt"

	"github.com/dexisback/YellowBird/internal/config"
	"github.com/dexisback/YellowBird/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Server struct {
	engine *gin.Engine
	config *config.Config
	db     *gorm.DB
}

func New(cfg *config.Config, db *gorm.DB) *Server {
	engine := gin.New() //because private , nobody outside the server package should be able to access this

	engine.Use(
		middleware.Recovery(),
		middleware.RequestID(),
		middleware.Logging(),
	)

	server := &Server{
		engine: engine,
		config: cfg,
		db:     db,
	}

	server.registerRoutes()

	return server

}

func (s *Server) Run() error {
	address := fmt.Sprintf(":%s", s.config.Port)

	return s.engine.Run(address)
}

//instead of engine.Run() , main.go will only do srv.Run(). this hides gin from rest of the application
