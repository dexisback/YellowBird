package rendition
import (
	"net/http"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)


type Handler struct {
	service Service 

}

func NewHandler(service Service) *Handler {
	return &Handler {
		service: service,
	}
}

func (h *Handler) CreateRendition(c *gin.Context){
	var req CreateRendtionRequest

	if err := c.ShouldBindJSON(&req); err != nil{
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return 
	}
	rendition, err := h.service.CreateRendition(c.Request.Context(), req,)
	if err != nil{
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return 
	}
	c.JSON(http.StatusCreated, rendition)
}

func (h *Handler) GetRendition(c *gin.Context){
	id, err := uuid.Parse(c.Param("id"))
	if err != nil{
		c.JSON(http.StatusBadRequest, gin.H{
			"error":"invalid rendition id",
		})
		return 
	}

	rendition, err := h.service.GetRendition(
		c.Request.Context(), id,
	)

	if err != nil{
		c.JSON(http.StatusNotFound,gin.H{
			"error": err.Error(),
		})
		return 
	}
	c.JSON(http.StatusOK, rendition)
}




func (h *Handler) ListRenditionsByMedia(c *gin.Context) {
	mediaID, err := uuid.Parse(c.Query("media_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid media id",
		})
		return
	}

	renditions, err := h.service.ListRenditionsByMedia(
		c.Request.Context(),
		mediaID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, renditions)
}

func (h *Handler) DeleteRendition(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid rendition id",
		})
		return
	}

	if err := h.service.DeleteRendition(
		c.Request.Context(),
		id,
	); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.Status(http.StatusNoContent)
}