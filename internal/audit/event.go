package audit

// Event — событие аудита.
//
// Формат JSON:
//
//	{
//	  "ts": 12345678,
//	  "action": "shorten"|"follow",
//	  "user_id": "...",
//	  "url": "https://..."
//	}
type Event struct {
	TS     int64  `json:"ts"`
	Action string `json:"action"`
	UserID string `json:"user_id"`
	URL    string `json:"url"`
}
