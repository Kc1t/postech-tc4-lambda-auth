// Package authorizer é o Lambda authorizer (payload 2.0, resposta simples) do API Gateway.
// Aceita o JWT do cliente (emitido pelo issuer) e o da equipe (emitido pelo os-service).
package authorizer

import (
	"context"
	"log/slog"
	"strings"

	"github.com/aws/aws-lambda-go/events"

	"github.com/Kc1t/postech-tc4-lambda-auth/internal/telemetry"
	"github.com/Kc1t/postech-tc4-lambda-auth/internal/token"
)

type Handler struct {
	issuer *token.Issuer
	logger *slog.Logger
}

func New(issuer *token.Issuer, logger *slog.Logger) *Handler {
	return &Handler{issuer: issuer, logger: logger}
}

func (h *Handler) Handle(ctx context.Context, event events.APIGatewayV2CustomAuthorizerV2Request) (events.APIGatewayV2CustomAuthorizerSimpleResponse, error) {
	logger := telemetry.Logger(ctx, h.logger).With("request_id", event.RequestContext.RequestID, "route", event.RouteKey)

	signed, ok := BearerToken(event.Headers)
	if !ok {
		logger.Warn("authorization header ausente ou malformado")
		telemetry.Attr(ctx, "auth.outcome", "missing_token")
		return deny(), nil
	}

	claims, err := h.issuer.Verify(signed)
	if err != nil {
		logger.Warn("token rejeitado", "error", err)
		telemetry.Attr(ctx, "auth.outcome", "invalid_token")
		return deny(), nil
	}

	telemetry.Attr(ctx, "auth.outcome", "allowed")
	telemetry.Attr(ctx, "auth.role", claims.Role)
	logger.Info("token aceito", "subject", claims.Subject, "role", claims.Role)

	return events.APIGatewayV2CustomAuthorizerSimpleResponse{
		IsAuthorized: true,
		Context: map[string]interface{}{
			"subject":  claims.Subject,
			"role":     claims.Role,
			"document": claims.Document,
		},
	}, nil
}

// BearerToken extrai o token de "Authorization: Bearer <token>" (header em qualquer caixa).
func BearerToken(headers map[string]string) (string, bool) {
	for key, value := range headers {
		if !strings.EqualFold(key, "authorization") {
			continue
		}
		parts := strings.SplitN(value, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") || strings.TrimSpace(parts[1]) == "" {
			return "", false
		}
		return strings.TrimSpace(parts[1]), true
	}
	return "", false
}

func deny() events.APIGatewayV2CustomAuthorizerSimpleResponse {
	return events.APIGatewayV2CustomAuthorizerSimpleResponse{IsAuthorized: false}
}
