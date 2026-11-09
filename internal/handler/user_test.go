package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/jtc/go-users-api/internal/domain"
)

type fakeUserService struct {
	createFn func(ctx context.Context, in domain.CreateUserInput) (*domain.User, error)
	getFn    func(ctx context.Context, id uuid.UUID) (*domain.User, error)
	listFn   func(ctx context.Context, limit, offset int) (*domain.UserPage, error)
	updateFn func(ctx context.Context, id uuid.UUID, in domain.UpdateUserInput) (*domain.User, error)
	deleteFn func(ctx context.Context, id uuid.UUID) error
}

func (f *fakeUserService) Create(ctx context.Context, in domain.CreateUserInput) (*domain.User, error) {
	if f.createFn == nil {
		return &domain.User{}, nil
	}
	return f.createFn(ctx, in)
}

func (f *fakeUserService) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	if f.getFn == nil {
		return &domain.User{ID: id}, nil
	}
	return f.getFn(ctx, id)
}

func (f *fakeUserService) List(ctx context.Context, limit, offset int) (*domain.UserPage, error) {
	if f.listFn == nil {
		return &domain.UserPage{Limit: limit, Offset: offset}, nil
	}
	return f.listFn(ctx, limit, offset)
}

func (f *fakeUserService) Update(ctx context.Context, id uuid.UUID, in domain.UpdateUserInput) (*domain.User, error) {
	if f.updateFn == nil {
		return &domain.User{ID: id}, nil
	}
	return f.updateFn(ctx, id, in)
}

func (f *fakeUserService) Delete(ctx context.Context, id uuid.UUID) error {
	if f.deleteFn == nil {
		return nil
	}
	return f.deleteFn(ctx, id)
}

func newTestRouter(svc userService) http.Handler {
	h := NewUserHandler(svc, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return h.Routes()
}

func doRequest(t *testing.T, router http.Handler, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()

	var reader io.Reader
	if body != "" {
		reader = bytes.NewBufferString(body)
	}

	req := httptest.NewRequest(method, target, reader)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestCreateUser(t *testing.T) {
	id := uuid.New()
	svc := &fakeUserService{
		createFn: func(_ context.Context, in domain.CreateUserInput) (*domain.User, error) {
			return &domain.User{ID: id, Name: in.Name, Email: in.Email}, nil
		},
	}

	rec := doRequest(t, newTestRouter(svc), http.MethodPost, "/",
		`{"name":"Ada Lovelace","email":"ada@example.com"}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	if got, want := rec.Header().Get("Location"), "/api/v1/users/"+id.String(); got != want {
		t.Errorf("Location = %q, want %q", got, want)
	}

	var resp userResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ID != id.String() {
		t.Errorf("id = %q, want %q", resp.ID, id.String())
	}
}

func TestCreateUserRejectsBadBody(t *testing.T) {
	tests := map[string]struct {
		body    string
		message string
	}{
		"empty body":    {"", "the request body is required"},
		"malformed":     {`{"name":`, "the request body contains malformed JSON"},
		"unknown field": {`{"name":"Ada","email":"ada@example.com","role":"admin"}`, "the request body contains an unknown field"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			rec := doRequest(t, newTestRouter(&fakeUserService{}), http.MethodPost, "/", tc.body)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
			}

			var resp httpxError
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if resp.Error != tc.message {
				t.Errorf("error = %q, want %q", resp.Error, tc.message)
			}
		})
	}
}

func TestCreateUserMapsServiceErrors(t *testing.T) {
	tests := map[string]struct {
		err        error
		wantStatus int
	}{
		"validation": {domain.ErrValidation, http.StatusBadRequest},
		"conflict":   {domain.ErrConflict, http.StatusConflict},
		"not found":  {domain.ErrNotFound, http.StatusNotFound},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			svc := &fakeUserService{
				createFn: func(context.Context, domain.CreateUserInput) (*domain.User, error) {
					return nil, tc.err
				},
			}

			rec := doRequest(t, newTestRouter(svc), http.MethodPost, "/",
				`{"name":"Ada","email":"ada@example.com"}`)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
		})
	}
}

func TestGetUserInvalidID(t *testing.T) {
	rec := doRequest(t, newTestRouter(&fakeUserService{}), http.MethodGet, "/not-a-uuid", "")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestGetUserNotFound(t *testing.T) {
	svc := &fakeUserService{
		getFn: func(context.Context, uuid.UUID) (*domain.User, error) {
			return nil, domain.ErrNotFound
		},
	}

	rec := doRequest(t, newTestRouter(svc), http.MethodGet, "/"+uuid.NewString(), "")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestListUsersInvalidPagination(t *testing.T) {
	tests := []string{"/?limit=abc", "/?limit=-1", "/?offset=-1"}

	for _, target := range tests {
		t.Run(target, func(t *testing.T) {
			rec := doRequest(t, newTestRouter(&fakeUserService{}), http.MethodGet, target, "")

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestListUsers(t *testing.T) {
	svc := &fakeUserService{
		listFn: func(_ context.Context, limit, offset int) (*domain.UserPage, error) {
			return &domain.UserPage{
				Users:  []domain.User{{ID: uuid.New(), Name: "Ada", Email: "ada@example.com"}},
				Limit:  limit,
				Offset: offset,
			}, nil
		},
	}

	rec := doRequest(t, newTestRouter(svc), http.MethodGet, "/?limit=5&offset=2", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp listUsersResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Limit != 5 || resp.Offset != 2 {
		t.Errorf("limit/offset = %d/%d, want 5/2", resp.Limit, resp.Offset)
	}
	if len(resp.Data) != 1 {
		t.Errorf("len(data) = %d, want 1", len(resp.Data))
	}
}

func TestUpdateUser(t *testing.T) {
	id := uuid.New()
	svc := &fakeUserService{
		updateFn: func(_ context.Context, gotID uuid.UUID, in domain.UpdateUserInput) (*domain.User, error) {
			return &domain.User{ID: gotID, Name: in.Name, Email: in.Email}, nil
		},
	}

	rec := doRequest(t, newTestRouter(svc), http.MethodPut, "/"+id.String(),
		`{"name":"Ada Byron","email":"ada@example.com"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestDeleteUser(t *testing.T) {
	rec := doRequest(t, newTestRouter(&fakeUserService{}), http.MethodDelete, "/"+uuid.NewString(), "")

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

type httpxError struct {
	Error string `json:"error"`
}
