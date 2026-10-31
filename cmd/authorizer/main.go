// Lambda authorizer do API Gateway: valida o JWT antes de a rota chegar aos serviços.
package main

import (
	"time"

	"github.com/Kc1t/postech-tc4-lambda-auth/internal/authorizer"
	"github.com/Kc1t/postech-tc4-lambda-auth/internal/runtime"
	"github.com/Kc1t/postech-tc4-lambda-auth/internal/token"
)

func main() {
	logger := runtime.Logger()
	// O TTL só importa para emitir; aqui o issuer é usado apenas para verificar.
	h := authorizer.New(token.NewIssuer(runtime.Require(logger, "JWT_SECRET"), time.Minute), logger)
	runtime.Start(logger, h.Handle)
}
