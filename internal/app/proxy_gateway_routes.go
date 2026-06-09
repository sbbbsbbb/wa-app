package app

import (
	"context"
	"strings"
	"time"

	waappv1 "github.com/byte-v-forge/wa-app/gen/go/byte/v/forge/waapp/v1"
)

func (s *Server) LoginStateCheckProxyRoute(ctx context.Context, correlationID string, ttl time.Duration) (DynamicProxyRoute, error) {
	if s == nil || s.proxyRuntime == nil {
		return DynamicProxyRoute{}, NewError(waappv1.WaErrorCode_WA_ERROR_CODE_ROUTE_UNAVAILABLE, "WA proxy runtime is not configured", false)
	}
	username := strings.TrimSpace(s.loginStateCheckProxyUsername)
	if username == "" {
		return DynamicProxyRoute{}, NewError(waappv1.WaErrorCode_WA_ERROR_CODE_ROUTE_UNAVAILABLE, "WA login-state proxy username is not configured", false)
	}
	return s.proxyRuntime.GatewayProxyRoute(ctx, username, DynamicProxyRouteRequest{
		Purpose:       "WA_LOGIN_STATE_CHECK",
		CorrelationID: correlationID,
		TTL:           ttl,
		Mode:          DynamicProxySessionModeRotating,
	})
}

func (s *Server) ReleaseProxyRoute(ctx context.Context, route DynamicProxyRoute) {
	s.releaseGatewayProxyRoute(ctx, route, "WA_PROXY_ROUTE")
}
