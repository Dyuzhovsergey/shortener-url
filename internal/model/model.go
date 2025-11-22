// Package model for struct JSON API
package model

// ShortenRequest и ShortenResponse структуры для JSON API
type ShortenRequest struct {
	URL string `json:"url"`
}

type ShortenResponse struct {
	Result string `json:"result"`
}
