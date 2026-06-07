package configs

import (
	"time"

	"github.com/dnonakolesax/viper"
)

const (
	geometryAddressKey            = "geometry.address"
	geometryDefaultAddress        = "geometry-kernel:7701"
	geometryRequestTimeoutKey     = "geometry.request-timeout"
	geometryDefaultRequestTimeout = 30 * time.Second
)

type GeometryConfig struct {
	Address        string
	RequestTimeout time.Duration
}

func (gc *GeometryConfig) SetDefaults(v *viper.Viper) {
	v.SetDefault(geometryAddressKey, geometryDefaultAddress)
	v.SetDefault(geometryRequestTimeoutKey, geometryDefaultRequestTimeout)
}

func (gc *GeometryConfig) Load(v *viper.Viper) {
	gc.Address = v.GetString(geometryAddressKey)
	gc.RequestTimeout = v.GetDuration(geometryRequestTimeoutKey)
}
