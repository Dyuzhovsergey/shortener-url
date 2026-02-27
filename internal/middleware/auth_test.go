package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCookieValue_RoundTrip(t *testing.T) {
	userID := "user-123"

	val := buildCookieValue(userID)

	got, ok := parseAndVerifyCookie(val)
	if !ok {
		t.Fatalf("ожидали ok=true, получили ok=false")
	}
	if got != userID {
		t.Fatalf("ожидали userID %q, получили %q", userID, got)
	}
}

func TestCookieValue_TamperedSignature(t *testing.T) {
	userID := "user-123"
	val := buildCookieValue(userID)

	tampered := val + "x"

	_, ok := parseAndVerifyCookie(tampered)
	if ok {
		t.Fatalf("ожидали ok=false для битой подписи, получили ok=true")
	}
}

func TestAuthMiddleware_NoCookie_SetsCookieAndContext(t *testing.T) {
	var gotUserID string

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, ok := UserIDFromContext(r.Context())
		if !ok || uid == "" {
			t.Fatalf("ожидали user_id в контексте, получили пусто")
		}
		gotUserID = uid
		_, _ = w.Write([]byte(uid))
	})

	handler := AuthMiddleware(next)

	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	resp := rr.Result()
	defer resp.Body.Close()

	cookies := resp.Cookies()
	if len(cookies) == 0 {
		t.Fatalf("ожидали, что middleware установит cookie, но cookies пустые")
	}

	if strings.TrimSpace(gotUserID) == "" {
		t.Fatalf("ожидали непустой user_id")
	}
}

func TestAuthMiddleware_ValidCookie_ReusesUserID(t *testing.T) {
	// 1) Первый запрос — middleware создаёт cookie.
	var userID1 string
	next1 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, ok := UserIDFromContext(r.Context())
		if !ok || uid == "" {
			t.Fatalf("ожидали user_id в контексте")
		}
		userID1 = uid
	})

	h1 := AuthMiddleware(next1)
	req1 := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	rr1 := httptest.NewRecorder()
	h1.ServeHTTP(rr1, req1)

	cookies := rr1.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatalf("ожидали cookie после первого запроса")
	}
	c := cookies[0]

	// 2) Второй запрос — кладём полученную cookie, user_id должен совпасть.
	var userID2 string
	next2 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, ok := UserIDFromContext(r.Context())
		if !ok || uid == "" {
			t.Fatalf("ожидали user_id в контексте")
		}
		userID2 = uid
	})

	h2 := AuthMiddleware(next2)
	req2 := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	req2.AddCookie(c)
	rr2 := httptest.NewRecorder()
	h2.ServeHTTP(rr2, req2)

	if userID1 == "" || userID2 == "" {
		t.Fatalf("ожидали оба user_id непустыми")
	}
	if userID1 != userID2 {
		t.Fatalf("ожидали одинаковый user_id, получили %q и %q", userID1, userID2)
	}
}
