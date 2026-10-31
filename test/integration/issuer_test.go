//go:build integration

// Teste de integração com o os-service de verdade (docker compose do os-service):
// CPF → issuer → GET /internal/requesters/{cpf} → JWT → rota do cliente no os-service.
//
//	OS_SERVICE_URL=http://localhost:8080 INTERNAL_API_TOKEN=local-internal-token \
//	JWT_SECRET=change-me-in-production TEST_CPF=52998224725 go test -tags integration ./test/integration/...
package integration

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"

	"github.com/Kc1t/postech-tc4-lambda-auth/internal/authorizer"
	"github.com/Kc1t/postech-tc4-lambda-auth/internal/issuer"
	"github.com/Kc1t/postech-tc4-lambda-auth/internal/requester"
	"github.com/Kc1t/postech-tc4-lambda-auth/internal/token"
)

func env(t *testing.T, key string) string {
	t.Helper()
	v := os.Getenv(key)
	if v == "" {
		t.Skipf("%s não definido", key)
	}
	return v
}

func TestCPFAteRotaDoCliente(t *testing.T) {
	baseURL := env(t, "OS_SERVICE_URL")
	internalToken := env(t, "INTERNAL_API_TOKEN")
	secret := env(t, "JWT_SECRET")
	cpf := env(t, "TEST_CPF")

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	tk := token.NewIssuer(secret, 15*time.Minute)
	client := requester.NewClient(baseURL, internalToken, &http.Client{Timeout: 3 * time.Second})
	h := issuer.New(client, tk, logger)

	resp, err := h.Handle(context.Background(), events.APIGatewayV2HTTPRequest{Body: `{"cpf":"` + cpf + `"}`})
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("issuer: %d %s %v", resp.StatusCode, resp.Body, err)
	}
	var out issuer.Response
	if err := json.Unmarshal([]byte(resp.Body), &out); err != nil {
		t.Fatal(err)
	}

	// O authorizer aceita o token que o issuer emitiu.
	auth, _ := authorizer.New(tk, logger).Handle(context.Background(), events.APIGatewayV2CustomAuthorizerV2Request{
		Headers: map[string]string{"authorization": "Bearer " + out.AccessToken},
	})
	if !auth.IsAuthorized {
		t.Fatal("authorizer recusou o token do issuer")
	}

	// E o os-service também (ele valida o JWT de novo, defesa em profundidade).
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, baseURL+"/api/v1/service-orders/requester?document="+cpf, nil)
	req.Header.Set("Authorization", "Bearer "+out.AccessToken)
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Body.Close() }()
	if r.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(r.Body)
		t.Fatalf("os-service recusou o token do cliente: %d %s", r.StatusCode, body)
	}

	// O token só vale para o próprio documento: consultar outro CPF é 403.
	req, _ = http.NewRequestWithContext(context.Background(), http.MethodGet, baseURL+"/api/v1/service-orders/requester?document=11144477735", nil)
	req.Header.Set("Authorization", "Bearer "+out.AccessToken)
	other, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = other.Body.Close() }()
	if other.StatusCode != http.StatusForbidden {
		t.Fatalf("esperava 403 ao consultar outro documento, veio %d", other.StatusCode)
	}

	// CPF válido sem cadastro → 404.
	resp, _ = h.Handle(context.Background(), events.APIGatewayV2HTTPRequest{Body: `{"cpf":"11144477735"}`})
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("esperava 404 para CPF sem cadastro, veio %d %s", resp.StatusCode, resp.Body)
	}

	// Token interno errado → 500 (erro de configuração, não do cliente).
	wrong := issuer.New(requester.NewClient(baseURL, "errado", http.DefaultClient), tk, logger)
	resp, _ = wrong.Handle(context.Background(), events.APIGatewayV2HTTPRequest{Body: `{"cpf":"` + cpf + `"}`})
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("esperava 500 com token interno errado, veio %d", resp.StatusCode)
	}
}
