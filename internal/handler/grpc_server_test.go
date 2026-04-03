package handler

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	pb "github.com/Dyuzhovsergey/shortener-url/api"
	"github.com/Dyuzhovsergey/shortener-url/internal/audit"
	"github.com/Dyuzhovsergey/shortener-url/internal/config"
	"github.com/Dyuzhovsergey/shortener-url/internal/middleware"
	"github.com/Dyuzhovsergey/shortener-url/internal/repository"
	"github.com/Dyuzhovsergey/shortener-url/internal/service"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
)

const grpcBufSize = 1024 * 1024

func newGRPCTestClient(t *testing.T) (pb.ShortenerServiceClient, func()) {
	t.Helper()

	lis := bufconn.Listen(grpcBufSize)

	cfg := &config.ShortenerConfig{
		CharSet:       "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789",
		LengthID:      8,
		BaseURL:       "http://localhost:8080",
		RunAddr:       ":8080",
		TrustedSubnet: "",
	}

	repo := repository.NewMemoryRepository()
	shorter := service.NewShorterService(repo, cfg)

	grpcSrv := grpc.NewServer(
		grpc.UnaryInterceptor(middleware.GRPCAuthInterceptor(zap.NewNop())),
	)

	grpcHandler := NewGRPCServer(cfg.BaseURL, shorter, zap.NewNop(), audit.NewPublisher())
	pb.RegisterShortenerServiceServer(grpcSrv, grpcHandler)

	go func() {
		_ = grpcSrv.Serve(lis)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(
		ctx,
		"bufnet",
		grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		t.Fatalf("failed to dial bufconn grpc server: %v", err)
	}

	cleanup := func() {
		_ = conn.Close()
		grpcSrv.Stop()
		_ = lis.Close()
	}

	return pb.NewShortenerServiceClient(conn), cleanup
}

func TestGRPCServer_ShortenExpandAndListUserURLs(t *testing.T) {
	client, cleanup := newGRPCTestClient(t)
	defer cleanup()

	var header metadata.MD

	shortenResp, err := client.ShortenURL(
		context.Background(),
		&pb.URLShortenRequest{
			Url: proto.String("https://example.com/test"),
		},
		grpc.Header(&header),
	)
	if err != nil {
		t.Fatalf("ShortenURL returned error: %v", err)
	}

	shortURL := shortenResp.GetResult()
	if shortURL == "" {
		t.Fatal("expected non-empty short url")
	}

	authValues := header.Get("authorization")
	if len(authValues) == 0 || authValues[0] == "" {
		t.Fatal("expected authorization header from grpc server")
	}

	authCtx := metadata.AppendToOutgoingContext(context.Background(), "authorization", authValues[0])

	shortID := strings.TrimPrefix(shortURL, "http://localhost:8080/")

	expandResp, err := client.ExpandURL(
		authCtx,
		&pb.URLExpandRequest{
			Id: proto.String(shortID),
		},
	)
	if err != nil {
		t.Fatalf("ExpandURL returned error: %v", err)
	}

	if got := expandResp.GetResult(); got != "https://example.com/test" {
		t.Fatalf("unexpected original url: got %q, want %q", got, "https://example.com/test")
	}

	listResp, err := client.ListUserURLs(authCtx, &pb.ListUserURLsRequest{})
	if err != nil {
		t.Fatalf("ListUserURLs returned error: %v", err)
	}

	items := listResp.GetUrl()
	if len(items) != 1 {
		t.Fatalf("expected 1 url in user list, got %d", len(items))
	}

	if got := items[0].GetOriginalUrl(); got != "https://example.com/test" {
		t.Fatalf("unexpected original_url in list: got %q, want %q", got, "https://example.com/test")
	}

	if got := items[0].GetShortUrl(); got != shortURL {
		t.Fatalf("unexpected short_url in list: got %q, want %q", got, shortURL)
	}
}

func TestGRPCServer_ListUserURLs_IsSeparatedByAuthorization(t *testing.T) {
	client, cleanup := newGRPCTestClient(t)
	defer cleanup()

	var header1 metadata.MD
	resp1, err := client.ShortenURL(
		context.Background(),
		&pb.URLShortenRequest{
			Url: proto.String("https://example.com/user1"),
		},
		grpc.Header(&header1),
	)
	if err != nil {
		t.Fatalf("ShortenURL user1 returned error: %v", err)
	}
	if resp1.GetResult() == "" {
		t.Fatal("expected non-empty short url for user1")
	}

	auth1 := header1.Get("authorization")
	if len(auth1) == 0 || auth1[0] == "" {
		t.Fatal("expected authorization header for user1")
	}

	var header2 metadata.MD
	resp2, err := client.ShortenURL(
		context.Background(),
		&pb.URLShortenRequest{
			Url: proto.String("https://example.com/user2"),
		},
		grpc.Header(&header2),
	)
	if err != nil {
		t.Fatalf("ShortenURL user2 returned error: %v", err)
	}
	if resp2.GetResult() == "" {
		t.Fatal("expected non-empty short url for user2")
	}

	auth2 := header2.Get("authorization")
	if len(auth2) == 0 || auth2[0] == "" {
		t.Fatal("expected authorization header for user2")
	}

	ctxUser1 := metadata.AppendToOutgoingContext(context.Background(), "authorization", auth1[0])
	ctxUser2 := metadata.AppendToOutgoingContext(context.Background(), "authorization", auth2[0])

	list1, err := client.ListUserURLs(ctxUser1, &pb.ListUserURLsRequest{})
	if err != nil {
		t.Fatalf("ListUserURLs user1 returned error: %v", err)
	}

	list2, err := client.ListUserURLs(ctxUser2, &pb.ListUserURLsRequest{})
	if err != nil {
		t.Fatalf("ListUserURLs user2 returned error: %v", err)
	}

	items1 := list1.GetUrl()
	items2 := list2.GetUrl()

	if len(items1) != 1 {
		t.Fatalf("expected 1 url for user1, got %d", len(items1))
	}
	if len(items2) != 1 {
		t.Fatalf("expected 1 url for user2, got %d", len(items2))
	}

	if got := items1[0].GetOriginalUrl(); got != "https://example.com/user1" {
		t.Fatalf("unexpected user1 original_url: got %q", got)
	}
	if got := items2[0].GetOriginalUrl(); got != "https://example.com/user2" {
		t.Fatalf("unexpected user2 original_url: got %q", got)
	}
}
