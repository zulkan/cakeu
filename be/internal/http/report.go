package httpserver

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"goskeleton/internal/model"
	"goskeleton/internal/service"
)

type ReportService interface {
	Cashflow(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]model.CashflowPoint, error)
	Categories(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]model.CategoryTotal, error)
	Merchants(ctx context.Context, userID uuid.UUID, from, to time.Time, limit int) ([]model.MerchantTotal, error)
}

func HandleCashflowReport(reports ReportService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, from, to, ok := reportRequestContext(w, r)
		if !ok {
			return
		}

		points, err := reports.Cashflow(r.Context(), userID, from, to)
		if err != nil {
			respondReportError(w, err)
			return
		}

		respondJSON(w, http.StatusOK, points)
	}
}

func HandleCategoryReport(reports ReportService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, from, to, ok := reportRequestContext(w, r)
		if !ok {
			return
		}

		totals, err := reports.Categories(r.Context(), userID, from, to)
		if err != nil {
			respondReportError(w, err)
			return
		}

		respondJSON(w, http.StatusOK, totals)
	}
}

func HandleMerchantReport(reports ReportService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, from, to, ok := reportRequestContext(w, r)
		if !ok {
			return
		}

		limit := 10
		if value := r.URL.Query().Get("limit"); value != "" {
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed < 1 {
				respondError(w, http.StatusBadRequest, "invalid limit")
				return
			}
			limit = parsed
		}

		totals, err := reports.Merchants(r.Context(), userID, from, to, limit)
		if err != nil {
			respondReportError(w, err)
			return
		}

		respondJSON(w, http.StatusOK, totals)
	}
}

func reportRequestContext(w http.ResponseWriter, r *http.Request) (uuid.UUID, time.Time, time.Time, bool) {
	userID, err := userIDFromContext(r.Context())
	if err != nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return uuid.Nil, time.Time{}, time.Time{}, false
	}

	from, err := time.Parse("2006-01-02", r.URL.Query().Get("from"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid from date")
		return uuid.Nil, time.Time{}, time.Time{}, false
	}

	to, err := time.Parse("2006-01-02", r.URL.Query().Get("to"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid to date")
		return uuid.Nil, time.Time{}, time.Time{}, false
	}

	return userID, from, to, true
}

func respondReportError(w http.ResponseWriter, err error) {
	if errors.Is(err, service.ErrInvalidRange) {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondError(w, http.StatusInternalServerError, "internal server error")
}
