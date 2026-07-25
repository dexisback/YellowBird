//contract between your backend and the outside world



//my db model is defined alr. our db model and api are now completely independent 
//tommorow if i add InternalSecret string to our db, api still remains same 




package project


import (
	"time"
	"github.com/google/uuid"
)


type CreateProjectRequest struct {
	Name    string     `json:"name"   binding:"required,max=255"`
	Description    string    `json:"description"`
}


type UpdateProjectRequest struct{
	Name     string   `json:"name"   binding:"omitempty, max=255"`
	Description   string  `json:"description"`
}


type ProjectResponse struct {
	ID      uuid.UUID     `json:"id"`
	Name    string     `json:"name"`
	Description   string   `json:"description"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}



