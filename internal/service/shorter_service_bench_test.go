package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/Dyuzhovsergey/shortener-url/internal/config"
	"github.com/Dyuzhovsergey/shortener-url/internal/repository"
)

func BenchmarkShorterService_CreateShortURL_MemoryRepo(b *testing.B) {
	cfg := &config.ShortenerConfig{
		CharSet:  "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789",
		LengthID: 8,
		BaseURL:  "http://localhost:8080",
	}

	repo := repository.NewMemoryRepository()
	svc := NewShorterService(repo, cfg)
	b.Cleanup(func() { close(svc.deleteCh) })

	ctx := context.Background()
	base := "http://localhost:8080"

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		original := fmt.Sprintf("https://example.com/path/%d", i)
		_, _ = svc.CreateShortURL(ctx, original, base)
	}
}
