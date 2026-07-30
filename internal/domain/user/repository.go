package user 

import (
	"context"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repository interface {
	Create(ctx context.Context, user *User) error
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
	GetByEmail(ctx context.Context, email string ) (*User, error)
	List(ctx context.Context, email string ) (*User, error)
	Update(ctx context.Context, user *User) error
	Delete(ctx context.Context, id uuid.UUID) error
}




type repository struct {
	db *gorm.DB
}


func NewRepository(db *gorm.DB) Repository {
	return &repository{
		db: db,
	}
}



func (r *repository) Create(ctx context.Context, user *User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *repository) GetByID(ctx context.Context, id uuid.UUID) (*User, error){
	var user User 


	err := r.db.WithContext(ctx).
		First(&user, "id = ?", id).
		Error 

		if err != nil{
			return nil, err
		}

		return &user, nil

}


//get by email, list, update, delete 
func (r *repository) GetByEmail(ctx context.Context, email string ) (*User, error){
	var user User 

	err := r.db.WithContext(ctx).Where("email = ?", email).First(&user).Error

	if err != nil{
		return nil, err
	}

	return &user, nil
}

func (r *repository) List(ctx context.Context) ([]User, error){
	var users []User

	err := r.db.WithContext(ctx).Find(&users).Error
	if err != nil{
		return nil, err
	}

	return users, nil
}



func (r *repository) Update(ctx context.Context, user *User) error {
	return r.db.WithContext(ctx).Save(user).Error
}


func (r *repository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&UserP{}, "id= ?", id).Error()
}


