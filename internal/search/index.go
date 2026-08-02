package server



import(
	"net/http"

	"github.com/dexisback/YellowBird/internal/domain/project"
	"github.com/dexisback/YellowBird/internal/domain/user"
	"github.com/gin-gonic/gin"
)


func (s *Server) registerRoutes() {
	s.engine.GET("/health", healthHandler)

	api := s.engine.Group("/api/v1")

	//project domain:
	projectRepository := project.NewRepository(s.db)
	projectService := project.NewService(projectRepository)
	projectHandler := project.NewHandler(projectService)
	project.RegisterRoutes(api, projectHandler)


	//user domain:
	userRepository := user.NewRepository(s.db)
	userService := user.NewService(userRepository)
	userHandler := user.NewHandler(userService)
	user.RegisterRoutes(api, userHandler)
 }


 func healthHandler(c *gin.Context){
	c.JSON(http.StatusOK, gin.H{
		"status": "ok (first gin response :) )",
	})
 }


 