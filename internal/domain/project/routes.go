package project


import "github.com/gin-gonic/gin"


func RegisterRoutes(router *gin.RouterGroup, handler *Handler) {
	projects := router.Group("/projects")

	{
		projects.POST("", handler.CreateProject)
		projects.GET("", handler.ListProjects)
		projects.GET("/:id", handler.GetProject)
		projects.PUT("/:id", handler.UpdateProject)
		projects.DELETE("/:id", handler.DeleteProject)
	}
}