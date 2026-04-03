package middleware

import (
	"context"
	"strings"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

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

		userID, ok := ParseAndVerifyAuthValue(rawAuth)
		if !ok || userID == "" {
			var err error
			userID, err = GenerateUserID()
			if err != nil {
				if logger != nil {
					logger.Error("failed to generate user id", zap.Error(err))
				}
				return nil, status.Error(codes.Internal, "failed to generate user id")
			}
		}

		authValue := BuildAuthValue(userID)
		ctx = WithUserID(ctx, userID)

		if err := grpc.SetHeader(ctx, metadata.Pairs("authorization", authValue)); err != nil && logger != nil {
			logger.Warn("failed to set grpc authorization header", zap.Error(err))
		}

		return handler(ctx, req)
	}
}