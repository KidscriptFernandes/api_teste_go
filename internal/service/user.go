package service

import (
	"context"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/jtc/go-users-api/internal/domain"
	"github.com/jtc/go-users-api/internal/repository"
)

const (
	defaultLimit = 20
	maxLimit     = 100
)

type UserService struct {
	repo repository.UserRepository
}

func NewUserService(repo repository.UserRepository) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) Create(ctx context.Context, in domain.CreateUserInput) (*domain.User, error) {
	name, email, err := validate(in.Name, in.Email)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	user := &domain.User{
		ID:        uuid.New(),
		Name:      name,
		Email:     email,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.repo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("service: create user: %w", err)
	}
	return user, nil
}

func (s *UserService) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("service: get user: %w", err)
	}
	return user, nil
}

func (s *UserService) List(ctx context.Context, limit, offset int) (*domain.UserPage, error) {
	limit, offset = normalizePagination(limit, offset)

	users, err := s.repo.List(ctx, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("service: list users: %w", err)
	}

	return &domain.UserPage{Users: users, Limit: limit, Offset: offset}, nil
}

func (s *UserService) Update(ctx context.Context, id uuid.UUID, in domain.UpdateUserInput) (*domain.User, error) {
	name, email, err := validate(in.Name, in.Email)
	if err != nil {
		return nil, err
	}

	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("service: update user: %w", err)
	}

	user.Name = name
	user.Email = email
	user.UpdatedAt = time.Now().UTC()

	if err := s.repo.Update(ctx, user); err != nil {
		return nil, fmt.Errorf("service: update user: %w", err)
	}
	return user, nil
}

func (s *UserService) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("service: delete user: %w", err)
	}
	return nil
}

func validate(name, email string) (string, string, error) {
	name = strings.TrimSpace(name)
	email = strings.ToLower(strings.TrimSpace(email))

	if name == "" {
		return "", "", fmt.Errorf("%w: name is required", domain.ErrValidation)
	}
	if len(name) > 120 {
		return "", "", fmt.Errorf("%w: name must be at most 120 characters", domain.ErrValidation)
	}
	if email == "" {
		return "", "", fmt.Errorf("%w: email is required", domain.ErrValidation)
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return "", "", fmt.Errorf("%w: email is invalid", domain.ErrValidation)
	}

	return name, email, nil
}

func normalizePagination(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}
