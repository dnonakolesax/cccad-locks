package locks

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"time"

	dbredis "github.com/dnonakolesax/cccad-locks/internal/db/redis"
	"github.com/dnonakolesax/cccad-locks/internal/model"
	"github.com/mailru/easyjson"
	"github.com/redis/go-redis/v9"
)

const lockKeyPrefix = "cccad:locks"

var refreshScript = redis.NewScript(`
if redis.call("get", KEYS[2]) ~= ARGV[3] then
	return 0
end
if redis.call("get", KEYS[1]) ~= ARGV[1] then
	return 0
end
redis.call("set", KEYS[1], ARGV[2], "PX", ARGV[4])
redis.call("set", KEYS[2], ARGV[3], "PX", ARGV[4])
return 1
`)

var releaseScript = redis.NewScript(`
if redis.call("get", KEYS[2]) ~= ARGV[2] then
	return 0
end
if redis.call("get", KEYS[1]) ~= ARGV[1] then
	redis.call("del", KEYS[2])
	return 0
end
redis.call("del", KEYS[1], KEYS[2])
return 1
`)

type Repository struct {
	redis *dbredis.Client
}

func NewRepository(redisClient *dbredis.Client) *Repository {
	return &Repository{redis: redisClient}
}

func (r *Repository) Acquire(
	ctx context.Context,
	lock *model.SketchLock,
	ttl time.Duration,
) (bool, *model.SketchLock, error) {
	if lock == nil {
		return false, nil, errors.New("lock is required")
	}

	scopeKey, err := scopeLockKey(lock.SketchID, lock.Scope)
	if err != nil {
		return false, nil, err
	}
	idKey := lockIDKey(lock.SketchID, lock.ID)

	body, err := easyjson.Marshal(lock)
	if err != nil {
		return false, nil, fmt.Errorf("marshal lock: %w", err)
	}

	client := r.client()
	rctx, cancel := r.context(ctx)
	defer cancel()

	for attempt := 0; attempt < 2; attempt++ {
		acquired, err := client.SetNX(rctx, scopeKey, string(body), ttl).Result()
		if err != nil {
			return false, nil, fmt.Errorf("acquire lock: %w", err)
		}
		if !acquired {
			_, existing, getErr := r.getByScopeKey(ctx, scopeKey)
			if errors.Is(getErr, model.ErrLockNotFound) {
				continue
			}
			if getErr != nil {
				return false, nil, getErr
			}
			return false, existing, nil
		}

		if err := client.Set(rctx, idKey, scopeKey, ttl).Err(); err != nil {
			_ = client.Del(rctx, scopeKey).Err()
			return false, nil, fmt.Errorf("index lock: %w", err)
		}

		return true, lock, nil
	}

	return false, nil, model.ErrLockNotFound
}

func (r *Repository) Refresh(
	ctx context.Context,
	sketchID string,
	lockID string,
	ownerUserID string,
	ttl time.Duration,
) (*model.SketchLock, error) {
	client := r.client()
	rctx, cancel := r.context(ctx)
	defer cancel()

	idKey := lockIDKey(sketchID, lockID)
	scopeKey, err := client.Get(rctx, idKey).Result()
	if errors.Is(err, redis.Nil) {
		return nil, model.ErrLockNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get lock index: %w", err)
	}

	currentBody, lock, err := r.getByScopeKey(ctx, scopeKey)
	if err != nil {
		return nil, err
	}
	if lock.ID != lockID || lock.SketchID != sketchID {
		return nil, model.ErrLockNotFound
	}
	if lock.OwnerUserID != ownerUserID {
		return nil, model.ErrLockNotOwned
	}

	lock.ExpiresAt = time.Now().UTC().Add(ttl).Format(time.RFC3339Nano)
	body, err := easyjson.Marshal(lock)
	if err != nil {
		return nil, fmt.Errorf("marshal lock: %w", err)
	}

	updated, err := refreshScript.Run(
		rctx,
		client,
		[]string{scopeKey, idKey},
		string(currentBody),
		string(body),
		scopeKey,
		ttl.Milliseconds(),
	).Int()
	if err != nil {
		return nil, fmt.Errorf("refresh lock: %w", err)
	}
	if updated != 1 {
		return nil, model.ErrLockNotFound
	}

	return lock, nil
}

func (r *Repository) Release(ctx context.Context, sketchID string, lockID string, ownerUserID string) error {
	client := r.client()
	rctx, cancel := r.context(ctx)
	defer cancel()

	idKey := lockIDKey(sketchID, lockID)
	scopeKey, err := client.Get(rctx, idKey).Result()
	if errors.Is(err, redis.Nil) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("get lock index: %w", err)
	}

	currentBody, lock, err := r.getByScopeKey(ctx, scopeKey)
	if errors.Is(err, model.ErrLockNotFound) {
		_ = client.Del(rctx, idKey).Err()
		return nil
	}
	if err != nil {
		return err
	}
	if lock.ID != lockID || lock.SketchID != sketchID {
		_ = client.Del(rctx, idKey).Err()
		return nil
	}
	if lock.OwnerUserID != ownerUserID {
		return model.ErrLockNotOwned
	}

	if _, err := releaseScript.Run(
		rctx,
		client,
		[]string{scopeKey, idKey},
		string(currentBody),
		scopeKey,
	).Result(); err != nil {
		return fmt.Errorf("release lock: %w", err)
	}

	return nil
}

func (r *Repository) getByScopeKey(ctx context.Context, scopeKey string) ([]byte, *model.SketchLock, error) {
	client := r.client()
	rctx, cancel := r.context(ctx)
	defer cancel()

	body, err := client.Get(rctx, scopeKey).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil, model.ErrLockNotFound
	}
	if err != nil {
		return nil, nil, fmt.Errorf("get lock: %w", err)
	}

	var lock model.SketchLock
	if err := easyjson.Unmarshal(body, &lock); err != nil {
		return nil, nil, fmt.Errorf("unmarshal lock: %w", err)
	}

	return body, &lock, nil
}

func (r *Repository) client() *redis.Client {
	if r.redis.ConnUpdating.Load() {
		for r.redis.ConnUpdating.Load() {
			runtime.Gosched()
		}
	}

	return r.redis.Client
}

func (r *Repository) context(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, r.redis.Timeout)
}

func scopeLockKey(sketchID string, scope []byte) (string, error) {
	canonicalScope, err := canonicalJSON(scope)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(canonicalScope)
	return fmt.Sprintf("%s:{%s}:scope:%s", lockKeyPrefix, sketchID, hex.EncodeToString(sum[:])), nil
}

func lockIDKey(sketchID string, lockID string) string {
	return fmt.Sprintf("%s:{%s}:id:%s", lockKeyPrefix, sketchID, lockID)
}

func canonicalJSON(raw []byte) ([]byte, error) {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, fmt.Errorf("canonicalize lock scope: %w", err)
	}

	return json.Marshal(v)
}
