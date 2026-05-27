package sql

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type fakeRows struct {
	canceled *bool
	closed   bool
}

func (r *fakeRows) Close() {
	r.closed = true
}

func (r *fakeRows) Err() error {
	if *r.canceled {
		return context.Canceled
	}
	return nil
}

func (r *fakeRows) CommandTag() pgconn.CommandTag {
	return pgconn.CommandTag{}
}

func (r *fakeRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}

func (r *fakeRows) Next() bool {
	r.Close()
	return false
}

func (r *fakeRows) Scan(_ ...any) error {
	return nil
}

func (r *fakeRows) Values() ([]any, error) {
	return nil, nil
}

func (r *fakeRows) RawValues() [][]byte {
	return nil
}

func (r *fakeRows) Conn() *pgx.Conn {
	return nil
}

func TestPGXResponseCloseChecksRowsErrBeforeCancel(t *testing.T) {
	canceled := false
	rows := &fakeRows{canceled: &canceled}
	response := &PGXResponse{
		rows: rows,
		cancel: func() {
			canceled = true
		},
	}

	if err := response.Close(); err != nil {
		t.Fatalf("Close() error = %v, want nil", err)
	}
	if !rows.closed {
		t.Fatal("Close() did not close rows")
	}
	if !canceled {
		t.Fatal("Close() did not cancel query context")
	}
}
