package user 


import (
	"time"
	"github.com/google/uuid"
)


type RegisterUserRequest struct {
	Name    string    `json:"name" binding:"required, min=2, max=100"`
	Email   string     `json:"email" binding:"required, email"`
	Password   string     `json:"password" binding:"required, min=8, max=72"`

}


type LoginUserRequest struct {
	Email    string   `json:"email" binding:"required,email"`
	Password    string   `json:"password" binding:"required"`
}


type UserResponse struct{
	ID      uuid.UUID    `json:"id"`
	Name    string    `json:"name"`
	Email    string    `json:"email"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}





