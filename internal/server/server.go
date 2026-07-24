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
	engine := gin.New()  //because private , nobody outside the server package should be able to access this 

	server := &Server{
		engine: engine, 
		config: cfg, 
	}

	server.registerRoutes()

	return server 

} 


func (s *Server) Run() error{
	address := fmt.Sprintf(":%s", s.config.Port)

	return s.engine.Run(address)
}




//instead of engine.Run() , main.go will only do srv.Run(). this hides gin from rest of the application 
//
