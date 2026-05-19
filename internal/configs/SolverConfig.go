package configs

import (
	"time"

	"github.com/dnonakolesax/viper"
)

const (
	solverAddressKey            = "solver.address"
	solverDefaultAddress        = "sk-solver:7701"
	solverRequestTimeoutKey     = "solver.request-timeout"
	solverDefaultRequestTimeout = 10 * time.Second
)

type SolverConfig struct {
	Address        string
	RequestTimeout time.Duration
}

func (sc *SolverConfig) SetDefaults(v *viper.Viper) {
	v.SetDefault(solverAddressKey, solverDefaultAddress)
	v.SetDefault(solverRequestTimeoutKey, solverDefaultRequestTimeout)
}

func (sc *SolverConfig) Load(v *viper.Viper) {
	sc.Address = v.GetString(solverAddressKey)
	sc.RequestTimeout = v.GetDuration(solverRequestTimeoutKey)
}
