// Package issuer é a Lambda que troca um CPF por um JWT do cliente (POST /auth/token).
package issuer

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"

	"github.com/Kc1t/postech-tc4-lambda-auth/internal/cpf"
	"github.com/Kc1t/postech-tc4-lambda-auth/internal/mask"
	"github.com/Kc1t/postech-tc4-lambda-auth/internal/requester"
	"github.com/Kc1t/postech-tc4-lambda-auth/internal/telemetry"
	"github.com/Kc1t/postech-tc4-lambda-auth/internal/token"
)

// Códigos de erro: os mesmos nomes do os-service, no mesmo envelope {"error":{"code","message"}}.
const (
	CodeValidation  = "VALIDATION_ERROR"
	CodeNotFound    = "NOT_FOUND"
	CodeForbidden   = "FORBIDDEN"
	CodeUnavailable = "UPSTREAM_UNAVAILABLE"
	CodeInternal    = "INTERNAL_ERROR"

	correlationHeader = "x-correlation-id"
)

// Finder é a consulta do cliente pelo documento.
type Finder interface {
	FindByDocument(ctx context.Context, document string) (requester.Requester, error)
}

type Handler struct {
	requesters Finder
	issuer     *token.Issuer
	logger     *slog.Logger
	now        func() time.Time
}

func New(requesters Finder, issuer *token.Issuer, logger *slog.Logger) *Handler {
	return &Handler{requesters: requesters, issuer: issuer, logger: logger, now: time.Now}
}

type request struct {
	CPF string `json:"cpf"`
}

type Response struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresAt   string `json:"expires_at"`
	ExpiresIn   int    `json:"expires_in"`
}

type ErrorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (h *Handler) Handle(ctx context.Context, event events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	corr := CorrelationID(event)
	ctx = requester.WithCorrelationID(ctx, corr)
	logger := telemetry.Logger(ctx, h.logger).With("correlation_id", corr)
	telemetry.Attr(ctx, "correlation_id", corr)

	reply := func(status int, payload any, outcome string) (events.APIGatewayV2HTTPResponse, error) {
		telemetry.Attr(ctx, "auth.outcome", outcome)
		return JSON(status, payload, corr)
	}

	body, err := decode(event)
	if err != nil {
		logger.Warn("payload invalido")
		return reply(400, errorBody(CodeValidation, "payload invalido"), "invalid_payload")
	}

	document, err := cpf.Validate(body.CPF)
	if err != nil {
		logger.Warn("cpf invalido")
		return reply(400, errorBody(CodeValidation, err.Error()), "invalid_cpf")
	}
	logger = logger.With("document", mask.Document(document))

	found, err := h.requesters.FindByDocument(ctx, document)
	switch {
	case errors.Is(err, requester.ErrNotFound):
		logger.Warn("cliente nao encontrado")
		return reply(404, errorBody(CodeNotFound, err.Error()), "not_found")
	case errors.Is(err, requester.ErrInactive):
		logger.Warn("cliente inativo", "requester_id", found.ID)
		return reply(403, errorBody(CodeForbidden, err.Error()), "inactive")
	case errors.Is(err, requester.ErrUnavailable):
		logger.Error("os-service indisponivel", "error", err)
		telemetry.NoticeError(ctx, err)
		return reply(503, errorBody(CodeUnavailable, "servico de clientes indisponivel, tente novamente"), "upstream_unavailable")
	case err != nil:
		logger.Error("falha ao consultar cliente", "error", err)
		telemetry.NoticeError(ctx, err)
		return reply(500, errorBody(CodeInternal, "erro interno"), "error")
	}

	now := h.now()
	signed, expiresAt, err := h.issuer.Issue(token.Subject{
		ID: found.ID, Name: found.Name, Email: found.Email, Document: document,
	}, now)
	if err != nil {
		logger.Error("falha ao assinar token", "error", err)
		telemetry.NoticeError(ctx, err)
		return reply(500, errorBody(CodeInternal, "erro interno"), "error")
	}

	telemetry.Attr(ctx, "requester_id", found.ID)
	logger.Info("token emitido", "requester_id", found.ID)

	return reply(200, Response{
		AccessToken: signed,
		TokenType:   "Bearer",
		ExpiresAt:   expiresAt.UTC().Format(time.RFC3339),
		ExpiresIn:   int(expiresAt.Sub(now).Seconds()),
	}, "issued")
}

func decode(event events.APIGatewayV2HTTPRequest) (request, error) {
	raw := []byte(event.Body)
	if event.IsBase64Encoded {
		decoded, err := base64.StdEncoding.DecodeString(event.Body)
		if err != nil {
			return request{}, err
		}
		raw = decoded
	}
	var body request
	if err := json.Unmarshal(raw, &body); err != nil {
		return request{}, err
	}
	return body, nil
}

// CorrelationID usa o header do cliente quando vier; senão, o request id do API Gateway.
func CorrelationID(event events.APIGatewayV2HTTPRequest) string {
	for key, value := range event.Headers {
		if value != "" && strings.EqualFold(key, correlationHeader) {
			return value
		}
	}
	return event.RequestContext.RequestID
}

func errorBody(code, message string) ErrorBody {
	var body ErrorBody
	body.Error.Code = code
	body.Error.Message = message
	return body
}

// JSON monta a resposta do API Gateway com o correlation id de volta.
func JSON(status int, payload any, corr string) (events.APIGatewayV2HTTPResponse, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return events.APIGatewayV2HTTPResponse{StatusCode: 500}, err
	}
	headers := map[string]string{"Content-Type": "application/json", "Cache-Control": "no-store"}
	if corr != "" {
		headers["X-Correlation-ID"] = corr
	}
	return events.APIGatewayV2HTTPResponse{StatusCode: status, Headers: headers, Body: string(encoded)}, nil
}
