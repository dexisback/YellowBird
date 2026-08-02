package project

import (
	"github.com/dexisback/YellowBird/internal/auth"
	"github.com/dexisback/YellowBird/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(router *gin.RouterGroup, handler *Handler, jwtService *auth.JWTService) {
	projects := router.Group("/projects")
	//public:
	projects.GET("", handler.ListProjects)
	projects.GET("/:id", handler.GetProject)

	//protected:
	protected := projects.Group("")
	protected.Use(middleware.Auth(jwtService))
	{
		protected.POST("", handler.CreateProject)
		protected.PUT("/:id", handler.UpdateProject)
		protected.DELETE("/:id", handler.DeleteProject)
	}
}
