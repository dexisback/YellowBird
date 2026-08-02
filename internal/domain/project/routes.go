package project

import (
	"github.com/dexisback/YellowBird/internal/auth"
	"github.com/dexisback/YellowBird/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(router *gin.RouterGroup, handler *Handler, jwtService *auth.JWTService) {
	// projects := router.Group("/projects")
	//public: no more public since projects are being accessed by a particular user


	//protected:
	protected := router.Group("/projects")
	protected.Use(middleware.Auth(jwtService))
	{
		protected.POST("", handler.CreateProject)
		protected.GET("", handler.ListProjects)
		protected.GET("/:id", handler.GetProject)
		protected.PUT("/:id", handler.UpdateProject)
		protected.DELETE("/:id", handler.DeleteProject)
	}
}
