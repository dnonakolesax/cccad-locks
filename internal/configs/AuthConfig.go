package configs

import (
	"time"

	"github.com/dnonakolesax/viper"
)

const (
	authAddressKey            = "auth.address"
	authDefaultAddress        = "auth:7701"
	authRequestTimeoutKey     = "auth.request-timeout"
	authDefaultRequestTimeout = 5 * time.Second
)

type AuthConfig struct {
	Address        string
	RequestTimeout time.Duration
}

func (ac *AuthConfig) SetDefaults(v *viper.Viper) {
	v.SetDefault(authAddressKey, authDefaultAddress)
	v.SetDefault(authRequestTimeoutKey, authDefaultRequestTimeout)
}

func (ac *AuthConfig) Load(v *viper.Viper) {
	ac.Address = v.GetString(authAddressKey)
	ac.RequestTimeout = v.GetDuration(authRequestTimeoutKey)
}
