package middleware


import (
	"log"
	"time"
	"github.com/gin-gonic/gin"
)


func Logging() gin.HandlerFunc {
	return func(c *gin.Context){


		startTime := time.Now()
		c.Next()

		endTime := time.Since(startTime)

		requestID, _ := c.Get(RequestIDKey)  //request id generation, we stored it through c.Set() in requestID middleware. now we get it through this 

		log.Println(
			requestID, c.Request.Method, c.Request.URL.Path, c.Writer.Status(), endTime,
		)
	}
	}




