package model

// ShortenRequest — тело запроса для POST /api/shorten.
type ShortenRequest struct {
	URL string `json:"url"`
}

// ShortenResponse — ответ для POST /api/shorten.
type ShortenResponse struct {
	Result string `json:"result"`
}

// StatsResponse — ответ для GET /api/internal/stats.
type StatsResponse struct {
	URLs  int `json:"urls"`
	Users int `json:"users"`
}

// BatchShortenRequestItem — элемент батч-запроса POST /api/shorten/batch.
type BatchShortenRequestItem struct {
	CorrelationID string `json:"correlation_id"`
	OriginalURL   string `json:"original_url"`
}

// BatchShortenResponseItem — элемент батч-ответа POST /api/shorten/batch.
type BatchShortenResponseItem struct {
	CorrelationID string `json:"correlation_id"`
	ShortURL      string `json:"short_url"`
}

// UserURLResponse — элемент ответа GET /api/user/urls.
type UserURLResponse struct {
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
}
