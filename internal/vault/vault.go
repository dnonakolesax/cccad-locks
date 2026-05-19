package vault

import (
	"context"
	"log/slog"
	"sync/atomic"

	"github.com/dnonakolesax/viper"
	"github.com/hashicorp/vault-client-go"
	"github.com/hashicorp/vault-client-go/schema"

	"github.com/dnonakolesax/cccad-locks/internal/consts"
)

type Client struct {
	Client      *vault.Client
	Healthcheck *atomic.Bool
	UpdateChan  chan viper.KVEntry
}

type Credentials struct {
	Login    string
	Password string
}

func SetupVault(addr string, creds *Credentials, logger *slog.Logger) (*Client, error) {
	vClient, err := vault.New(vault.WithAddress(addr))
	if err != nil {
		logger.Error("Coulndt create vault client", slog.String(consts.ErrorLoggerKey, err.Error()))
		return nil, err
	}
	resp, err := vClient.Auth.UserpassLogin(context.Background(), creds.Login, schema.UserpassLoginRequest{
		Password: creds.Password,
	})
	if err != nil {
		logger.Error("Coulndt create vault client", slog.String(consts.ErrorLoggerKey, err.Error()))
		return nil, err
	}
	err = vClient.SetToken(resp.Auth.ClientToken)
	if err != nil {
		logger.Error("Coulndt set vault token", slog.String(consts.ErrorLoggerKey, err.Error()))
		return nil, err
	}

	updateChan := make(chan viper.KVEntry)
	return &Client{
		Client:      vClient,
		Healthcheck: &atomic.Bool{},
		UpdateChan:  updateChan,
	}, nil
}
