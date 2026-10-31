package authorizer_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"

	"github.com/Kc1t/postech-tc4-lambda-auth/internal/authorizer"
	"github.com/Kc1t/postech-tc4-lambda-auth/internal/token"
)

func event(headers map[string]string) events.APIGatewayV2CustomAuthorizerV2Request {
	return events.APIGatewayV2CustomAuthorizerV2Request{Headers: headers, RouteKey: "GET /api/v1/service-orders"}
}

func TestHandle(t *testing.T) {
	issuer := token.NewIssuer("segredo", time.Minute)
	signed, _, err := issuer.Issue(token.Subject{ID: "req-1", Document: "52998224725"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	expired, _, _ := issuer.Issue(token.Subject{ID: "req-1"}, time.Now().Add(-time.Hour))
	other, _, _ := token.NewIssuer("outro", time.Minute).Issue(token.Subject{ID: "req-1"}, time.Now())

	h := authorizer.New(issuer, slog.New(slog.NewJSONHandler(io.Discard, nil)))

	cases := []struct {
		name    string
		headers map[string]string
		allowed bool
	}{
		{"valido", map[string]string{"authorization": "Bearer " + signed}, true},
		{"caixa diferente", map[string]string{"Authorization": "bearer " + signed}, true},
		{"sem header", map[string]string{}, false},
		{"sem esquema", map[string]string{"authorization": signed}, false},
		{"token vazio", map[string]string{"authorization": "Bearer  "}, false},
		{"expirado", map[string]string{"authorization": "Bearer " + expired}, false},
		{"outro segredo", map[string]string{"authorization": "Bearer " + other}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := h.Handle(context.Background(), event(tc.headers))
			if err != nil {
				t.Fatal(err)
			}
			if resp.IsAuthorized != tc.allowed {
				t.Fatalf("IsAuthorized = %v, esperava %v", resp.IsAuthorized, tc.allowed)
			}
			if tc.allowed && (resp.Context["subject"] != "req-1" || resp.Context["role"] != "client" || resp.Context["document"] != "52998224725") {
				t.Errorf("contexto: %+v", resp.Context)
			}
		})
	}
}
