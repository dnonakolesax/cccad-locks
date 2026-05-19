package configs

import (
	"strings"
	"time"

	"github.com/dnonakolesax/viper"
)

const (
	postgresAddressKey          = "postgres.address"
	postgresDefaultAddress      = "postgres"
	postgresPortKey             = "postgres.port"
	postgresDefaultPort         = 5432
	postgresDBNameKey           = "postgres.database"
	postgresDefaultDBName       = "keycloak"
	postgresLoginKey            = "postgres.user"
	postgresPasswordKey         = "postgres_password"
	postgresRequestsPathKey     = "postgres.requests-path"
	postgresDefaultRequestsPath = "./sql_requests"
	postgresRolePath            = "database/kc-selector"
)

const (
	postgresConnTimeoutKey           = "postgres.conn-timeout"
	postgresDefaultConnTimeout       = 10 * time.Second
	postgresMinConnsKey              = "postgres.min-conns"
	postgresDefaultMinConns          = 5
	postgresMaxConnsKey              = "postgres.max-conns"
	postgresDefaultMaxConns          = 20
	postgresMaxConnLifetimeKey       = "postgres.max-conn-lifetime"
	postgresDefaultMaxConnLifetime   = time.Hour
	postgresMaxConnIdleTimeKey       = "postgres.max-conn-idle-time"
	postgresDefaultMaxConnIdleTime   = 30 * time.Minute
	postgresHealthCheckPeriodKey     = "postgres.healthcheck-period"
	postgresDefaultHealthCheckPeriod = time.Minute
	postgresRequestTimeoutKey        = "postgres.request-timeout"
	postgresDefaultRequestTimeout    = 30 * time.Second
)

const (
	RedisAddressKey            = "redis.address"
	RedisDefaultAddress        = "redis"
	RedisPortKey               = "redis.port"
	RedisDefaultPort           = 6379
	RedisPasswordKey           = "secret/redis:password"
	RedisRequestTimeoutKey     = "redis.request-timeout"
	RedisDefaultRequestTimeout = 10 * time.Second
)

const rolePartsCount = 2

type RDBConfig struct {
	Address      string
	Port         uint
	DBName       string
	Login        string
	Password     string
	RequestsPath string

	ConnTimeout       time.Duration
	MinConns          int32
	MaxConns          int32
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
	RequestTimeout    time.Duration
}

type RedisConfig struct {
	Address        string
	Port           int
	Password       string
	RequestTimeout time.Duration
}

func (rc *RDBConfig) SetDefaults(v *viper.Viper) {
	v.SetDefault(postgresAddressKey, postgresDefaultAddress)
	v.SetDefault(postgresPortKey, postgresDefaultPort)
	v.SetDefault(postgresDBNameKey, postgresDefaultDBName)
	v.SetDefault(postgresLoginKey, nil)
	v.SetDefault(postgresPasswordKey, nil)
	v.SetDefault(postgresRequestsPathKey, postgresDefaultRequestsPath)

	v.SetDefault(postgresConnTimeoutKey, postgresDefaultConnTimeout)
	v.SetDefault(postgresMinConnsKey, postgresDefaultMinConns)
	v.SetDefault(postgresMaxConnsKey, postgresDefaultMaxConns)
	v.SetDefault(postgresMaxConnLifetimeKey, postgresDefaultMaxConnLifetime)
	v.SetDefault(postgresMaxConnIdleTimeKey, postgresDefaultMaxConnIdleTime)
	v.SetDefault(postgresHealthCheckPeriodKey, postgresDefaultHealthCheckPeriod)
	v.SetDefault(postgresRequestTimeoutKey, postgresDefaultRequestTimeout)
}

func (rc *RDBConfig) Load(v *viper.Viper) {
	rc.Address = v.GetString(postgresAddressKey)
	rc.Port = v.GetUint(postgresPortKey)
	rc.DBName = v.GetString(postgresDBNameKey)
	roleString := v.GetString(postgresRolePath)
	roleStringSplitted := strings.Split(roleString, ":")
	if len(roleStringSplitted) == rolePartsCount {
		rc.Login = roleStringSplitted[0]
		rc.Password = roleStringSplitted[1]
	} else {
		rc.Login = v.GetString(postgresLoginKey)
		rc.Password = v.GetString(postgresPasswordKey)
	}
	rc.RequestsPath = v.GetString(postgresRequestsPathKey)

	rc.ConnTimeout = v.GetDuration(postgresConnTimeoutKey)
	rc.MinConns = v.GetInt32(postgresMinConnsKey)
	rc.MaxConns = v.GetInt32(postgresMaxConnsKey)
	rc.MaxConnLifetime = v.GetDuration(postgresMaxConnLifetimeKey)
	rc.MaxConnIdleTime = v.GetDuration(postgresMaxConnIdleTimeKey)
	rc.HealthCheckPeriod = v.GetDuration(postgresHealthCheckPeriodKey)
	rc.RequestTimeout = v.GetDuration(postgresRequestTimeoutKey)
}

func (rc *RedisConfig) SetDefaults(v *viper.Viper) {
	v.SetDefault(RedisAddressKey, RedisDefaultAddress)
	v.SetDefault(RedisPortKey, RedisDefaultPort)
	v.SetDefault(RedisPasswordKey, "")
	v.SetDefault(RedisRequestTimeoutKey, RedisDefaultRequestTimeout)
}

func (rc *RedisConfig) Load(v *viper.Viper) {
	rc.Address = v.GetString(RedisAddressKey)
	rc.Port = v.GetInt(RedisPortKey)
	rc.Password = v.GetString(RedisPasswordKey)
	rc.RequestTimeout = v.GetDuration(RedisRequestTimeoutKey)
}
