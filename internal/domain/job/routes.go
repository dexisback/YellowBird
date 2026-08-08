package job

import(
	"github.com/dexisback/YellowBird/internal/auth"
	"github.com/dexisback/YellowBird/internal/server/middleware"
	"github.com/gin-gonic/gin"
)


func RegisterRoutes(router *gin.RouterGroup, handler *Handler, jwtService *auth.JWTService) {
	jobs := router.Group("/jobs")
	jobs.Use(middleware.Auth(jwtService))


	{
		jobs.POST("", handler.CreateJob)
		jobs.GET("", handler.ListJobsByMedia)
		jobs.GET("/:id", handler.GetJob)
		jobs.DELETE("/:id", handler.DeleteJob)
		

	}
}