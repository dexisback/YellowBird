package user

import "github.com/gin-gonic/gin"

func RegisterRoutes(router *gin.RouterGroup, handler *Handler) {
	users := router.Group("/users")
	{
		users.POST("/register", handler.RegisterUser)
		users.POST("/login", handler.LoginUser)
		users.GET("", handler.ListUsers)

		// users.GET("/:id", handler.ListUser)
		users.GET("/:id", handler.GetUser)

		users.DELETE("/:id", handler.DeleteUser)

	}
}
