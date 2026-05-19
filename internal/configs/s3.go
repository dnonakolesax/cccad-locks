package configs

import (
	"github.com/dnonakolesax/viper"
)

const (
	s3AddrKey       = "s3.address"
	s3DefaultAddr   = "https://storage.yandexcloud.net"
	s3BucketKey     = "s3.bucket"
	s3DefaultBucket = "cccad"
)

type S3Config struct {
	Addr   string
	Bucket string
}

func (sc *S3Config) SetDefaults(v *viper.Viper) {
	v.SetDefault(s3AddrKey, s3DefaultAddr)
	v.SetDefault(s3BucketKey, s3DefaultBucket)
}

func (sc *S3Config) Load(v *viper.Viper) {
	sc.Addr = v.GetString(s3AddrKey)
	sc.Bucket = v.GetString(s3BucketKey)
}
