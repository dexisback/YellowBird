package server 


import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/dexisback/YellowBird/internal/config"
)


type Server struct {
	engine *gin.Engine 
	config  *config.Config

}


func New(cfg *config.Config) *Server{
	engine := gin.New()

	server := &Server{
		engine: engine, 
		config: cfg, 
	}

	return server 

} 


func (s *Server) Run() error{
	address := fmt.Sprintf(":%s", s.config.Port)

	return s.engine.Run(address)
}



