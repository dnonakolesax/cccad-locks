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
	replaceCommentAssigneesRequest  = "comments_replace_assignees"
	listCommentStatusHistoryRequest = "comments_status_history"
	listCommentEditHistoryRequest   = "comments_edit_history"
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
		filter.DocumentID,
		userID,
		filter.PartID,
		filter.TargetType,
		filter.TargetID,
		filter.Kind,
		filter.Status,
		filter.AssigneeID,
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

func (r *Repository) Create(
	ctx context.Context,
	documentID string,
	request *model.CreateCommentRequest,
	actorUserID string,
) (*model.CadComment, error) {
	sqlRequest, err := r.db.Request(createCommentRequest)
	if err != nil {
		return nil, fmt.Errorf("create comment request: %w", err)
	}
	replaceAssigneesSQL, err := r.db.Request(replaceCommentAssigneesRequest)
	if err != nil {
		return nil, fmt.Errorf("replace comment assignees request: %w", err)
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
			documentID,
			nullableString(request.PartID),
			request.TargetType,
			request.TargetID,
			request.Kind,
			actorUserID,
			request.Body,
			request.Status,
			request.DocumentVersion,
			request.PartVersion,
			nullableRaw(request.Anchor),
			string(request.Metadata),
		)
		commentID, scanErr := scanCommentID(row)
		if scanErr != nil {
			return scanErr
		}
		if execErr := execAuthorized(txCtx, tx, replaceAssigneesSQL, commentID, actorUserID, request.AssigneeIDs); execErr != nil {
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
) (*model.CadComment, error) {
	sqlRequest, err := r.db.Request(changeCommentStatusRequest)
	if err != nil {
		return nil, fmt.Errorf("change comment status request: %w", err)
	}
	return r.queryOneComment(ctx, sqlRequest, commentID, actorUserID, request.Status, request.Reason)
}

func (r *Repository) ReplaceAssignees(
	ctx context.Context,
	commentID string,
	request *model.ReplaceCommentAssigneesRequest,
	actorUserID string,
) (*model.CadComment, error) {
	sqlRequest, err := r.db.Request(replaceCommentAssigneesRequest)
	if err != nil {
		return nil, fmt.Errorf("replace comment assignees request: %w", err)
	}
	getSQL, err := r.db.Request(getCommentRequest)
	if err != nil {
		return nil, fmt.Errorf("get comment request: %w", err)
	}

	var comment *model.CadComment
	err = r.db.WithTx(ctx, func(txCtx context.Context, tx pgx.Tx) error {
		if execErr := execAuthorized(txCtx, tx, sqlRequest, commentID, actorUserID, request.AssigneeIDs); execErr != nil {
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
		return nil, fmt.Errorf("replace comment assignees: %w", err)
	}
	return comment, nil
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
	var partID *string
	var documentVersion *int64
	var partVersion *int64
	var anchor []byte
	var metadata []byte
	var assignees []byte
	var createdAt time.Time
	var updatedAt time.Time
	var deletedAt *time.Time
	total := 0

	if err := scanner.Scan(
		&comment.ID,
		&comment.WorkspaceID,
		&comment.DocumentID,
		&partID,
		&comment.TargetType,
		&comment.TargetID,
		&comment.Kind,
		&comment.Status,
		&comment.AuthorID,
		&comment.Body,
		&documentVersion,
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

	comment.PartID = partID
	comment.DocumentVersion = documentVersion
	comment.PartVersion = partVersion
	comment.Anchor = easyjson.RawMessage(anchor)
	comment.Metadata = easyjson.RawMessage(metadata)
	if len(comment.Metadata) == 0 {
		comment.Metadata = easyjson.RawMessage(`{}`)
	}
	if err := json.Unmarshal(assignees, &comment.AssigneeIDs); err != nil {
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
		&item.ChangedBy,
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
		&item.EditedBy,
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
