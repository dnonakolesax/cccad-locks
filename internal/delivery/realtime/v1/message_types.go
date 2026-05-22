package v1

const (
	MsgSessionJoin        = "session.join"
	MsgSessionJoined      = "session.joined"
	MsgSessionUserJoined  = "session.user_joined"
	MsgSessionUserLeft    = "session.user_left"
	MsgSessionPing        = "session.ping"
	MsgSessionPong        = "session.pong"
	MsgAccessRevoked      = "session.access_revoked"

	MsgPresenceCursor    = "presence.cursor"
	MsgPresenceSelection = "presence.selection"
	MsgPresenceHover     = "presence.hover"
	MsgPresenceTool      = "presence.tool"

	MsgDragBegin         = "drag.begin"
	MsgDragBeginAccepted = "drag.begin.accepted"
	MsgDragBeginRejected = "drag.begin.rejected"
	MsgDragPreview       = "drag.preview"
	MsgDragCommit        = "drag.commit"
	MsgDragCancel        = "drag.cancel"
	MsgDragCancelled     = "drag.cancelled"

	MsgOpSubmit    = "op.submit"
	MsgOpCommitted = "op.committed"
	MsgOpRejected  = "op.rejected"
	MsgOpsBatch    = "ops.batch"

	MsgLockAcquire  = "lock.acquire"
	MsgLockAcquired = "lock.acquired"
	MsgLockRejected = "lock.rejected"
	MsgLockRefresh  = "lock.refresh"
	MsgLockRefreshed = "lock.refreshed"
	MsgLockRelease  = "lock.release"
	MsgLockReleased = "lock.released"

	MsgPermissionUpdated = "permission.updated"
	MsgPermissionRevoked = "permission.revoked"

	MsgConflictCreated  = "conflict.created"
	MsgConflictResolved = "conflict.resolved"

	MsgStateResyncRequired = "state.resync_required"
	MsgStateSnapshot       = "state.snapshot"

	MsgError = "error"
)
