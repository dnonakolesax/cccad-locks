package configs

import (
	"time"

	"github.com/dnonakolesax/viper"
)

const (
	sHTTPReadTimeoutKey               = "http-server.read-timeout"
	sHTTPReadTimeoutDefault           = time.Second * 5
	sHTTPWriteTimeoutKey              = "http-server.write-timeout" //nolint:gosec // false positive Potential hardcoded credentials
	sHTTPWriteTimeoutDefault          = time.Second * 10
	sHTTPIdleTimeoutKey               = "http-server.idle-timeout"
	sHTTPIdleTimeoutDefault           = time.Second * 30
	sHTTPMaxIdleWorkerDurationKey     = "http-server.max-idle-worker-duration"
	sHTTPDefaultWorkerDurationDefault = time.Second * 10
	sHTTPMaxReqBodySizeKey            = "http-server.max-req-body-size"
	sHTTPMaxReqBodySizeDefault        = 4 * 1024 * 1024
	sHTTPReadBufferSizeKey            = "http-server.read-buffer-size"
	sHTTPReadBufferSizeDefault        = 8 * 1024
	sHTTPWriteBufferSizeKey           = "http-server.write-buffer-size" //nolint:gosec // false positive Potential hardcoded credentials
	sHTTPWriteBufferSizeDefault       = 4 * 1024
	sHTTPConcurrencyKey               = "http-server.concurrency"
	sHTTPConcurrencyDefault           = 256 * 1024
	sHTTPMaxConnsPerIPKey             = "http-server.max-conns-per-ip"
	sHTTPMaxConnsPerIPDefault         = 100
	sHTTPMaxRequestsPerConnKey        = "http-server.max-requests-per-conn"
	sHTTPMaxRequestsPerConnDefault    = 1000
	sHTTPTCPKeepAlivePeriod           = "http-server.tcp-keepalive-period"
	sHTTPTCPKeepAliveDefault          = time.Minute * 3
)

type HTTPServerConfig struct {
	ReadTimeout           time.Duration
	WriteTimeout          time.Duration
	IdleTimeout           time.Duration
	MaxIdleWorkerDuration time.Duration
	MaxReqBodySize        int
	ReadBufferSize        int
	WriteBufferSize       int
	Concurrency           int
	MaxConnsPerIP         int
	MaxRequestsPerConn    int
	TCPKeepAlivePeriod    time.Duration
}

func (hc *HTTPServerConfig) Load(v *viper.Viper) {
	hc.ReadTimeout = v.GetDuration(sHTTPReadTimeoutKey)
	hc.WriteTimeout = v.GetDuration(sHTTPWriteTimeoutKey)
	hc.IdleTimeout = v.GetDuration(sHTTPIdleTimeoutKey)
	hc.MaxIdleWorkerDuration = v.GetDuration(sHTTPMaxIdleWorkerDurationKey)
	hc.MaxReqBodySize = v.GetInt(sHTTPMaxReqBodySizeKey)
	hc.ReadBufferSize = v.GetInt(sHTTPReadBufferSizeKey)
	hc.WriteBufferSize = v.GetInt(sHTTPWriteBufferSizeKey)
	hc.Concurrency = v.GetInt(sHTTPConcurrencyKey)
	hc.MaxConnsPerIP = v.GetInt(sHTTPMaxConnsPerIPKey)
	hc.MaxRequestsPerConn = v.GetInt(sHTTPMaxRequestsPerConnKey)
	hc.TCPKeepAlivePeriod = v.GetDuration(sHTTPTCPKeepAlivePeriod)
}

func (hc *HTTPServerConfig) SetDefaults(v *viper.Viper) {
	v.SetDefault(sHTTPReadTimeoutKey, sHTTPReadTimeoutDefault)
	v.SetDefault(sHTTPWriteTimeoutKey, sHTTPWriteTimeoutDefault)
	v.SetDefault(sHTTPIdleTimeoutKey, sHTTPIdleTimeoutDefault)
	v.SetDefault(sHTTPMaxIdleWorkerDurationKey, sHTTPDefaultWorkerDurationDefault)
	v.SetDefault(sHTTPMaxReqBodySizeKey, sHTTPMaxReqBodySizeDefault)
	v.SetDefault(sHTTPReadBufferSizeKey, sHTTPReadBufferSizeDefault)
	v.SetDefault(sHTTPWriteBufferSizeKey, sHTTPWriteBufferSizeDefault)
	v.SetDefault(sHTTPConcurrencyKey, sHTTPConcurrencyDefault)
	v.SetDefault(sHTTPMaxConnsPerIPKey, sHTTPMaxConnsPerIPDefault)
	v.SetDefault(sHTTPMaxRequestsPerConnKey, sHTTPMaxRequestsPerConnDefault)
	v.SetDefault(sHTTPTCPKeepAlivePeriod, sHTTPTCPKeepAliveDefault)
}
