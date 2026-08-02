package project

import (
	"net/http" //statusCodes baby

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{
		service: service,
	}
}

func (h *Handler) CreateProject(c *gin.Context) {
	var req CreateProjectRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	ownerIDValue, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "missing authenticated user",
		})
		return
	}

	ownerID, ok := ownerIDValue.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "invalid authenticated user",
		})
		return
	}

	project, err := h.service.CreateProject(c.Request.Context(), ownerID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, project)
}

func (h *Handler) GetProject(c *gin.Context) {
	// id, err := uuid.Parse(c.Param("id"))

	// if err != nil {
	// 	c.JSON(http.StatusBadRequest, gin.H{
	// 		"error": "invalid project id",
	// 	})
	// 	return
	// }

	// project, err := h.service.GetProject(c.Request.Context(), id)
	// if err != nil {
	// 	c.JSON(http.StatusNotFound, gin.H{
	// 		"error": err.Error(),
	// 	})
	// 	return
	// }

	// c.JSON(http.StatusOK, project)
	ownerIDValue, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized , gin.H{"error": "unauthorised ownerid"})
		return 
	} 

	ownerID, ok := ownerIDValue.(uuid.UUID)

	if !ok{
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauth man"})
		return 
	}

	id , err := uuid.Parse(c.Param("id"))
	if err != nil{
		c.JSON(http.StatusBadRequest, gin.H{"error": "unable to parse id"})
		return 
	}

	project, err := h.service.GetProject(c.Request.Context(), ownerID, id)
	if err != nil{
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return 
	}
	c.JSON(http.StatusOK, project)
}

func (h *Handler) ListProjects(c *gin.Context) {
	ownerIDValue, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "missing authenticated user",
		})
		return
	}

	ownerID, ok := ownerIDValue.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "invalid authenticated user",
		})
		return
	}

	projects, err := h.service.ListProjects(c.Request.Context(), ownerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})

		return
	}

	c.JSON(http.StatusOK, projects)
}

func (h *Handler) UpdateProject(c *gin.Context) {
	ownerIDValue, exists := c.Get("userID")
	if !exists{
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authenticated user :("})
		return 
	}
	ownerID, ok := ownerIDValue.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authenticated user"})
		return 

	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	var req UpdateProjectRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}
	project, err := h.service.UpdateProject(c.Request.Context(), ownerID, id, req,)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, project)
}




func (h *Handler) DeleteProject(c *gin.Context) {
	ownerIDValue, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authenticated user"})
		return 
	}

	ownerID, ok := ownerIDValue.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authenticated user"})
		return 
	}


	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id",})
		return
	}

	if err := h.service.DeleteProject(c.Request.Context(), ownerID, id, ); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.Status(http.StatusNoContent)
}
