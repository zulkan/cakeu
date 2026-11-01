package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"goskeleton/internal/model"
	"goskeleton/internal/service"
)

type RecurringService interface {
	Create(ctx context.Context, userID uuid.UUID, req model.CreateRecurringRequest) (*model.RecurringRule, error)
	List(ctx context.Context, userID uuid.UUID) ([]model.RecurringRule, error)
	Update(ctx context.Context, userID, id uuid.UUID, req model.UpdateRecurringRequest) (*model.RecurringRule, error)
	Delete(ctx context.Context, userID, id uuid.UUID) error
	RunNow(ctx context.Context, userID, id uuid.UUID) (int, error)
}

func HandleListRecurring(recurring RecurringService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := userIDFromContext(r.Context())
		if err != nil {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		rules, err := recurring.List(r.Context(), userID)
		if err != nil {
			respondRecurringError(w, err)
			return
		}

		respondJSON(w, http.StatusOK, rules)
	}
}

func HandleCreateRecurring(recurring RecurringService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := userIDFromContext(r.Context())
		if err != nil {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		var req model.CreateRecurringRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		rule, err := recurring.Create(r.Context(), userID, req)
		if err != nil {
			respondRecurringError(w, err)
			return
		}

		respondJSON(w, http.StatusCreated, rule)
	}
}

func HandleUpdateRecurring(recurring RecurringService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, id, ok := recurringPathIDs(w, r)
		if !ok {
			return
		}

		var req model.UpdateRecurringRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		rule, err := recurring.Update(r.Context(), userID, id, req)
		if err != nil {
			respondRecurringError(w, err)
			return
		}

		respondJSON(w, http.StatusOK, rule)
	}
}

func HandleDeleteRecurring(recurring RecurringService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, id, ok := recurringPathIDs(w, r)
		if !ok {
			return
		}

		if err := recurring.Delete(r.Context(), userID, id); err != nil {
			respondRecurringError(w, err)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func HandleRunRecurringNow(recurring RecurringService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, id, ok := recurringPathIDs(w, r)
		if !ok {
			return
		}

		posted, err := recurring.RunNow(r.Context(), userID, id)
		if err != nil {
			respondRecurringError(w, err)
			return
		}

		respondJSON(w, http.StatusOK, map[string]int{"posted": posted})
	}
}

func recurringPathIDs(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	userID, err := userIDFromContext(r.Context())
	if err != nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return uuid.Nil, uuid.Nil, false
	}

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid id")
		return uuid.Nil, uuid.Nil, false
	}

	return userID, id, true
}

func respondRecurringError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrRecurringNotFound):
		respondError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, service.ErrInvalidFrequency),
		errors.Is(err, service.ErrInvalidRecurringAmount),
		errors.Is(err, service.ErrInvalidInterval),
		errors.Is(err, service.ErrInvalidNextRun),
		errors.Is(err, service.ErrInvalidType),
		errors.Is(err, service.ErrAccountNotFound),
		errors.Is(err, service.ErrCategoryNotFound):
		respondError(w, http.StatusBadRequest, err.Error())
	default:
		respondError(w, http.StatusInternalServerError, "internal server error")
	}
}
