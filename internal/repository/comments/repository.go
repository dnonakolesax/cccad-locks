package comments

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	dbsql "github.com/dnonakolesax/cccad-locks/internal/db/sql"
	"github.com/dnonakolesax/cccad-locks/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/mailru/easyjson"
)

const (
	listCommentsRequest             = "comments_list"
	getCommentRequest               = "comments_get"
	createCommentRequest            = "comments_create"
	updateCommentRequest            = "comments_update"
	deleteCommentRequest            = "comments_delete"
	changeCommentStatusRequest      = "comments_change_status"
	setCommentAssigneesRequest      = "comments_set_assignees"
	clearCommentAssigneesRequest    = "comments_clear_assignees"
	assigneesSystemMessageRequest   = "comments_assignees_system_message"
	listCommentRepliesRequest       = "comments_replies"
	commentThreadRequest            = "comments_thread"
	listCommentStatusHistoryRequest = "comments_status_history"
	listCommentEditHistoryRequest   = "comments_edit_history"
	commentDocumentAccessRequest    = "comments_document_access"
)

type Repository struct {
	db *dbsql.PGXWorker
}

func NewRepository(db *dbsql.PGXWorker) *Repository {
	return &Repository{db: db}
}

func (r *Repository) List(
	ctx context.Context,
	filter model.CommentListFilter,
	userID string,
) (*model.CommentListResponse, error) {
	sqlRequest, err := r.db.Request(listCommentsRequest)
	if err != nil {
		return nil, fmt.Errorf("list comments request: %w", err)
	}

	rows, err := r.db.Query(
		ctx,
		sqlRequest,
		filter.WorkspaceID,
		userID,
		filter.SketchID,
		filter.PartID,
		filter.TargetType,
		filter.TargetID,
		filter.Kind,
		filter.Status,
		filter.MessageType,
		filter.SystemEventType,
		filter.AssigneeUserID,
		filter.ParentCommentID,
		filter.ThreadRootID,
		filter.RootsOnly,
		filter.IncludeDeleted,
		filter.Limit,
		filter.Offset,
	)
	if err != nil {
		return nil, fmt.Errorf("list comments: %w", err)
	}

	response := &model.CommentListResponse{
		Items:  make([]model.CadComment, 0),
		Limit:  filter.Limit,
		Offset: filter.Offset,
	}
	for rows.Next() {
		comment, total, scanErr := scanCommentWithTotal(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		response.Total = total
		response.Items = append(response.Items, *comment)
	}
	if closeErr := rows.Close(); closeErr != nil {
		return nil, fmt.Errorf("list comments rows: %w", closeErr)
	}

	return response, nil
}

func (r *Repository) Get(ctx context.Context, commentID string, userID string) (*model.CadComment, error) {
	sqlRequest, err := r.db.Request(getCommentRequest)
	if err != nil {
		return nil, fmt.Errorf("get comment request: %w", err)
	}
	return r.queryOneComment(ctx, sqlRequest, commentID, userID)
}

func (r *Repository) DocumentWorkspace(ctx context.Context, documentID string, userID string) (string, error) {
	sqlRequest, err := r.db.Request(commentDocumentAccessRequest)
	if err != nil {
		return "", fmt.Errorf("comment document access request: %w", err)
	}
	rows, err := r.db.Query(ctx, sqlRequest, documentID, userID)
	if err != nil {
		return "", fmt.Errorf("comment document access: %w", err)
	}
	if !rows.Next() {
		if closeErr := rows.Close(); closeErr != nil {
			return "", fmt.Errorf("comment document access rows: %w", closeErr)
		}
		return "", errors.New("comment document access returned no rows")
	}
	var workspaceID string
	if err := rows.Scan(&workspaceID); err != nil {
		return "", fmt.Errorf("scan comment document access: %w", err)
	}
	if closeErr := rows.Close(); closeErr != nil {
		return "", fmt.Errorf("comment document access rows: %w", closeErr)
	}
	return workspaceID, nil
}

func (r *Repository) Create(
	ctx context.Context,
	workspaceID string,
	request *model.CreateCommentRequest,
	actorUserID string,
) (*model.CadComment, error) {
	sqlRequest, err := r.db.Request(createCommentRequest)
	if err != nil {
		return nil, fmt.Errorf("create comment request: %w", err)
	}
	setAssigneesSQL, err := r.db.Request(setCommentAssigneesRequest)
	if err != nil {
		return nil, fmt.Errorf("set comment assignees request: %w", err)
	}
	getSQL, err := r.db.Request(getCommentRequest)
	if err != nil {
		return nil, fmt.Errorf("get comment request: %w", err)
	}

	var comment *model.CadComment
	err = r.db.WithTx(ctx, func(txCtx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(
			txCtx,
			sqlRequest,
			workspaceID,
			nullableString(request.ParentCommentID),
			nullableString(request.SketchID),
			nullableString(request.PartID),
			request.TargetType,
			request.TargetID,
			request.Kind,
			actorUserID,
			request.Body,
			request.Status,
			request.SketchVersion,
			request.PartVersion,
			nullableRaw(request.Anchor),
			string(request.Metadata),
		)
		commentID, scanErr := scanCommentID(row)
		if scanErr != nil {
			return scanErr
		}
		if execErr := execAuthorized(txCtx, tx, setAssigneesSQL, commentID, actorUserID, request.AssigneeUserIDs); execErr != nil {
			return execErr
		}
		found, getErr := queryTxComment(txCtx, tx, getSQL, commentID, actorUserID)
		if getErr != nil {
			return getErr
		}
		comment = found
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("create comment: %w", err)
	}
	return comment, nil
}

func (r *Repository) ListReplies(
	ctx context.Context,
	commentID string,
	filter model.CommentListFilter,
	userID string,
) (*model.CommentListResponse, error) {
	sqlRequest, err := r.db.Request(listCommentRepliesRequest)
	if err != nil {
		return nil, fmt.Errorf("list comment replies request: %w", err)
	}
	rows, err := r.db.Query(ctx, sqlRequest, commentID, userID, filter.IncludeSystem, filter.IncludeDeleted, filter.Limit, filter.Offset)
	if err != nil {
		return nil, fmt.Errorf("list comment replies: %w", err)
	}
	response := &model.CommentListResponse{
		Items:  make([]model.CadComment, 0),
		Limit:  filter.Limit,
		Offset: filter.Offset,
	}
	for rows.Next() {
		comment, total, scanErr := scanCommentWithTotal(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		response.Total = total
		response.Items = append(response.Items, *comment)
	}
	if closeErr := rows.Close(); closeErr != nil {
		return nil, fmt.Errorf("list comment replies rows: %w", closeErr)
	}
	return response, nil
}

func (r *Repository) Thread(
	ctx context.Context,
	commentID string,
	filter model.CommentListFilter,
	userID string,
) (*model.CommentThreadResponse, error) {
	sqlRequest, err := r.db.Request(commentThreadRequest)
	if err != nil {
		return nil, fmt.Errorf("comment thread request: %w", err)
	}
	rows, err := r.db.Query(ctx, sqlRequest, commentID, userID, filter.IncludeSystem, filter.MaxDepth)
	if err != nil {
		return nil, fmt.Errorf("comment thread: %w", err)
	}
	items := make([]model.CadComment, 0)
	for rows.Next() {
		comment, scanErr := scanComment(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *comment)
	}
	if closeErr := rows.Close(); closeErr != nil {
		return nil, fmt.Errorf("comment thread rows: %w", closeErr)
	}
	if len(items) == 0 {
		return nil, errors.New("comment thread returned no rows")
	}
	return &model.CommentThreadResponse{Root: items[0], Items: items}, nil
}

func (r *Repository) Update(
	ctx context.Context,
	commentID string,
	request *model.UpdateCommentRequest,
	actorUserID string,
) (*model.CadComment, error) {
	sqlRequest, err := r.db.Request(updateCommentRequest)
	if err != nil {
		return nil, fmt.Errorf("update comment request: %w", err)
	}
	return r.queryOneComment(
		ctx,
		sqlRequest,
		commentID,
		actorUserID,
		request.Body,
		nullableRaw(request.Anchor),
		nullableRaw(request.Metadata),
	)
}

func (r *Repository) Delete(ctx context.Context, commentID string, actorUserID string) error {
	sqlRequest, err := r.db.Request(deleteCommentRequest)
	if err != nil {
		return fmt.Errorf("delete comment request: %w", err)
	}
	rows, err := r.db.Query(ctx, sqlRequest, commentID, actorUserID)
	if err != nil {
		return fmt.Errorf("delete comment: %w", err)
	}
	if !rows.Next() {
		if closeErr := rows.Close(); closeErr != nil {
			return fmt.Errorf("delete comment rows: %w", closeErr)
		}
		return errors.New("delete comment returned no rows")
	}
	if closeErr := rows.Close(); closeErr != nil {
		return fmt.Errorf("delete comment rows: %w", closeErr)
	}
	return nil
}

func (r *Repository) ChangeStatus(
	ctx context.Context,
	commentID string,
	request *model.ChangeCommentStatusRequest,
	actorUserID string,
) (*model.ChangeCommentStatusResponse, error) {
	sqlRequest, err := r.db.Request(changeCommentStatusRequest)
	if err != nil {
		return nil, fmt.Errorf("change comment status request: %w", err)
	}
	comment, systemMessage, err := r.queryCommentPair(ctx, sqlRequest, commentID, actorUserID, request.Status, request.Reason)
	if err != nil {
		return nil, err
	}
	return &model.ChangeCommentStatusResponse{Comment: *comment, SystemMessage: *systemMessage}, nil
}

func (r *Repository) ReplaceAssignees(
	ctx context.Context,
	commentID string,
	request *model.ReplaceCommentAssigneesRequest,
	actorUserID string,
) (*model.ChangeCommentAssigneesResponse, error) {
	clearSQL, err := r.db.Request(clearCommentAssigneesRequest)
	if err != nil {
		return nil, fmt.Errorf("clear comment assignees request: %w", err)
	}
	setSQL, err := r.db.Request(setCommentAssigneesRequest)
	if err != nil {
		return nil, fmt.Errorf("set comment assignees request: %w", err)
	}
	systemSQL, err := r.db.Request(assigneesSystemMessageRequest)
	if err != nil {
		return nil, fmt.Errorf("assignees system message request: %w", err)
	}
	getSQL, err := r.db.Request(getCommentRequest)
	if err != nil {
		return nil, fmt.Errorf("get comment request: %w", err)
	}

	var comment *model.CadComment
	var systemMessage *model.CadComment
	err = r.db.WithTx(ctx, func(txCtx context.Context, tx pgx.Tx) error {
		if execErr := execAuthorized(txCtx, tx, clearSQL, commentID, actorUserID); execErr != nil {
			return execErr
		}
		if execErr := execAuthorized(txCtx, tx, setSQL, commentID, actorUserID, request.AssigneeUserIDs); execErr != nil {
			return execErr
		}
		systemID, scanErr := scanCommentID(tx.QueryRow(txCtx, systemSQL, commentID, actorUserID, request.AssigneeUserIDs))
		if scanErr != nil {
			return scanErr
		}
		foundComment, getErr := queryTxComment(txCtx, tx, getSQL, commentID, actorUserID)
		if getErr != nil {
			return getErr
		}
		foundSystem, getErr := queryTxComment(txCtx, tx, getSQL, systemID, actorUserID)
		if getErr != nil {
			return getErr
		}
		comment = foundComment
		systemMessage = foundSystem
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("replace comment assignees: %w", err)
	}
	return &model.ChangeCommentAssigneesResponse{Comment: *comment, SystemMessage: *systemMessage}, nil
}

func (r *Repository) StatusHistory(
	ctx context.Context,
	commentID string,
	userID string,
) ([]model.CommentStatusHistoryItem, error) {
	sqlRequest, err := r.db.Request(listCommentStatusHistoryRequest)
	if err != nil {
		return nil, fmt.Errorf("comment status history request: %w", err)
	}
	rows, err := r.db.Query(ctx, sqlRequest, commentID, userID)
	if err != nil {
		return nil, fmt.Errorf("comment status history: %w", err)
	}
	items := make([]model.CommentStatusHistoryItem, 0)
	for rows.Next() {
		item, scanErr := scanStatusHistory(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *item)
	}
	if closeErr := rows.Close(); closeErr != nil {
		return nil, fmt.Errorf("comment status history rows: %w", closeErr)
	}
	return items, nil
}

func (r *Repository) EditHistory(
	ctx context.Context,
	commentID string,
	userID string,
) ([]model.CommentEditHistoryItem, error) {
	sqlRequest, err := r.db.Request(listCommentEditHistoryRequest)
	if err != nil {
		return nil, fmt.Errorf("comment edit history request: %w", err)
	}
	rows, err := r.db.Query(ctx, sqlRequest, commentID, userID)
	if err != nil {
		return nil, fmt.Errorf("comment edit history: %w", err)
	}
	items := make([]model.CommentEditHistoryItem, 0)
	for rows.Next() {
		item, scanErr := scanEditHistory(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *item)
	}
	if closeErr := rows.Close(); closeErr != nil {
		return nil, fmt.Errorf("comment edit history rows: %w", closeErr)
	}
	return items, nil
}

func (r *Repository) queryOneComment(ctx context.Context, sqlRequest string, args ...any) (*model.CadComment, error) {
	rows, err := r.db.Query(ctx, sqlRequest, args...)
	if err != nil {
		return nil, err
	}
	if !rows.Next() {
		if closeErr := rows.Close(); closeErr != nil {
			return nil, closeErr
		}
		return nil, errors.New("comment returned no rows")
	}
	comment, err := scanComment(rows)
	if err != nil {
		return nil, err
	}
	if closeErr := rows.Close(); closeErr != nil {
		return nil, closeErr
	}
	return comment, nil
}

func (r *Repository) queryCommentPair(ctx context.Context, sqlRequest string, args ...any) (*model.CadComment, *model.CadComment, error) {
	rows, err := r.db.Query(ctx, sqlRequest, args...)
	if err != nil {
		return nil, nil, err
	}
	comments := make([]*model.CadComment, 0, 2)
	for rows.Next() {
		comment, scanErr := scanComment(rows)
		if scanErr != nil {
			return nil, nil, scanErr
		}
		comments = append(comments, comment)
	}
	if closeErr := rows.Close(); closeErr != nil {
		return nil, nil, closeErr
	}
	if len(comments) != 2 {
		return nil, nil, errors.New("comment mutation returned incomplete rows")
	}
	return comments[0], comments[1], nil
}

func queryTxComment(ctx context.Context, tx pgx.Tx, sqlRequest string, args ...any) (*model.CadComment, error) {
	rows, err := tx.Query(ctx, sqlRequest, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, errors.New("comment returned no rows")
	}
	comment, err := scanPGXComment(rows)
	if err != nil {
		return nil, err
	}
	return comment, rows.Err()
}

func execAuthorized(ctx context.Context, tx pgx.Tx, sqlRequest string, args ...any) error {
	rows, err := tx.Query(ctx, sqlRequest, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	if !rows.Next() {
		return errors.New("comment mutation returned no rows")
	}
	return rows.Err()
}

type commentScanner interface {
	Scan(dest ...any) error
}

func scanComment(rows *dbsql.PGXResponse) (*model.CadComment, error) {
	return scanCommentFields(rows)
}

func scanPGXComment(rows pgx.Rows) (*model.CadComment, error) {
	return scanCommentFields(rows)
}

func scanCommentWithTotal(rows *dbsql.PGXResponse) (*model.CadComment, int, error) {
	comment, total, err := scanCommentFieldsWithTotal(rows)
	return comment, total, err
}

func scanCommentFields(scanner commentScanner) (*model.CadComment, error) {
	comment, _, err := scanCommentFieldsWithTotal(scanner)
	return comment, err
}

func scanCommentFieldsWithTotal(scanner commentScanner) (*model.CadComment, int, error) {
	var comment model.CadComment
	var sketchID *string
	var partID *string
	var parentCommentID *string
	var systemEventType *string
	var sketchVersion *int64
	var partVersion *int64
	var anchor []byte
	var eventPayload []byte
	var metadata []byte
	var assignees []byte
	var createdAt time.Time
	var updatedAt time.Time
	var deletedAt *time.Time
	total := 0

	if err := scanner.Scan(
		&comment.ID,
		&comment.WorkspaceID,
		&sketchID,
		&partID,
		&parentCommentID,
		&comment.ThreadRootID,
		&comment.ReplyDepth,
		&comment.ReplyCount,
		&comment.MessageType,
		&systemEventType,
		&eventPayload,
		&comment.TargetType,
		&comment.TargetID,
		&comment.Kind,
		&comment.Status,
		&comment.AuthorUserID,
		&comment.Body,
		&sketchVersion,
		&partVersion,
		&anchor,
		&metadata,
		&assignees,
		&createdAt,
		&updatedAt,
		&deletedAt,
		&total,
	); err != nil {
		return nil, 0, fmt.Errorf("scan comment: %w", err)
	}

	comment.SketchID = sketchID
	comment.PartID = partID
	comment.ParentCommentID = parentCommentID
	comment.SystemEventType = systemEventType
	comment.SketchVersion = sketchVersion
	comment.PartVersion = partVersion
	comment.Anchor = easyjson.RawMessage(anchor)
	comment.EventPayload = easyjson.RawMessage(eventPayload)
	if len(comment.EventPayload) == 0 {
		comment.EventPayload = easyjson.RawMessage(`{}`)
	}
	comment.Metadata = easyjson.RawMessage(metadata)
	if len(comment.Metadata) == 0 {
		comment.Metadata = easyjson.RawMessage(`{}`)
	}
	if err := json.Unmarshal(assignees, &comment.AssigneeUserIDs); err != nil {
		return nil, 0, fmt.Errorf("scan comment assignees: %w", err)
	}
	comment.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
	comment.UpdatedAt = updatedAt.UTC().Format(time.RFC3339Nano)
	if deletedAt != nil {
		value := deletedAt.UTC().Format(time.RFC3339Nano)
		comment.DeletedAt = &value
	}
	return &comment, total, nil
}

func scanCommentID(row pgx.Row) (string, error) {
	var id string
	if err := row.Scan(&id); err != nil {
		return "", err
	}
	return id, nil
}

func scanStatusHistory(rows *dbsql.PGXResponse) (*model.CommentStatusHistoryItem, error) {
	var item model.CommentStatusHistoryItem
	var changedAt time.Time
	if err := rows.Scan(
		&item.ID,
		&item.CommentID,
		&item.OldStatus,
		&item.NewStatus,
		&item.ChangedByUserID,
		&changedAt,
		&item.Reason,
	); err != nil {
		return nil, fmt.Errorf("scan comment status history: %w", err)
	}
	item.ChangedAt = changedAt.UTC().Format(time.RFC3339Nano)
	return &item, nil
}

func scanEditHistory(rows *dbsql.PGXResponse) (*model.CommentEditHistoryItem, error) {
	var item model.CommentEditHistoryItem
	var editedAt time.Time
	if err := rows.Scan(
		&item.ID,
		&item.CommentID,
		&item.OldBody,
		&item.NewBody,
		&item.EditedByUserID,
		&editedAt,
	); err != nil {
		return nil, fmt.Errorf("scan comment edit history: %w", err)
	}
	item.EditedAt = editedAt.UTC().Format(time.RFC3339Nano)
	return &item, nil
}

func nullableString(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableRaw(value easyjson.RawMessage) any {
	if len(value) == 0 || string(value) == "null" {
		return nil
	}
	return string(value)
}
