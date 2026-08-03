package media

import (
	"github.com/dexisback/YellowBird/internal/auth"
	"github.com/dexisback/YellowBird/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(
	router *gin.RouterGroup,
	handler *Handler,
	jwtService *auth.JWTService,
) {
	media := router.Group("/media")
	media.Use(middleware.Auth(jwtService))

	{
		media.POST("", handler.CreateMedia)
		media.GET("", handler.ListMedia)
		media.GET("/:id", handler.GetMedia)
		media.PUT("/:id", handler.UpdateMedia)
		media.DELETE("/:id", handler.DeleteMedia)
	}
}
