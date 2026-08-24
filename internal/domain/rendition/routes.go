package rendition 

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
	renditions:= router.Group("/renditions")
	renditions.Use(middleware.Auth(jwtService))


	{
			renditions.POST("", handler.CreateRendition)
			renditions.GET("", handler.ListRenditionsByMedia)
			renditions.GET("/:id", handler.GetRendition)
			renditions.DELETE("/:id", handler.DeleteRendition)

	}
}



