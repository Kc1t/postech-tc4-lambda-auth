package issuer_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"

	"github.com/Kc1t/postech-tc4-lambda-auth/internal/issuer"
	"github.com/Kc1t/postech-tc4-lambda-auth/internal/requester"
	"github.com/Kc1t/postech-tc4-lambda-auth/internal/token"
)

type fakeFinder struct {
	found requester.Requester
	err   error
	got   string
}

func (f *fakeFinder) FindByDocument(_ context.Context, document string) (requester.Requester, error) {
	f.got = document
	return f.found, f.err
}

func newHandler(f issuer.Finder) (*issuer.Handler, *token.Issuer) {
	tk := token.NewIssuer("segredo", 15*time.Minute)
	return issuer.New(f, tk, slog.New(slog.NewJSONHandler(io.Discard, nil))), tk
}

func call(t *testing.T, h *issuer.Handler, event events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, issuer.ErrorBody) {
	t.Helper()
	resp, err := h.Handle(context.Background(), event)
	if err != nil {
		t.Fatal(err)
	}
	var body issuer.ErrorBody
	_ = json.Unmarshal([]byte(resp.Body), &body)
	return resp, body
}

func TestHandle_EmiteToken(t *testing.T) {
	f := &fakeFinder{found: requester.Requester{ID: "req-1", Name: "Ana", Email: "ana@x.com", Status: "active"}}
	h, tk := newHandler(f)

	resp, _ := call(t, h, events.APIGatewayV2HTTPRequest{
		Body:    `{"cpf":"529.982.247-25"}`,
		Headers: map[string]string{"X-Correlation-ID": "corr-9"},
	})
	if resp.StatusCode != 200 {
		t.Fatalf("status %d: %s", resp.StatusCode, resp.Body)
	}
	if f.got != "52998224725" {
		t.Errorf("documento enviado ao os-service: %q", f.got)
	}
	if resp.Headers["X-Correlation-ID"] != "corr-9" || resp.Headers["Cache-Control"] != "no-store" {
		t.Errorf("headers: %+v", resp.Headers)
	}

	var out issuer.Response
	if err := json.Unmarshal([]byte(resp.Body), &out); err != nil {
		t.Fatal(err)
	}
	if out.TokenType != "Bearer" || out.ExpiresIn != 900 {
		t.Errorf("resposta: %+v", out)
	}
	claims, err := tk.Verify(out.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Subject != "req-1" || claims.Role != "client" || claims.Document != "52998224725" {
		t.Errorf("claims: %+v", claims)
	}
}

func TestHandle_Base64(t *testing.T) {
	h, _ := newHandler(&fakeFinder{found: requester.Requester{ID: "req-1", Status: "active"}})
	resp, _ := call(t, h, events.APIGatewayV2HTTPRequest{
		Body:            base64.StdEncoding.EncodeToString([]byte(`{"cpf":"52998224725"}`)),
		IsBase64Encoded: true,
	})
	if resp.StatusCode != 200 {
		t.Fatalf("status %d: %s", resp.StatusCode, resp.Body)
	}
}

func TestHandle_Erros(t *testing.T) {
	cases := []struct {
		name   string
		body   string
		b64    bool
		err    error
		status int
		code   string
	}{
		{"json invalido", `{`, false, nil, 400, issuer.CodeValidation},
		{"base64 invalido", `%%%`, true, nil, 400, issuer.CodeValidation},
		{"cpf invalido", `{"cpf":"111.111.111-11"}`, false, nil, 400, issuer.CodeValidation},
		{"nao encontrado", `{"cpf":"52998224725"}`, false, requester.ErrNotFound, 404, issuer.CodeNotFound},
		{"inativo", `{"cpf":"52998224725"}`, false, requester.ErrInactive, 403, issuer.CodeForbidden},
		{"indisponivel", `{"cpf":"52998224725"}`, false, requester.ErrUnavailable, 503, issuer.CodeUnavailable},
		{"outro erro", `{"cpf":"52998224725"}`, false, errors.New("os-service respondeu 401"), 500, issuer.CodeInternal},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, _ := newHandler(&fakeFinder{err: tc.err})
			resp, body := call(t, h, events.APIGatewayV2HTTPRequest{
				Body:            tc.body,
				IsBase64Encoded: tc.b64,
				RequestContext:  events.APIGatewayV2HTTPRequestContext{RequestID: "req-abc"},
			})
			if resp.StatusCode != tc.status || body.Error.Code != tc.code {
				t.Fatalf("status %d code %q, esperava %d %q", resp.StatusCode, body.Error.Code, tc.status, tc.code)
			}
			if resp.Headers["X-Correlation-ID"] != "req-abc" {
				t.Errorf("correlation id deveria cair no request id: %+v", resp.Headers)
			}
		})
	}
}

func TestJSON_PayloadInvalido(t *testing.T) {
	resp, err := issuer.JSON(200, make(chan int), "")
	if err == nil || resp.StatusCode != 500 {
		t.Fatalf("esperava erro de serialização, veio %d %v", resp.StatusCode, err)
	}
}
