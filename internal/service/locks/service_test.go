package locks

import (
	"context"
	"testing"
	"time"

	"github.com/dnonakolesax/cccad-locks/internal/auth"
	"github.com/dnonakolesax/cccad-locks/internal/model"
)

type repositoryStub struct {
	acquireGranted bool
	acquireLock    *model.SketchLock
	acquireTTL     time.Duration
	refreshLock    *model.SketchLock
	releaseUserID  string
}

func (r *repositoryStub) Acquire(
	_ context.Context,
	lock *model.SketchLock,
	ttl time.Duration,
) (bool, *model.SketchLock, error) {
	r.acquireTTL = ttl
	if r.acquireLock == nil {
		r.acquireLock = lock
	}
	return r.acquireGranted, r.acquireLock, nil
}

func (r *repositoryStub) Refresh(
	_ context.Context,
	_ string,
	_ string,
	ownerUserID string,
	_ time.Duration,
) (*model.SketchLock, error) {
	r.releaseUserID = ownerUserID
	return r.refreshLock, nil
}

func (r *repositoryStub) Release(_ context.Context, _ string, _ string, ownerUserID string) error {
	r.releaseUserID = ownerUserID
	return nil
}

func TestServiceAcquireUsesAuthenticatedUser(t *testing.T) {
	repo := &repositoryStub{acquireGranted: true}
	service := NewService(repo)
	ctx := auth.ContextWithUserID(context.Background(), "user-id")

	response, err := service.Acquire(ctx, "sketch-id", &model.AcquireLockRequest{
		Scope: []byte(`{"entityId":"p1","type":"entity"}`),
		Mode:  "edit",
		TTLMS: 2500,
	})
	if err != nil {
		t.Fatalf("Acquire returned error: %v", err)
	}
	if !response.Granted {
		t.Fatal("Acquire response was not granted")
	}
	if repo.acquireLock.OwnerUserID != "user-id" {
		t.Fatalf("Acquire owner = %q, want %q", repo.acquireLock.OwnerUserID, "user-id")
	}
	if repo.acquireTTL != 2500*time.Millisecond {
		t.Fatalf("Acquire ttl = %s, want 2.5s", repo.acquireTTL)
	}
}

func TestServiceAcquireReturnsConflictForAnotherOwner(t *testing.T) {
	repo := &repositoryStub{
		acquireGranted: false,
		acquireLock: &model.SketchLock{
			ID:          "lock-id",
			OwnerUserID: "other-user",
			Scope:       []byte(`{"entityId":"p1","type":"entity"}`),
			Mode:        "edit",
		},
	}
	service := NewService(repo)
	ctx := auth.ContextWithUserID(context.Background(), "user-id")

	response, err := service.Acquire(ctx, "sketch-id", &model.AcquireLockRequest{
		Scope: []byte(`{"entityId":"p1","type":"entity"}`),
		Mode:  "edit",
	})
	if err != nil {
		t.Fatalf("Acquire returned error: %v", err)
	}
	if response.Granted {
		t.Fatal("Acquire response was granted")
	}
	if response.Conflict == nil || response.Conflict.HolderUserID != "other-user" {
		t.Fatalf("Acquire conflict = %#v, want holder other-user", response.Conflict)
	}
}

func TestServiceAcquireRequiresAuthenticatedUser(t *testing.T) {
	service := NewService(&repositoryStub{})

	if _, err := service.Acquire(context.Background(), "sketch-id", &model.AcquireLockRequest{
		Scope: []byte(`{"entityId":"p1","type":"entity"}`),
		Mode:  "edit",
	}); err == nil {
		t.Fatal("Acquire returned nil error without authenticated user")
	}
}
