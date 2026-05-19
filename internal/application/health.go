package application

import "sync/atomic"

type HealthChecks struct {
	Postgres *atomic.Bool
	Redis    *atomic.Bool
	S3       *atomic.Bool
	Vault    *atomic.Bool
}

func (a *App) SetupHealthChecks() {
	a.health = &HealthChecks{
		Postgres: &atomic.Bool{},
		Redis:    &atomic.Bool{},
		S3:       &atomic.Bool{},
		Vault:    &atomic.Bool{},
	}
}
