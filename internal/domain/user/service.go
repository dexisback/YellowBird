//we register users, hash passwords using bcrypt, prevent emails, verify creds during login 




//internal/domain/user/service.go 



package user 

import (
	"context"
	"errors"
	"golang.org/x/crypto/bcrypt"
	"github.com/google/uuid"
	"gorm.io/gorm"
)


type Service interface{
	RegisterUser(ctx context.Context , req RegisterUserRequest) (*UserResponse, error)
	LoginUser(ctx context.Context, req LoginUserRequest) (*UserResponse, error)
	GetUser(ctx context.Context) ([]UserResponse, error)
	ListUsers(ctx context.Context) ([]UserResponse, error)
	DeleteUsers(ctx context.Context, id uuid.UUID) error 
}



type service struct {
	repository Repository
}


func NewService(repository Repository) Service {
	return &service{
		repository: repository, 
	}
}

func (s *service) RegisterUser(ctx context.Context, req RegisterUserRequest) (*UserResponse, error){
	_, err := s.repository.GetByEmail((ctx, req.Email))
	if err == nil {
		return nil, errors.New("email already exists")
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	hashedPassword, err := bcrypt.GenerateFromPassword(
		[]byte (req.Password),
		bcrypt.DefaultCost
	)

	if err != nil{
		return nil, err
	}

	user := &User{
		Name: req.Name,
		Email : req.Email,
		PasswordHash: string(hashedPassword),
	}

	if err := s.repository.Create(ctx, user); err != nil{
		return nil, err 
	}

	return &UserResponse{
		ID: user.ID,
		Name: user.Name,
		Email: user.Email,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt.
	}, nil
}


