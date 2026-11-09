package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/jtc/go-users-api/internal/domain"
	"github.com/jtc/go-users-api/internal/httpx"
)

type userService interface {
	Create(ctx context.Context, in domain.CreateUserInput) (*domain.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	List(ctx context.Context, limit, offset int) (*domain.UserPage, error)
	Update(ctx context.Context, id uuid.UUID, in domain.UpdateUserInput) (*domain.User, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type UserHandler struct {
	svc userService
	log *slog.Logger
}

func NewUserHandler(svc userService, log *slog.Logger) *UserHandler {
	return &UserHandler{svc: svc, log: log}
}

func (h *UserHandler) Routes() chi.Router {
	r := chi.NewRouter()

	r.Post("/", h.create)
	r.Get("/", h.list)

	r.Route("/{id}", func(r chi.Router) {
		r.Get("/", h.get)
		r.Put("/", h.update)
		r.Delete("/", h.delete)
	})

	return r
}

type createUserRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type updateUserRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type userResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type listUsersResponse struct {
	Data   []userResponse `json:"data"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
}

func (h *UserHandler) create(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		h.writeDecodeError(w, err)
		return
	}

	user, err := h.svc.Create(r.Context(), domain.CreateUserInput{
		Name:  req.Name,
		Email: req.Email,
	})
	if err != nil {
		h.writeError(w, err)
		return
	}

	w.Header().Set("Location", "/api/v1/users/"+user.ID.String())
	httpx.JSON(w, http.StatusCreated, toResponse(user))
}

func (h *UserHandler) get(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	user, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		h.writeError(w, err)
		return
	}

	httpx.JSON(w, http.StatusOK, toResponse(user))
}

func (h *UserHandler) list(w http.ResponseWriter, r *http.Request) {
	limit, offset, err := parsePagination(r)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	page, err := h.svc.List(r.Context(), limit, offset)
	if err != nil {
		h.writeError(w, err)
		return
	}

	data := make([]userResponse, len(page.Users))
	for i := range page.Users {
		data[i] = toResponse(&page.Users[i])
	}

	httpx.JSON(w, http.StatusOK, listUsersResponse{
		Data:   data,
		Limit:  page.Limit,
		Offset: page.Offset,
	})
}

func (h *UserHandler) update(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	var req updateUserRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		h.writeDecodeError(w, err)
		return
	}

	user, err := h.svc.Update(r.Context(), id, domain.UpdateUserInput{
		Name:  req.Name,
		Email: req.Email,
	})
	if err != nil {
		h.writeError(w, err)
		return
	}

	httpx.JSON(w, http.StatusOK, toResponse(user))
}

func (h *UserHandler) delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.svc.Delete(r.Context(), id); err != nil {
		h.writeError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *UserHandler) writeDecodeError(w http.ResponseWriter, err error) {
	var decodeErr *httpx.DecodeError
	if errors.As(err, &decodeErr) {
		httpx.Error(w, decodeErr.Status, decodeErr.Message)
		return
	}

	h.log.Error("unhandled decode error", "error", err)
	httpx.Error(w, http.StatusBadRequest, "the request body is invalid")
}

func (h *UserHandler) writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrValidation):
		httpx.Error(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, domain.ErrNotFound):
		httpx.Error(w, http.StatusNotFound, "user not found")
	case errors.Is(err, domain.ErrConflict):
		httpx.Error(w, http.StatusConflict, "email already in use")
	default:
		h.log.Error("unhandled handler error", "error", err)
		httpx.Error(w, http.StatusInternalServerError, "internal server error")
	}
}

func parsePagination(r *http.Request) (limit, offset int, err error) {
	if raw := r.URL.Query().Get("limit"); raw != "" {
		limit, err = strconv.Atoi(raw)
		if err != nil || limit < 0 {
			return 0, 0, errors.New("query parameter 'limit' must be zero or a positive integer")
		}
	}

	if raw := r.URL.Query().Get("offset"); raw != "" {
		offset, err = strconv.Atoi(raw)
		if err != nil || offset < 0 {
			return 0, 0, errors.New("query parameter 'offset' must be zero or a positive integer")
		}
	}

	return limit, offset, nil
}

func parseID(r *http.Request) (uuid.UUID, error) {
	raw := chi.URLParam(r, "id")

	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, errors.New("invalid user id")
	}
	return id, nil
}

func toResponse(u *domain.User) userResponse {
	return userResponse{
		ID:        u.ID.String(),
		Name:      u.Name,
		Email:     u.Email,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}
