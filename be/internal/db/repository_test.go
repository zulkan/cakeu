package db

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestIsDuplicateEmail(t *testing.T) {
	t.Parallel()

	if isDuplicateEmail(nil) {
		t.Fatal("nil error must not be treated as a duplicate")
	}

	if isDuplicateEmail(errors.New("dial tcp 127.0.0.1:5433: connect: connection refused")) {
		t.Fatal("connection failures must not be reported as duplicate emails")
	}

	if !isDuplicateEmail(&pgconn.PgError{Code: "23505", ConstraintName: "users_email_key"}) {
		t.Fatal("unique violation 23505 must be treated as a duplicate")
	}

	if isDuplicateEmail(&pgconn.PgError{Code: "42P01"}) {
		t.Fatal("unrelated SQL errors must not be treated as duplicates")
	}
}
