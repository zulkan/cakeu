package service_test

import (
	"context"
	"errors"
	"testing"

	"goskeleton/internal/service"
)

type fakePinger struct {
	err error
}

func (f fakePinger) Ping(context.Context) error {
	return f.err
}

func TestHealthCheckReportsDatabaseUp(t *testing.T) {
	t.Parallel()

	health := service.NewHealth(fakePinger{})
	status := health.Check(context.Background())

	if status.Status != "ok" {
		t.Fatalf("expected overall status ok, got %q", status.Status)
	}

	if status.Database != "up" {
		t.Fatalf("expected database status up, got %q", status.Database)
	}
}

func TestHealthCheckReportsDatabaseDown(t *testing.T) {
	t.Parallel()

	health := service.NewHealth(fakePinger{err: errors.New("db down")})
	status := health.Check(context.Background())

	if status.Status != "degraded" {
		t.Fatalf("expected overall status degraded, got %q", status.Status)
	}

	if status.Database != "down" {
		t.Fatalf("expected database status down, got %q", status.Database)
	}
}
