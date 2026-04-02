package handler

import (
	"context"
	"errors"
	"strings"
	"time"

	pb "github.com/Dyuzhovsergey/shortener-url/api"
	"github.com/Dyuzhovsergey/shortener-url/internal/audit"
	"github.com/Dyuzhovsergey/shortener-url/internal/middleware"
	"github.com/Dyuzhovsergey/shortener-url/internal/repository"
	"github.com/Dyuzhovsergey/shortener-url/internal/service"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
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

// GRPCAuthInterceptor достаёт авторизационные данные из metadata "authorization".
// Если данные отсутствуют или невалидны, создаёт новый userID, кладёт его в context
// и возвращает клиенту новое значение authorization в header metadata.
func GRPCAuthInterceptor(logger *zap.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		var rawAuth string

		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if values := md.Get("authorization"); len(values) > 0 {
				rawAuth = strings.TrimSpace(values[0])
			}
		}

		userID, ok := middleware.ParseAndVerifyAuthValue(rawAuth)
		if !ok || userID == "" {
			var err error
			userID, err = middleware.GenerateUserID()
			if err != nil {
				if logger != nil {
					logger.Error("failed to generate user id", zap.Error(err))
				}
				return nil, status.Error(codes.Internal, "failed to generate user id")
			}
		}

		authValue := middleware.BuildAuthValue(userID)
		ctx = middleware.WithUserID(ctx, userID)

		if err := grpc.SetHeader(ctx, metadata.Pairs("authorization", authValue)); err != nil && logger != nil {
			logger.Warn("failed to set grpc authorization header", zap.Error(err))
		}

		return handler(ctx, req)
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

			resp := &pb.URLShortenResponse{
				Result: proto.String(shortURL),
			}
			return resp, nil
		}

		return nil, status.Error(codes.InvalidArgument, "invalid URL format")
	}

	s.publishAudit(ctx, "shorten", originalURL)

	resp := &pb.URLShortenResponse{
		Result: proto.String(shortURL),
	}
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
		return nil, status.Error(codes.Internal, "internal error")
	}

	if !ok {
		return nil, status.Error(codes.NotFound, "short url not found")
	}

	s.publishAudit(ctx, "follow", originalURL)

	resp := &pb.URLExpandResponse{
		Result: proto.String(originalURL),
	}
	return resp, nil
}

// ListUserURLs соответствует HTTP-эндпоинту GET /api/user/urls.
func (s *GRPCServer) ListUserURLs(ctx context.Context, _ *emptypb.Empty) (*pb.UserURLsResponse, error) {
	userURLs, err := s.shorter.GetUserURLs(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, "internal error")
	}

	items := make([]*pb.URLData, 0, len(userURLs))

	for _, u := range userURLs {
		item := &pb.URLData{
			ShortUrl:    proto.String(s.baseURL + "/" + u.ShortID),
			OriginalUrl: proto.String(u.OriginalURL),
		}
		items = append(items, item)
	}

	resp := &pb.UserURLsResponse{
		Url: items,
	}

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
