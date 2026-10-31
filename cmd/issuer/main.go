// Lambda emissora: POST /auth/token troca o CPF do cliente por um JWT.
package main

import (
	"net/http"
	"os"
	"time"

	"github.com/newrelic/go-agent/v3/newrelic"

	"github.com/Kc1t/postech-tc4-lambda-auth/internal/issuer"
	"github.com/Kc1t/postech-tc4-lambda-auth/internal/requester"
	"github.com/Kc1t/postech-tc4-lambda-auth/internal/runtime"
	"github.com/Kc1t/postech-tc4-lambda-auth/internal/token"
)

func main() {
	logger := runtime.Logger()

	ttl, err := time.ParseDuration(runtime.Env("JWT_TTL", "15m"))
	if err != nil {
		logger.Error("JWT_TTL invalido", "error", err)
		os.Exit(1)
	}
	timeout, err := time.ParseDuration(runtime.Env("UPSTREAM_TIMEOUT", "3s"))
	if err != nil {
		logger.Error("UPSTREAM_TIMEOUT invalido", "error", err)
		os.Exit(1)
	}

	// O RoundTripper do New Relic cria o segmento externo e injeta os headers do trace
	// distribuído, então a chamada aparece ligada ao os-service no mesmo trace.
	httpClient := &http.Client{Timeout: timeout, Transport: newrelic.NewRoundTripper(http.DefaultTransport)}

	h := issuer.New(
		requester.NewClient(
			runtime.Require(logger, "OS_SERVICE_URL"),
			runtime.Require(logger, "INTERNAL_API_TOKEN"),
			httpClient,
		),
		token.NewIssuer(runtime.Require(logger, "JWT_SECRET"), ttl),
		logger,
	)

	runtime.Start(logger, h.Handle)
}
