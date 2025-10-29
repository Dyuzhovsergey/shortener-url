// Package config for params
package config

type ShortenerConfig struct {
	CharSet  string
	LengthID int
}

var Param = &ShortenerConfig{
	CharSet:  "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789",
	LengthID: 8,
}
