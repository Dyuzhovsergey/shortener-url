package middleware

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// простой тестовый хендлер, который возвращает JSON
func testHandler(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// возвращаем то же тело, что получили (эхо)
	w.Write(body)
}

func TestGzipMiddleware(t *testing.T) {
	handler := GzipMiddleware(http.HandlerFunc(testHandler))
	server := httptest.NewServer(handler)
	defer server.Close()

	requestBody := `{"message":"hello world"}`

	t.Run("sends gzip (client -> server)", func(t *testing.T) {
		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)

		_, err := zw.Write([]byte(requestBody))
		require.NoError(t, err)
		require.NoError(t, zw.Close())

		req, err := http.NewRequest(http.MethodPost, server.URL, &buf)
		require.NoError(t, err)

		req.Header.Set("Content-Encoding", "gzip")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		require.JSONEq(t, requestBody, string(body))
	})

	t.Run("accepts gzip (server -> client)", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPost, server.URL, bytes.NewBufferString(requestBody))
		require.NoError(t, err)

		req.Header.Set("Accept-Encoding", "gzip")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, "gzip", resp.Header.Get("Content-Encoding"))

		zr, err := gzip.NewReader(resp.Body)
		require.NoError(t, err)
		defer zr.Close()

		body, err := io.ReadAll(zr)
		require.NoError(t, err)

		require.JSONEq(t, requestBody, string(body))
	})
}
