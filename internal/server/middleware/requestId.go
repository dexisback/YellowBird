//requestId . go does what it says lol, we attach a request Id to each request so logging me mushkil na ho. what stripe's canonical loggin was all about and shi 


package middleware


import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)


const RequestIDKey = "request_id"



func RequestID() gin.HandlerFunc {   //returns a handler function 
	return func(c *gin.Context){   //returns a function of type c *github.Context 
		
		
		requestID := uuid.NewString()   //generate a random UUID

		c.Set(RequestIDKey,  requestID)  //string concatenation ("request_id<uuid>")
		c.Next()   //then next. now we call this whenever needed to generate a random uuid 



	}
}








