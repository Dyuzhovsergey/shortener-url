// Package model for struct JSON API
package model

// ShortenRequest и ShortenResponse структуры для JSON API
type ShortenRequest struct {
	URL string `json:"url"`
}

type ShortenResponse struct {
	Result string `json:"result"`
}

// BatchShortenRequestItem — элемент батч-запроса.
type BatchShortenRequestItem struct {
	CorrelationID string `json:"correlation_id"`
	OriginalURL   string `json:"original_url"`
}

// BatchShortenResponseItem — элемент батч-ответа.
type BatchShortenResponseItem struct {
	CorrelationID string `json:"correlation_id"`
	ShortURL      string `json:"short_url"`
}
