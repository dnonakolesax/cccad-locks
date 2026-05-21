package locks

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/dnonakolesax/cccad-locks/internal/auth"
	"github.com/dnonakolesax/cccad-locks/internal/model"
)

const (
	defaultLockTTL = 30 * time.Second
	minLockTTL     = time.Second
	maxLockTTL     = 5 * time.Minute
)

var (
	ErrLockNotFound = model.ErrLockNotFound
	ErrLockNotOwned = model.ErrLockNotOwned
)

type Repository interface {
	Acquire(ctx context.Context, lock *model.SketchLock, ttl time.Duration) (bool, *model.SketchLock, error)
	Refresh(ctx context.Context, sketchID, lockID, ownerUserID string, ttl time.Duration) (*model.SketchLock, error)
	Release(ctx context.Context, sketchID, lockID, ownerUserID string) error
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Acquire(
	ctx context.Context,
	sketchID string,
	request *model.AcquireLockRequest,
) (*model.AcquireLockResponse, error) {
	sketchID = strings.TrimSpace(sketchID)
	if sketchID == "" {
		return nil, errors.New("sketchID is required")
	}
	if request == nil {
		return nil, errors.New("request is required")
	}
	if len(request.Scope) == 0 {
		return nil, errors.New("scope is required")
	}
	request.Mode = strings.TrimSpace(request.Mode)
	if !isValidMode(request.Mode) {
		return nil, errors.New("mode must be edit or preview")
	}

	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, errors.New("authenticated user id is required")
	}

	ttl := normalizeTTL(request.TTLMS)
	expiresAt := time.Now().UTC().Add(ttl)
	lock := &model.SketchLock{
		ID:          newLockID(),
		SketchID:    sketchID,
		OwnerUserID: userID,
		Scope:       append([]byte(nil), request.Scope...),
		Mode:        request.Mode,
		ExpiresAt:   expiresAt.Format(time.RFC3339Nano),
	}

	granted, existing, err := s.repo.Acquire(ctx, lock, ttl)
	if err != nil {
		return nil, err
	}
	if granted {
		return &model.AcquireLockResponse{Granted: true, Lock: existing}, nil
	}
	if existing == nil {
		return nil, ErrLockNotFound
	}
	if existing.OwnerUserID == userID && existing.Mode == request.Mode {
		refreshed, err := s.repo.Refresh(ctx, sketchID, existing.ID, userID, ttl)
		if err != nil {
			return nil, err
		}
		return &model.AcquireLockResponse{Granted: true, Lock: refreshed}, nil
	}

	return &model.AcquireLockResponse{
		Granted: false,
		Conflict: &model.LockConflict{
			HolderUserID: existing.OwnerUserID,
			LockID:       existing.ID,
			Scope:        append([]byte(nil), existing.Scope...),
		},
	}, nil
}

func (s *Service) Refresh(
	ctx context.Context,
	sketchID string,
	lockID string,
	request *model.RefreshLockRequest,
) (*model.SketchLock, error) {
	sketchID = strings.TrimSpace(sketchID)
	lockID = strings.TrimSpace(lockID)
	if sketchID == "" || lockID == "" {
		return nil, errors.New("sketchID and lockID are required")
	}

	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, errors.New("authenticated user id is required")
	}

	ttlMS := 0
	if request != nil {
		ttlMS = request.TTLMS
	}

	return s.repo.Refresh(ctx, sketchID, lockID, userID, normalizeTTL(ttlMS))
}

func (s *Service) Release(ctx context.Context, sketchID string, lockID string) error {
	sketchID = strings.TrimSpace(sketchID)
	lockID = strings.TrimSpace(lockID)
	if sketchID == "" || lockID == "" {
		return errors.New("sketchID and lockID are required")
	}

	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return errors.New("authenticated user id is required")
	}

	return s.repo.Release(ctx, sketchID, lockID, userID)
}

func normalizeTTL(ttlMS int) time.Duration {
	if ttlMS <= 0 {
		return defaultLockTTL
	}

	ttl := time.Duration(ttlMS) * time.Millisecond
	if ttl < minLockTTL {
		return minLockTTL
	}
	if ttl > maxLockTTL {
		return maxLockTTL
	}

	return ttl
}

func isValidMode(mode string) bool {
	switch mode {
	case "edit", "preview":
		return true
	default:
		return false
	}
}

func newLockID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "lock_" + hex.EncodeToString([]byte(time.Now().UTC().Format(time.RFC3339Nano)))
	}

	return "lock_" + hex.EncodeToString(b[:])
}
