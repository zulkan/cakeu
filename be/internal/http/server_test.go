package httpserver_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	httpserver "goskeleton/internal/http"
	"goskeleton/internal/service"
)

func TestHealthEndpointReturnsOKStatus(t *testing.T) {
	t.Parallel()

	handler := httpserver.NewHandler(service.NewHealth(nil), nil, nil, nil, nil, nil, "test-secret", nil)
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	request.RemoteAddr = "127.0.0.1:12345"
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d with body %q", recorder.Code, recorder.Body.String())
	}

	var response service.Status
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected JSON response, got error: %v", err)
	}

	if response.Status != "ok" {
		t.Fatalf("expected response status ok, got %q", response.Status)
	}
}

func TestHealthEndpointRejectsNonLocalRequests(t *testing.T) {
	t.Parallel()

	handler := httpserver.NewHandler(service.NewHealth(nil), nil, nil, nil, nil, nil, "test-secret", nil)
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	request.RemoteAddr = "203.0.113.10:12345"
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected status 403 for non-local request, got %d with body %q", recorder.Code, recorder.Body.String())
	}
}
