package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/jtc/go-users-api/internal/domain"
)

type fakeUserRepository struct {
	users map[uuid.UUID]*domain.User
}

func newFakeRepo() *fakeUserRepository {
	return &fakeUserRepository{users: make(map[uuid.UUID]*domain.User)}
}

func (f *fakeUserRepository) Create(_ context.Context, user *domain.User) error {
	f.users[user.ID] = user
	return nil
}

func (f *fakeUserRepository) GetByID(_ context.Context, id uuid.UUID) (*domain.User, error) {
	user, ok := f.users[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return user, nil
}

func (f *fakeUserRepository) List(_ context.Context, limit, offset int) ([]domain.User, error) {
	return nil, nil
}

func (f *fakeUserRepository) Update(_ context.Context, user *domain.User) error {
	f.users[user.ID] = user
	return nil
}

func (f *fakeUserRepository) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := f.users[id]; !ok {
		return domain.ErrNotFound
	}
	delete(f.users, id)
	return nil
}

func TestCreateValidatesInput(t *testing.T) {
	svc := NewUserService(newFakeRepo())

	tests := map[string]domain.CreateUserInput{
		"empty name":  {Name: "  ", Email: "a@example.com"},
		"empty email": {Name: "Ada", Email: ""},
		"bad email":   {Name: "Ada", Email: "not-an-email"},
	}

	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := svc.Create(context.Background(), input)
			if !errors.Is(err, domain.ErrValidation) {
				t.Fatalf("expected ErrValidation, got %v", err)
			}
		})
	}
}

func TestCreateNormalizesEmail(t *testing.T) {
	repo := newFakeRepo()
	svc := NewUserService(repo)

	user, err := svc.Create(context.Background(), domain.CreateUserInput{
		Name:  "  Ada Lovelace  ",
		Email: "  ADA@Example.COM ",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.Name != "Ada Lovelace" {
		t.Errorf("name = %q, want %q", user.Name, "Ada Lovelace")
	}
	if user.Email != "ada@example.com" {
		t.Errorf("email = %q, want %q", user.Email, "ada@example.com")
	}
}

func TestGetByIDNotFound(t *testing.T) {
	svc := NewUserService(newFakeRepo())

	_, err := svc.GetByID(context.Background(), uuid.New())
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestListClampsPagination(t *testing.T) {
	svc := NewUserService(newFakeRepo())

	page, err := svc.List(context.Background(), 0, -5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if page.Limit != defaultLimit {
		t.Errorf("limit = %d, want %d", page.Limit, defaultLimit)
	}
	if page.Offset != 0 {
		t.Errorf("offset = %d, want 0", page.Offset)
	}

	page, err = svc.List(context.Background(), 1000, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if page.Limit != maxLimit {
		t.Errorf("limit = %d, want %d", page.Limit, maxLimit)
	}
}
