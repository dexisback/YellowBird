package server

//we still import net/http, because we dont wanna write statusCodes as literal numbers but rather use the functionality of writing http.something soemthing

import (
	"context"
	"net/http"

	"github.com/dexisback/YellowBird/internal/auth"
	"github.com/dexisback/YellowBird/internal/domain/job"
	"github.com/dexisback/YellowBird/internal/domain/project"
	"github.com/dexisback/YellowBird/internal/domain/rendition"
	"github.com/dexisback/YellowBird/internal/domain/user"
	"github.com/dexisback/YellowBird/internal/queue"
	"github.com/gin-gonic/gin"
)

func (s *Server) registerRoutes() {
	s.engine.GET("/health", healthHandler)

	api := s.engine.Group("/api/v1")

	//jwt:
	jwtService := auth.NewJWTService(s.config.JWT_SECRET)

	//creating the redis queue, right after creating the jwt service
	redisQueue := queue.NewRedisQueue(
		s.config.RedisAddr,
		s.config.RedisPassword,
		s.config.RedisDB,
		"api",
	)

	if err := redisQueue.Ping(context.Background()); err != nil {
		panic(err)
	}

	//project kundli:
	projectRepository := project.NewRepository(s.db)
	projectService := project.NewService(projectRepository)
	projectHandler := project.NewHandler(projectService)

	project.RegisterRoutes(api, projectHandler, jwtService)

	//user kundli:
	userRepository := user.NewRepository(s.db)
	userService := user.NewService(userRepository, jwtService)
	userHandler := user.NewHandler(userService)
	user.RegisterRoutes(api, userHandler)

	//rendntion kundli:
	renditionRepository := rendition.NewRepository(s.db)
	renditionService := rendition.NewService(renditionRepository)
	renditionHandler := rendition.NewHandler(renditionService)
	rendition.RegisterRoutes(api, renditionHandler, jwtService)

	//job repository + service kundli.
	//JobService now recieves the redis queue so that:
	//createJOb() -> Postgres -> redis stream -> worker

	jobRepository := job.NewRepository(s.db)
	jobService := job.NewService(jobRepository, redisQueue)
	jobHandler := job.NewHandler(jobService)
	job.RegisterRoutes(api, jobHandler, jwtService)

}

func healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok (also my first gin response)",
	})
}
