package audit

// Event — событие аудита.
//
// Поля:
//   - TS: unix timestamp события
//   - Action: действие ("shorten" или "follow")
//   - UserID: идентификатор пользователя (если есть)
//   - URL: оригинальный URL (не сокращённый)

type Event struct {
	TS     int64  `json:"ts"`
	Action string `json:"action"`
	UserID string `json:"user_id"`
	URL    string `json:"url"`
}
