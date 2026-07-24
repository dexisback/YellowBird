package server 
//we still import net/http, because we dont wanna write statusCodes as literal numbers but rather use the functionality of writing http.something soemthing 


import(
	"net/http"

	"github.com/gin-gonic/gin"
)



func (s *Server) registerRoutes(){
	s.engine.GET("/health", healthHandler)
}


func healthHandler(c *gin.Context){
	c.JSON(http.StatusOK, gin.H{
		"status": "ok (also my first gin response)", 
	})
}



