package user

import (
	"context"
	"errors"
	"github.com/dexisback/YellowBird/internal/auth"
	"golang.org/x/crypto/bcrypt"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Service interface {
	RegisterUser(ctx context.Context, req RegisterUserRequest) (*UserResponse, error)
	LoginUser(ctx context.Context, req LoginUserRequest) (*UserResponse, error)
	GetUser(ctx context.Context, id uuid.UUID) (*UserResponse, error)
	ListUsers(ctx context.Context) ([]UserResponse, error)
	DeleteUser(ctx context.Context, id uuid.UUID) error
}

type service struct {
	repository Repository
	jwtService   *auth.JWTService
}

func NewService(repository Repository, jwtService *auth.JWTService) Service {
	return &service{
		repository: repository,
		jwtService: jwtService,
	}
}

func (s *service) RegisterUser(ctx context.Context, req RegisterUserRequest) (*UserResponse, error) {
	_, err := s.repository.GetByEmail(ctx, req.Email)
	if err == nil {
		return nil, errors.New("email already exists")
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	hashedPassword, err := bcrypt.GenerateFromPassword(
		[]byte(req.Password),
		bcrypt.DefaultCost,
	)
	if err != nil {
		return nil, err
	}

	user := &User{
		Name:         req.Name,
		Email:        req.Email,
		PasswordHash: string(hashedPassword),
	}

	if err := s.repository.Create(ctx, user); err != nil {
		return nil, err
	}

	return &UserResponse{
		ID:        user.ID,
		Name:      user.Name,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}, nil
}

func (s *service) LoginUser(ctx context.Context, req LoginUserRequest) (*UserResponse, error) {
	user, err := s.repository.GetByEmail(ctx, req.Email)
	if err != nil {
		return nil, errors.New("invalid email or password")
	}

	err = bcrypt.CompareHashAndPassword(
		[]byte(user.PasswordHash),
		[]byte(req.Password),
	)
	if err != nil {
		return nil, errors.New("invalid email or password")
	}

	return &UserResponse{
		ID:        user.ID,
		Name:      user.Name,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}, nil
}

func (s *service) GetUser(ctx context.Context, id uuid.UUID) (*UserResponse, error) {
	user, err := s.repository.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return &UserResponse{
		ID:        user.ID,
		Name:      user.Name,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}, nil
}

func (s *service) ListUsers(ctx context.Context) ([]UserResponse, error) {
	users, err := s.repository.List(ctx)
	if err != nil {
		return nil, err
	}

	response := make([]UserResponse, 0, len(users))

	for _, user := range users {
		response = append(response, UserResponse{
			ID:        user.ID,
			Name:      user.Name,
			Email:     user.Email,
			CreatedAt: user.CreatedAt,
			UpdatedAt: user.UpdatedAt,
		})
	}

	return response, nil
}

func (s *service) DeleteUser(ctx context.Context, id uuid.UUID) error {
	return s.repository.Delete(ctx, id)
}