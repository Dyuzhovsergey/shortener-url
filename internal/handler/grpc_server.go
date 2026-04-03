package handler

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	pb "github.com/Dyuzhovsergey/shortener-url/api"
	"github.com/Dyuzhovsergey/shortener-url/internal/audit"
	"github.com/Dyuzhovsergey/shortener-url/internal/middleware"
	"github.com/Dyuzhovsergey/shortener-url/internal/repository"
	"github.com/Dyuzhovsergey/shortener-url/internal/service"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// GRPCServer реализует gRPC API сервиса сокращения URL.
type GRPCServer struct {
	pb.UnimplementedShortenerServiceServer

	baseURL string
	shorter *service.ShorterService
	logger  *zap.Logger
	audit   *audit.Publisher
}

// NewGRPCServer создаёт gRPC-сервер поверх уже существующего service-слоя.
func NewGRPCServer(baseURL string, shorter *service.ShorterService, logger *zap.Logger, auditor *audit.Publisher) *GRPCServer {
	return &GRPCServer{
		baseURL: strings.TrimRight(baseURL, "/"),
		shorter: shorter,
		logger:  logger,
		audit:   auditor,
	}
}

// ShortenURL соответствует HTTP-эндпоинту POST /api/shorten.
func (s *GRPCServer) ShortenURL(ctx context.Context, req *pb.URLShortenRequest) (*pb.URLShortenResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	originalURL := strings.TrimSpace(req.GetUrl())
	if originalURL == "" {
		return nil, status.Error(codes.InvalidArgument, "url is required")
	}

	shortURL, err := s.shorter.CreateShortURL(ctx, originalURL, s.baseURL)
	if err != nil {
		if errors.Is(err, service.ErrAlreadyExists) && shortURL != "" {
			s.publishAudit(ctx, "shorten", originalURL)

			resp := pb.URLShortenResponse_builder{
				Result: proto.String(shortURL),
			}.Build()
			return resp, nil
		}

		errText := strings.ToLower(err.Error())
		if strings.Contains(errText, "invalid url") || strings.Contains(errText, "unsupported url scheme") {
			return nil, status.Error(codes.InvalidArgument, "invalid URL format")
		}

		if s.logger != nil {
			s.logger.Error("failed to create short url", zap.Error(err))
		}

		return nil, status.Error(codes.Internal, "internal error")
	}

	s.publishAudit(ctx, "shorten", originalURL)

	resp := pb.URLShortenResponse_builder{
		Result: proto.String(shortURL),
	}.Build()

	return resp, nil
}

// ExpandURL соответствует HTTP-эндпоинту GET /{id}.
func (s *GRPCServer) ExpandURL(ctx context.Context, req *pb.URLExpandRequest) (*pb.URLExpandResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	shortID := strings.TrimSpace(req.GetId())
	if shortID == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	originalURL, ok, err := s.shorter.GetOriginalURL(ctx, shortID)
	if err != nil {
		if errors.Is(err, repository.ErrDeleted) {
			return nil, status.Error(codes.FailedPrecondition, "short url is deleted")
		}

		if s.logger != nil {
			s.logger.Error("failed to expand short url", zap.Error(err), zap.String("short_id", shortID))
		}

		return nil, status.Error(codes.Internal, "internal error")
	}

	if !ok {
		return nil, status.Error(codes.NotFound, "short url not found")
	}

	s.publishAudit(ctx, "follow", originalURL)

	resp := pb.URLExpandResponse_builder{
		Result: proto.String(originalURL),
	}.Build()
	return resp, nil
}

// ListUserURLs соответствует HTTP-эндпоинту GET /api/user/urls.
func (s *GRPCServer) ListUserURLs(ctx context.Context, _ *pb.ListUserURLsRequest) (*pb.UserURLsResponse, error) {
	userURLs, err := s.shorter.GetUserURLs(ctx)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("failed to list user urls", zap.Error(err))
		}
		return nil, status.Error(codes.Internal, "internal error")
	}

	items := make([]*pb.URLData, 0, len(userURLs))

	for _, u := range userURLs {
		shortURL, err := url.JoinPath(s.baseURL, u.ShortID)
		if err != nil {
			if s.logger != nil {
				s.logger.Error("failed to build short url", zap.Error(err), zap.String("short_id", u.ShortID))
			}
			return nil, status.Error(codes.Internal, "internal error")
		}

		item := pb.URLData_builder{
			ShortUrl:    proto.String(shortURL),
			OriginalUrl: proto.String(u.OriginalURL),
		}.Build()

		items = append(items, item)
	}

	resp := pb.UserURLsResponse_builder{
		Url: items,
	}.Build()

	return resp, nil
}

// publishAudit публикует событие аудита, если аудит включён.
func (s *GRPCServer) publishAudit(ctx context.Context, action, originalURL string) {
	if s.audit == nil {
		return
	}

	userID, _ := middleware.UserIDFromContext(ctx)

	ev := audit.Event{
		TS:     time.Now().Unix(),
		Action: action,
		UserID: userID,
		URL:    originalURL,
	}

	if err := s.audit.Publish(ctx, ev); err != nil && s.logger != nil {
		s.logger.Warn("audit publish failed", zap.Error(err))
	}
}
