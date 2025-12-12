package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
)

const userIDCookieName = "user_id"

// секрет для подписи
var userSecretKey = []byte("super-secret-user-key")

// ключ для context.Context
type userIDKey struct{}

// UserIDFromContext достаёт userID из контекста.
func UserIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(userIDKey{}).(string)
	if !ok || id == "" {
		return "", false
	}
	return id, true
}

// generateUserID генерирует случайный userID (32 hex-символа).
func generateUserID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// signUserID строит HMAC-подпись от userID.
func signUserID(userID string) string {
	mac := hmac.New(sha256.New, userSecretKey)
	mac.Write([]byte(userID))
	return hex.EncodeToString(mac.Sum(nil))
}

// buildCookieValue = userID.signature
func buildCookieValue(userID string) string {
	sig := signUserID(userID)
	return userID + "." + sig
}

// parseAndVerifyCookie разбирает значение куки и проверяет подпись.
func parseAndVerifyCookie(val string) (string, bool) {
	parts := strings.Split(val, ".")
	if len(parts) != 2 {
		return "", false
	}
	userID, sigHex := parts[0], parts[1]
	if userID == "" || sigHex == "" {
		return "", false
	}
	expected := signUserID(userID)
	// сравниваем подпись в константное время
	if !hmac.Equal([]byte(expected), []byte(sigHex)) {
		return "", false
	}
	return userID, true
}

// AuthMiddleware - выдаёт/проверяет подписанную куку и кладёт userID в контекст.
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var userID string

		// 1. Пытаемся прочитать существующую куку
		if c, err := r.Cookie(userIDCookieName); err == nil {
			if id, ok := parseAndVerifyCookie(c.Value); ok {
				userID = id
			}
		}

		// 2. Если куки нет или она невалидна — генерируем новую
		if userID == "" {
			id, err := generateUserID()
			if err != nil {
				http.Error(w, "cannot generate user id", http.StatusInternalServerError)
				return
			}
			userID = id
			val := buildCookieValue(userID)
			http.SetCookie(w, &http.Cookie{
				Name:     userIDCookieName,
				Value:    val,
				Path:     "/",
				HttpOnly: true,
				// Можно добавить Secure/SameSite по желанию
			})
		}

		// 3. Кладём userID в контекст и передаём дальше
		ctx := context.WithValue(r.Context(), userIDKey{}, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
