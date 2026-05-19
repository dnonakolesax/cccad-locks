package configs

import (
	"time"

	"github.com/dnonakolesax/viper"
)

const (
	servicePortKey                = "service.port"
	servicePortDefault            = 7800
	serviceBasePathKey            = "service.base-path"
	serviceBasePathDefault        = "/sketches"
	serviceMetricsPortKey         = "service.metrics-port"
	serviceMetricsPortDefault     = 7801
	serviceGRPCPortKey            = "service.grpc-port"
	serviceGRPCPortDefault        = 7802
	serviceMetricsEndpointKey     = "service.metrics-endpoint"
	serviceMetricsEndpointDefault = "/metrics"
)

const (
	logLevelKey           = "service.log-level"
	logLevelDefault       = "info"
	logAddSourceKey       = "service.log-add-source"
	logAddSourceDefault   = true
	logTimeoutKey         = "service.log-timeout"
	logTimeoutDefault     = 10 * time.Second
	logMaxFileSizeKey     = "service.log-max-file-size"
	logMaxFileSizeDefault = 100
	logMaxBackupsKey      = "service.log-max-backups"
	logMaxBackupsDefault  = 3
	logMaxAgeKey          = "service.log-max-age"
	logMaxAgeDefault      = 28
)

type LoggerConfig struct {
	LogLevel       string
	LogTimeout     time.Duration
	LogAddSource   bool
	LogMaxFileSize int
	LogMaxBackups  int
	LogMaxAge      int
}

type ServiceConfig struct {
	BasePath        string
	Port            int
	MetricsPort     int
	GRPCPort        int
	MetricsEndpoint string
}

func (lc *LoggerConfig) SetDefaults(v *viper.Viper) {
	v.SetDefault(logLevelKey, logLevelDefault)
	v.SetDefault(logAddSourceKey, logAddSourceDefault)
	v.SetDefault(logTimeoutKey, logTimeoutDefault)
	v.SetDefault(logMaxFileSizeKey, logMaxFileSizeDefault)
	v.SetDefault(logMaxBackupsKey, logMaxBackupsDefault)
	v.SetDefault(logMaxAgeKey, logMaxAgeDefault)
}

func (lc *LoggerConfig) Load(v *viper.Viper) {
	lc.LogLevel = v.GetString(logLevelKey)
	lc.LogAddSource = v.GetBool(logAddSourceKey)
	lc.LogTimeout = v.GetDuration(logTimeoutKey)
	lc.LogMaxFileSize = v.GetInt(logMaxFileSizeKey)
	lc.LogMaxBackups = v.GetInt(logMaxBackupsKey)
	lc.LogMaxAge = v.GetInt(logMaxAgeKey)
}

func (sc *ServiceConfig) SetDefaults(v *viper.Viper) {
	v.SetDefault(servicePortKey, servicePortDefault)
	v.SetDefault(serviceBasePathKey, serviceBasePathDefault)
	v.SetDefault(serviceMetricsPortKey, serviceMetricsPortDefault)
	v.SetDefault(serviceGRPCPortKey, serviceGRPCPortDefault)
	v.SetDefault(serviceMetricsEndpointKey, serviceMetricsEndpointDefault)
}

func (sc *ServiceConfig) Load(v *viper.Viper) {
	sc.Port = v.GetInt(servicePortKey)
	sc.BasePath = v.GetString(serviceBasePathKey)
	sc.MetricsPort = v.GetInt(serviceMetricsPortKey)
	sc.GRPCPort = v.GetInt(serviceGRPCPortKey)
	sc.MetricsEndpoint = v.GetString(serviceMetricsEndpointKey)
}
