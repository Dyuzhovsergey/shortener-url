package repository

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
)

func BenchmarkFileRepository_Save(b *testing.B) {
	path := filepath.Join(b.TempDir(), "storage.ndjson")

	repo, err := NewFileRepository(path)
	if err != nil {
		b.Fatalf("NewFileRepository error: %v", err)
	}

	ctx := context.Background()
	userID := "bench-user"

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		shortID := fmt.Sprintf("id%08d", i)
		original := fmt.Sprintf("https://example.com/%d", i)
		if err := repo.Save(ctx, shortID, original, userID); err != nil {
			b.Fatalf("Save error: %v", err)
		}
	}
}
