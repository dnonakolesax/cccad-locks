package sql

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dnonakolesax/cccad-locks/internal/configs"
	"github.com/dnonakolesax/cccad-locks/internal/consts"
)

const (
	addressLoggerKey = "address"
	sqlLoggerKey     = "sql"
)

const sqlFileExtension = ".sql"

type PGXConn struct {
	pool           *pgxpool.Pool
	requestTimeout time.Duration
	logger         *slog.Logger
	conf           configs.RDBConfig
}

type RDBError struct {
	Type  string
	Field string
}

func (err RDBError) Error() string {
	return err.Type + " " + err.Field
}

type PGXResponse struct {
	rows   pgx.Rows
	cancel context.CancelFunc
}

func NewPGXConn(config configs.RDBConfig, logger *slog.Logger) (*PGXConn, error) {
	connString := fmt.Sprintf("postgres://%v:%v@%v:%v/%v?sslmode=disable",
		config.Login,
		config.Password,
		config.Address,
		config.Port,
		config.DBName)

	pgxConfig, err := pgxpool.ParseConfig(connString)

	if err != nil {
		return nil, err
	}

	pgxConfig.ConnConfig.ConnectTimeout = config.ConnTimeout

	pgxConfig.MaxConns = config.MaxConns
	pgxConfig.MinConns = config.MinConns
	pgxConfig.MaxConnLifetime = config.MaxConnLifetime
	pgxConfig.MaxConnIdleTime = config.MaxConnIdleTime
	pgxConfig.HealthCheckPeriod = config.HealthCheckPeriod

	ctx, cancel := context.WithTimeout(context.Background(), config.ConnTimeout)
	defer cancel()

	logger.Info("Starting pgxpool", slog.String(addressLoggerKey, config.Address))
	pool, err := pgxpool.NewWithConfig(ctx, pgxConfig)
	if err != nil {
		logger.Error("Error while starting pgxpool", slog.String(addressLoggerKey, config.Address),
			slog.String(consts.ErrorLoggerKey, err.Error()))
		return nil, fmt.Errorf("CreatePool error: %w", err)
	}
	logger.Info("Started pgxpool", slog.String(addressLoggerKey, config.Address))

	poolCtx, poolCancel := context.WithTimeout(context.Background(), config.RequestTimeout)
	defer poolCancel()
	logger.Info("Trying to ping pgsql", slog.String(addressLoggerKey, config.Address))
	err = pool.Ping(poolCtx)

	if err != nil {
		logger.Error("Error while pinging pgxpool", slog.String(addressLoggerKey, config.Address),
			slog.String(consts.ErrorLoggerKey, err.Error()))
		return nil, fmt.Errorf("ping error: %w", err)
	}
	logger.Info("Pgsql ping success", slog.String(addressLoggerKey, config.Address))

	return &PGXConn{pool: pool, requestTimeout: config.RequestTimeout, logger: logger, conf: config}, nil
}

func (pc *PGXConn) Disconnect() {
	pc.logger.Info("closing connection to pgsql")
	pc.pool.Close()
}

type PGXWorker struct {
	Conn         *PGXConn
	ConnUpdating *atomic.Bool
	Requests     map[string]string
	Alive        *atomic.Bool
}

func NewPGXWorker(conn *PGXConn, alive *atomic.Bool, vaultChan chan string) (*PGXWorker, error) {
	requests, err := LoadSQLRequests(conn.conf.RequestsPath)
	if err != nil {
		return nil, err
	}

	alive.Store(true)

	worker := &PGXWorker{
		Conn:         conn,
		Requests:     requests,
		Alive:        alive,
		ConnUpdating: &atomic.Bool{},
	}

	go worker.MonitorVault(vaultChan)

	return worker, nil
}

func (pw *PGXWorker) Request(name string) (string, error) {
	request, ok := pw.Requests[name]
	if !ok {
		return "", fmt.Errorf("sql request %q not found", name)
	}

	return request, nil
}

func LoadSQLRequests(dirPath string) (map[string]string, error) {
	files, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	sqlRequests := make(map[string]string)

	for _, file := range files {
		if file.IsDir() {
			continue // Пропускаем директории
		}

		if filepath.Ext(file.Name()) != sqlFileExtension {
			continue // Пропускаем файлы без .sql расширения
		}

		filePath := filepath.Join(dirPath, file.Name())
		content, fileErr := os.ReadFile(filePath)
		if fileErr != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", file.Name(), fileErr)
		}

		// Получаем имя файла без расширения
		fileName := strings.TrimSuffix(file.Name(), filepath.Ext(file.Name()))
		sqlRequests[fileName] = string(content)
	}

	return sqlRequests, nil
}

func (pw *PGXWorker) MonitorVault(vaultChan chan string) {
	for newPassword := range vaultChan {
		newConf := pw.Conn.conf
		lp := strings.Split(newPassword, ":")
		if len(lp) != 2 { //nolint:mnd // 2 потому что логин и пароль
			pw.Conn.logger.Error("invalid password format received from vault")
			continue
		}
		newConf.Login = lp[0]
		newConf.Password = lp[1]
		pw.Conn.Disconnect()
		newConn, err := NewPGXConn(newConf, pw.Conn.logger)
		if err != nil {
			pw.Conn.logger.Error("Error creating new pgsql conn from vault credentials",
				slog.String(consts.ErrorLoggerKey, err.Error()))
			continue
		}
		for !pw.ConnUpdating.CompareAndSwap(false, true) {
			runtime.Gosched()
		}
		pw.Conn = newConn
		pw.ConnUpdating.Store(false)
	}
}

func (pw *PGXWorker) Exec(ctx context.Context, sql string, args ...interface{}) error {
	timeCtx, cancel := context.WithTimeout(ctx, pw.Conn.requestTimeout)
	defer cancel()

	// pw.Conn.logger.DebugContext(ctx, "executing sql", slog.String(sqlLoggerKey, sql))
	if pw.ConnUpdating.Load() {
		for pw.ConnUpdating.Load() {
		}
	}
	_, err := pw.Conn.pool.Exec(timeCtx, sql, args...)

	var pgErr *pgconn.PgError

	if err != nil {
		pw.Alive.Store(false)
		pw.Conn.logger.ErrorContext(ctx, "failed executing sql", slog.String(sqlLoggerKey, sql),
			slog.String(consts.ErrorLoggerKey, err.Error()))
		rdbErr := new(RDBError)
		if errors.As(err, &pgErr) {
			rdbErr.Type = pgErr.Code
			rdbErr.Field = pgErr.ColumnName
		} else {
			rdbErr.Field = err.Error()
		}
		return rdbErr
	}
	// pw.Conn.logger.DebugContext(ctx, "done executing sql", slog.String(sqlLoggerKey, sql))

	return nil
}

func (pw *PGXWorker) Query(ctx context.Context, sql string, args ...interface{}) (*PGXResponse, error) {
	timeCtx, cancel := context.WithTimeout(ctx, pw.Conn.requestTimeout)
	// pw.Conn.logger.DebugContext(ctx, "executing sql", slog.String(sqlLoggerKey, sql))
	if pw.ConnUpdating.Load() {
		for pw.ConnUpdating.Load() {
		}
	}
	result, err := pw.Conn.pool.Query(timeCtx, sql, args...)

	var pgErr *pgconn.PgError

	if err != nil {
		cancel()
		pw.Alive.Store(false)
		pw.Conn.logger.ErrorContext(ctx, "failed executing sql", slog.String(sqlLoggerKey, sql),
			slog.String(consts.ErrorLoggerKey, err.Error()))
		rdbErr := new(RDBError)
		if errors.As(err, &pgErr) {
			rdbErr.Type = pgErr.Code
			rdbErr.Field = pgErr.ColumnName
		} else {
			rdbErr.Field = err.Error()
		}
		return nil, rdbErr
	}
	// pw.Conn.logger.DebugContext(ctx, "done executing sql", slog.String(sqlLoggerKey, sql))

	return &PGXResponse{rows: result, cancel: cancel}, nil
}

func (pw *PGXWorker) WithTx(ctx context.Context, fn func(context.Context, pgx.Tx) error) error {
	timeCtx, cancel := context.WithTimeout(ctx, pw.Conn.requestTimeout)
	defer cancel()

	if pw.ConnUpdating.Load() {
		for pw.ConnUpdating.Load() {
		}
	}

	tx, err := pw.Conn.pool.Begin(timeCtx)
	if err != nil {
		return err
	}

	if err := fn(timeCtx, tx); err != nil {
		_ = tx.Rollback(timeCtx)
		return err
	}

	return tx.Commit(timeCtx)
}

func (pr *PGXResponse) Next() bool {
	return pr.rows.Next()
}

func (pr *PGXResponse) Scan(dest ...any) error {
	err := pr.rows.Scan(dest...)
	if err != nil {
		return fmt.Errorf("scan error: %w", err)
	}

	return nil
}

func (pr *PGXResponse) Close() error {
	pr.rows.Close()
	// Err() on the returned Rows must be checked after the Rows is closed to determine
	// if the query executed successfully as some errors can only be detected by reading the entire response.
	// e.g. A divide by zero error on the last row.
	err := pr.rows.Err()
	if pr.cancel != nil {
		pr.cancel()
	}
	if err != nil {
		return err
	}
	return nil
}
