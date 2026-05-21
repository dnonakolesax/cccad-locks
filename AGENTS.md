# AGENTS.md — cccAD Go Backend Service

This document defines working rules for AI agents and contributors modifying the cccAD Go backend service.

The service is part of **cccAD**, an MVP cloud collaborative CAD system focused on 2D sketching, parametric features, collaboration, and later potential FEM/CAE workflows.

---

## 1. Service role

The Go backend is the **application, collaboration, authorization, persistence, and orchestration layer**.

It is responsible for:

- HTTP API exposed to the frontend.
- WebSocket realtime collaboration.
- Sketch document lifecycle.
- Operation log management.
- Versioning and idempotency.
- Access control for sketches.
- Integration with Keycloak SSO.
- Dynamic database credentials through Vault.
- PostgreSQL persistence.
- Redis-based ephemeral realtime state.
- S3-compatible object storage metadata and object operations.
- Calling the C++ solver service through gRPC.
- Persisting solver results, diagnostics, conflicts, and materialized sketch state.

The Go backend must **not** implement the geometric constraint solving kernel. Solver logic belongs to the C++ solver service.

---

## 2. Tech stack

Expected stack:

- Language: Go
- HTTP transport: `net/http`
- Router: `*http.ServeMux`
- OpenAPI workflow: spec-first
- OpenAPI generator: preferably `ogen` or `oapi-codegen`
- WebSocket: `github.com/gorilla/websocket`
- JSON serialization for realtime models: `github.com/mailru/easyjson`
- Auth: Keycloak, client ID `cad`
- Secrets: Vault
- Primary database: PostgreSQL
- Ephemeral realtime state: Redis
- Object storage: S3-compatible storage
- Solver communication: gRPC + protobuf

Do not introduce Gin, Echo, Fiber, or another HTTP framework unless explicitly requested.

---

## 3. Repository layout

Use this general structure:

```text
backend/
  cmd/
    api/
      main.go

  internal/
    auth/
    config/
    delivery/
      http/
        v1/
      realtime/
        v1/
    model/
    service/
      sketch/
      realtime/
      permission/
      storage/
    repository/
      postgres/
      redis/
      s3/
    solverclient/
    middleware/
    observability/

  migrations/
    001_init.sql

  api/
    openapi.yaml
    openapi.json

  gen/
    openapi/

  proto/
    solver/
      v1/
```

Layering rule:

```text
delivery -> service -> repository / solverclient / external integrations
```

The delivery layer must not contain business logic beyond request parsing, authentication extraction, basic validation, and response mapping.

---

## 4. Package boundaries

### `internal/delivery/http/v1`

Responsible for REST endpoints generated from OpenAPI or implemented around generated interfaces.

Allowed responsibilities:

- Decode request.
- Extract authenticated user from context.
- Call service interface.
- Convert service result to API response.
- Map known errors to HTTP status codes.

Not allowed:

- SQL queries.
- Redis operations.
- S3 calls.
- Solver calls directly.
- Permission decisions beyond invoking service methods.

### `internal/delivery/realtime/v1`

Responsible for WebSocket transport.

Required shape:

```go
func (h *Handler) RegisterRoutes(mux *http.ServeMux)
```

Register routes using:

```go
mux.HandleFunc(...)
```

The handler should expose the sketch realtime endpoint:

```text
/api/v1/sketches/{sketchId}/ws
```

The realtime handler should depend on interfaces, not concrete service implementations.

### `internal/model`

Contains shared internal domain DTOs and realtime message types.

Realtime JSON models must support `mailru/easyjson`.

Use generation comments where needed:

```go
//go:generate easyjson -all realtime_messages.go
```

Do not put PostgreSQL-specific row structs here unless they are true domain models.

### `internal/service`

Contains business logic.

This layer decides:

- Whether a user can read/edit/admin a sketch.
- Whether an operation can be accepted.
- How operation idempotency works.
- When to call the C++ solver.
- Whether a solver result is acceptable.
- When to create conflicts.
- How to broadcast realtime events.
- How to rebuild sketch state from snapshots and operations.

### `internal/repository/postgres`

Contains PostgreSQL persistence.

Allowed:

- SQL queries.
- Transactions.
- Row mapping.
- Migration-adjacent storage logic.

Not allowed:

- Keycloak-specific logic.
- WebSocket connection management.
- Solver-specific algorithms.

### `internal/repository/redis`

Contains Redis integration.

Use Redis for ephemeral state only:

- Presence.
- WebSocket fanout/pubsub.
- Locks.
- Rate limits.
- Short-lived caches.

Do not store source-of-truth sketch data in Redis.

### `internal/repository/s3`

Contains S3 object storage integration.

Use S3 for:

- Preview images.
- Exports: SVG, DXF, PDF.
- Imports.
- Large snapshots.
- Backup artifacts.
- Future FEM meshes/results.

Do not use S3 as the primary source of live sketch state.

### `internal/solverclient`

Contains the Go client for the C++ solver service.

The solver client must expose an internal interface similar to:

```go
type Client interface {
    Solve(ctx context.Context, req SolveRequest) (*SolveResult, error)
    Check(ctx context.Context, req CheckRequest) (*CheckResult, error)
    ApplyIntent(ctx context.Context, req ApplyIntentRequest) (*ApplyIntentResult, error)
    Analyze(ctx context.Context, req AnalyzeRequest) (*AnalyzeResult, error)
}
```

The solver service is computational only. It must not know about:

- Users.
- Keycloak.
- Permissions.
- Workspaces.
- PostgreSQL.
- Redis.
- S3.
- WebSocket sessions.

---

## 5. Source of truth

The primary source of truth is PostgreSQL.

Use:

```text
sketch_ops             — persistent operation log
sketch_current_states  — fast current state
sketch_snapshots       — replay acceleration / backup
sketch_permissions     — sketch ACL
sketch_conflicts       — explicit conflict tracking
sketch_files           — S3 object metadata
```

Do not treat WebSocket messages, Redis state, or S3 files as the source of truth for the active sketch document.

---

## 6. Sketch opening flow

When a user opens a sketch:

1. Frontend calls:

   ```http
   GET /api/v1/sketches/{sketchId}
   ```

2. Go backend:
   - Validates JWT.
   - Extracts Keycloak `sub`.
   - Checks sketch permission.
   - Loads `sketch_current_states`.
   - If missing, rebuilds from snapshot + operation log.
   - Returns the document and version.

3. Frontend connects to WebSocket.

4. Frontend sends `session.join` with `lastSeenVersion`.

5. WebSocket is used only for realtime events after the initial REST load.

---

## 7. Operation log rules

All persistent document changes must be represented as operations.

Examples:

- `create_point`
- `create_line`
- `create_circle`
- `create_arc`
- `create_rectangle`
- `create_polyline`
- `move_point`
- `delete_entity`
- `add_constraint`
- `remove_constraint`
- `add_dimension`
- `set_dimension_value`
- `remove_dimension`

Each operation must have:

- Server operation ID.
- Sketch ID.
- Actor user ID.
- Client operation ID.
- Base version.
- Committed version.
- Payload.
- Created timestamp.

Use `clientOpId` for idempotency.

A retried request with the same `(sketchId, actorUserId, clientOpId)` must not create a duplicate operation.

---

## 8. Operation pipeline

Persistent operation processing should follow this pipeline:

```text
1. Parse request.
2. Authenticate user.
3. Check permission.
4. Load current sketch state.
5. Check base version and idempotency.
6. Check locks if required.
7. Apply operation to draft graph state.
8. Convert draft state to solver model.
9. Call solver when needed.
10. Validate solver result.
11. Start PostgreSQL transaction.
12. Insert operation into sketch_ops.
13. Update sketch_current_states.
14. Create snapshot if needed.
15. Persist conflicts if any.
16. Commit transaction.
17. Publish op.committed through realtime hub / Redis pubsub.
18. Return response.
```

Never broadcast `op.committed` before the database transaction commits.

---

## 9. Realtime WebSocket rules

Realtime messages are divided into:

```text
session/control
presence
drag preview
persistent operations
locks
permissions
conflicts
state synchronization
errors
```

Persistent changes:

- `op.submit`
- `drag.commit`

Transient events:

- `presence.cursor`
- `presence.selection`
- `presence.hover`
- `presence.tool`
- `drag.preview`
- `session.ping`
- `session.pong`
- `lock.refresh`

Do not write transient events to PostgreSQL.

---

## 10. Drag rules

Do not persist every mouse movement.

Correct flow:

```text
drag.begin       — transient, may acquire lock
drag.preview     — transient, may be throttled
drag.commit      — persistent operation
drag.cancel      — transient
```

Only `drag.commit` becomes a sketch operation, typically `move_point` or another solver intent.

---

## 11. Locking rules

Locks are soft, temporary, and stored in Redis.

Recommended lock scopes:

- Entity.
- Constraint.
- Dimension.
- Constraint component.

For CAD editing, prefer locking the connected constraint component rather than only a single point when the edit may affect multiple entities.

Locks must have TTL and refresh.

Expired locks must not block operations.

---

## 12. Permission model

Sketch roles:

```text
reader
editor
admin
```

Rules:

- `reader` can read the sketch.
- `editor` can read and edit sketch geometry/constraints/dimensions.
- `admin` can manage `reader` and `editor` permissions.
- Only the sketch creator can grant or revoke `admin`.
- The creator's own `admin` permission must not be removed or downgraded.

The authoritative user identity is the Keycloak `sub` claim.

Do not store or handle passwords in this service.

---

## 13. Keycloak integration

The service uses Keycloak SSO.

Known client ID:

```text
cad
```

The Go service should validate JWTs using Keycloak JWKS.

Extract at minimum:

- `sub`
- `preferred_username`
- `email`, if available
- realm/client roles, if needed later

Use the `sub` claim as the stable user ID in PostgreSQL.

---

## 14. Vault integration

Production database credentials should come from Vault dynamic database secrets.

Recommended model:

- Permanent PostgreSQL role: `cccad_app NOLOGIN`
- Vault-created temporary login roles inherit `cccad_app`
- Go service receives temporary username/password from Vault

Do not hardcode DB passwords.

Do not commit `.env` files with secrets.

---

## 15. PostgreSQL rules

Use PostgreSQL as the source of truth.

Recommended tables:

```text
user_profiles
workspaces
sketches
sketch_permissions
sketch_permission_audit
sketch_ops
sketch_current_states
sketch_snapshots
sketch_conflicts
sketch_files
simulation_jobs
audit_events
```

Use transactions for operation commits.

Use JSONB for operation payloads and current sketch state in MVP.

Normalize later only when access patterns become clear.

---

## 16. S3 rules

S3 is already available and should be used for object storage.

Use S3 for:

- `preview_png`
- `export_svg`
- `export_dxf`
- `export_pdf`
- `snapshot_json_zst`
- `import_original`
- future FEM files

Recommended object key format:

```text
workspaces/{workspaceId}/sketches/{sketchId}/snapshots/v{version}.json.zst
workspaces/{workspaceId}/sketches/{sketchId}/previews/v{version}.png
workspaces/{workspaceId}/sketches/{sketchId}/exports/{exportId}.svg
workspaces/{workspaceId}/sketches/{sketchId}/imports/{fileId}/original.dxf
workspaces/{workspaceId}/sketches/{sketchId}/simulations/{jobId}/result.vtu
```

Store S3 metadata in PostgreSQL, not only in object names.

---

## 17. C++ solver boundary

The Go backend calls the C++ solver through gRPC.

Main solver operations:

```text
Solve
Check
ApplyIntent
Analyze
```

Use the solver for:

- Moving constrained points/entities.
- Adding/removing constraints.
- Setting driving dimensions.
- Checking consistency.
- Computing DOF.
- Getting diagnostics.
- Materializing solved geometry.

The solver must be deterministic for the same input.

The Go backend must not implement a second incompatible solver.

---

## 18. Error model

Errors must be machine-readable.

Use stable error codes, for example:

```text
AUTH_REQUIRED
PERMISSION_DENIED
SKETCH_NOT_FOUND
STALE_VERSION
LOCK_CONFLICT
INVALID_REFERENCE
INVALID_OPERATION
SOLVER_INCONSISTENT
SOLVER_OVER_CONSTRAINED
SOLVER_NUMERICAL_FAILURE
CONSTRAINT_NOT_SUPPORTED
ENTITY_NOT_SUPPORTED
INTERNAL_ERROR
```

Map errors consistently to HTTP and WebSocket responses.

---

## 19. OpenAPI rules

The REST API is spec-first.

Do not manually change generated files.

Generated files should be clearly separated, for example:

```text
internal/delivery/http/v1/gen/
```

or:

```text
internal/gen/openapi/
```

If the OpenAPI spec changes:

1. Update `api/openapi.yaml`.
2. Regenerate code.
3. Update handlers/services to satisfy generated interfaces.
4. Run tests.

Keep the OpenAPI spec compatible with the chosen generator.

For `ogen`, avoid unsupported OpenAPI features such as:

- OpenAPI 3.1-only `type: [string, "null"]`
- array defaults
- object defaults
- numeric `exclusiveMinimum` from OpenAPI 3.1

Prefer OpenAPI 3.0-compatible constructs:

```yaml
type: string
nullable: true
```

and:

```yaml
minimum: 0
exclusiveMinimum: true
```

---

## 20. easyjson rules

Realtime models in `internal/model` should be easyjson-compatible.

Do not use fields that are hard to serialize without a custom implementation unless necessary.

After editing realtime models, run:

```bash
go generate ./internal/model
```

Generated files usually look like:

```text
*_easyjson.go
```

Do not manually edit generated easyjson files.

---

## 21. Logging and observability

Use structured logging.

Each request or operation should carry:

- Request ID.
- User ID.
- Sketch ID, when applicable.
- Operation ID, when applicable.
- Client operation ID, when applicable.
- Version, when applicable.

Never log secrets, JWTs, passwords, Vault tokens, or S3 access keys.

Log solver failures with diagnostics, but avoid dumping huge sketch payloads by default.

---

## 22. Testing rules

At minimum, cover:

- Permission checks.
- Creator admin invariants.
- Operation idempotency.
- Stale version behavior.
- Lock conflict behavior.
- `GetSketch` fallback rebuild from operation log.
- WebSocket message parsing.
- Error mapping.
- Solver client error handling.

Use interface-based service tests. Do not require a real solver for delivery layer tests.

---

## 23. Security rules

Always enforce authorization server-side.

Frontend checks are only UX hints.

Every REST and WebSocket action must be associated with an authenticated user.

Before accepting a WebSocket connection:

1. Validate JWT.
2. Extract user ID.
3. Check sketch permission.
4. Reject unauthorized users.

For `permission.revoked`, if the affected user currently has an open WebSocket connection, send `session.access_revoked` and close the connection.

---

## 24. Code style

Prefer simple, idiomatic Go.

Use:

- `context.Context`
- small interfaces
- explicit error handling
- dependency injection through constructors
- table-driven tests

Avoid:

- global mutable state
- hidden goroutine leaks
- framework-specific abstractions
- panics in request handling
- business logic in delivery handlers
- generated code edits

---

## 25. Concurrency rules

Be careful with:

- WebSocket connection maps.
- Redis pub/sub consumers.
- lock TTL refresh.
- operation commits.
- duplicate client operations.
- reconnect/resync flows.

Use mutexes or channel ownership patterns intentionally.

Do not hold locks while doing slow network calls to PostgreSQL, Redis, S3, Vault, or the solver.

---

## 26. Backward compatibility

Realtime protocol and REST API are versioned.

Current paths should use:

```text
/api/v1/...
```

Realtime messages should include a protocol version during `session.join`.

Breaking changes require a new version or an explicit migration path.

---

## 27. What not to do

Do not:

- Store live sketch source of truth in S3.
- Store persistent sketch state only in Redis.
- Let frontend call the C++ solver directly.
- Store Keycloak passwords or duplicate auth state locally.
- Broadcast operation commits before database commit.
- Persist `drag.preview` events.
- Use array index references for entities.
- Use unstable IDs for geometry references.
- Manually edit generated OpenAPI or easyjson files.
- Add a web framework without explicit approval.
- Implement solver logic in Go delivery handlers.

---

## 28. Minimal service interfaces

A delivery handler should depend on interfaces like this:

```go
type SketchService interface {
    GetSketch(ctx context.Context, userID string, sketchID string) (*model.SketchDocument, error)
    SubmitOperation(ctx context.Context, userID string, sketchID string, req model.SubmitOperationRequest) (*model.OperationCommitResult, error)
    CreateSketch(ctx context.Context, userID string, req model.CreateSketchRequest) (*model.SketchDocument, error)
}
```

Realtime delivery should depend on something like:

```go
type RealtimeService interface {
    Join(ctx context.Context, conn Connection, userID string, sketchID string, req model.SessionJoinPayload) error
    Leave(ctx context.Context, userID string, sketchID string, clientID string) error
    HandleMessage(ctx context.Context, userID string, sketchID string, msg model.ClientRealtimeMessage) error
}
```

Exact names may differ, but the direction must remain:

```text
delivery calls service interfaces
service owns business logic
repositories own persistence
```

---

## 29. Final architectural rule

The Go backend is the authoritative coordinator.

PostgreSQL is the source of truth.

Redis is ephemeral realtime infrastructure.

S3 is object storage for large files and artifacts.

Keycloak is identity.

Vault is secrets.

The C++ service is the deterministic geometric solver.

Keep these boundaries strict.
