package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"

	"github.com/dnonakolesax/cccad-locks/internal/configs"
	"github.com/dnonakolesax/viper"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	"github.com/pressly/goose/v3"
)

const (
	defaultConfigDir     = "./configs"
	defaultMigrationsDir = "./migrations"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	configDir := flag.String("configs", defaultConfigDir, "Path to configs")
	migrationsDir := flag.String("dir", defaultMigrationsDir, "Path to goose migrations")
	dsn := flag.String("dsn", "", "Postgres DSN. Defaults to DATABASE_URL, then postgres config")
	flag.Parse()

	command := "up"
	if flag.NArg() > 0 {
		command = flag.Arg(0)
	}

	db, err := openDB(*dsn, *configDir)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			log.Printf("close db: %v", closeErr)
		}
	}()

	err = goose.SetDialect("postgres")
	if err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}

	args := flag.Args()[1:]
	err = goose.RunContext(context.Background(), command, db, *migrationsDir, args...)
	if err != nil {
		return fmt.Errorf("goose %s: %w", command, err)
	}

	return nil
}

func openDB(flagDSN, configDir string) (*sql.DB, error) {
	dsn := strings.TrimSpace(flagDSN)
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		cfg, err := loadPostgresConfig(configDir)
		if err != nil {
			return nil, err
		}
		dsn = postgresDSN(cfg)
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	err = db.PingContext(context.Background())
	if err != nil {
		if closeErr := db.Close(); closeErr != nil {
			return nil, fmt.Errorf("ping db: %w; close db: %w", err, closeErr)
		}
		return nil, err
	}

	return db, nil
}

func loadPostgresConfig(configDir string) (*configs.RDBConfig, error) {
	_ = godotenv.Load()

	v := viper.New()
	v.PanicOnNil = true
	cfg := &configs.RDBConfig{
		DBName:   "cccad",
		Address:  "gobddocker-postgres-1",
		Port:     5432,
		Login:    "kopilka",
		Password: "12345",
	}
	// cfg.SetDefaults(v)

	// logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	// if err := configs.Load(configDir, v, logger, nil, nil, cfg); err != nil {
	// 	return nil, err
	// }

	// if strings.TrimSpace(cfg.Login) == "" || strings.TrimSpace(cfg.DBName) == "" {
	// 	return nil, errors.New("postgres config is incomplete; provide -dsn or DATABASE_URL")
	// }

	return cfg, nil
}

func postgresDSN(cfg *configs.RDBConfig) string {
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(cfg.Login, cfg.Password),
		Host:   fmt.Sprintf("%s:%d", cfg.Address, cfg.Port),
		Path:   cfg.DBName,
	}
	q := u.Query()
	q.Set("sslmode", "disable")
	u.RawQuery = q.Encode()

	return u.String()
}
