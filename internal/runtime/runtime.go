// Package runtime tem o que os dois mains compartilham: logger, env e o start com New Relic.
package runtime

import (
	"log/slog"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/newrelic/go-agent/v3/integrations/nrlambda"
	"github.com/newrelic/go-agent/v3/newrelic"
)

// Logger em JSON no stdout; a extension do New Relic encaminha os logs da function.
func Logger() *slog.Logger {
	var level slog.Level
	if err := level.UnmarshalText([]byte(Env("LOG_LEVEL", "info"))); err != nil {
		level = slog.LevelInfo
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)
	return logger
}

func Env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// Require encerra o processo se a variável obrigatória não estiver definida: melhor falhar
// no cold start do que emitir token com segredo vazio.
func Require(logger *slog.Logger, key string) string {
	value := os.Getenv(key)
	if value == "" {
		logger.Error("variavel obrigatoria ausente", "env", key)
		os.Exit(1)
	}
	return value
}

// Start sobe o handler com o nrlambda quando o New Relic está configurado (layer da
// extension + NEW_RELIC_ACCOUNT_ID); sem ele, sobe direto.
func Start(logger *slog.Logger, handler any) {
	app, err := newrelic.NewApplication(nrlambda.ConfigOption())
	if err != nil {
		logger.Warn("new relic desabilitado", "error", err)
		lambda.Start(handler)
		return
	}
	nrlambda.Start(handler, app)
}
