package user

import (
	"time"
	"github.com/google/uuid"
)


type User struct {
	ID      uuid.UUID    `gorm:"type:uuid;default: gen_random_uuid();primaryKey"`
	Name     string       `gorm:"size:100;not null"`
	Email    string     `gorm:"size:255;uniqueIndex;not null"`
	PasswordHash    string     `gorm:"size:255;not null"`


	CreatedAt    time.Time 
	UpdatedAt     time.Time 
}


