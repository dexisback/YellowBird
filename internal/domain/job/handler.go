package job


import (
	"net/http"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)


type Handler struct{
	service Service 
}


func NewHandler(service Service) *Handler{
	return &Handler{
		service: service,
	}
}



func (h *Handler) CreateJob(c *gin.Context) {
	var req CreateJobRequest


	if err := c.ShouldBindJSON(&req); err != nil{
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return 
	}


	job, err := h.service.CreateJob(
		c.Request.Context(),
		req,
	)
	if err!= nil{
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})

		return 
	}

	c.JSON(http.StatusCreated, job)
}




func (h *Handler) GetJob(c *gin.Context) {
	id , err := uuid.Parse(c.Param("id"))
	if err != nil{
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid job id"})
		return 
	}

	job , err := h.service.GetJob(c.Request.Context(), id,)
	if err != nil{
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return 
	}
	c.JSON(http.StatusOK, job)
}


func (h *Handler) ListJobsByMedia(c *gin.Context){
	mediaID, err := uuid.Parse(c.Query("media_id"))
	if err != nil{
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid media id",
		})
		return 
	}
	jobs, err := h.service.ListJobsByMedia(c.Request.Context(), mediaID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, jobs)
}



func (h *Handler) DeleteJob(c *gin.Context){
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error":  "invalid job id"})
		return
	}
	if err := h.service.DeleteJob(c.Request.Context(), id,); err != nil{
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}