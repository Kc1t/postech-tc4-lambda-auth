package runtime

import (
	"log/slog"
	"testing"
)

func TestEnv(t *testing.T) {
	t.Setenv("AUTH_TESTE", "valor")
	if Env("AUTH_TESTE", "padrao") != "valor" || Env("AUTH_TESTE_AUSENTE", "padrao") != "padrao" {
		t.Fatal("Env não respeitou o valor ou o padrão")
	}
}

func TestLoggerNivelInvalido(t *testing.T) {
	t.Setenv("LOG_LEVEL", "barulhento")
	if Logger() == nil {
		t.Fatal("logger nil")
	}
	t.Setenv("JWT_SECRET_TESTE", "x")
	if Require(Logger(), "JWT_SECRET_TESTE") != "x" {
		t.Fatal("Require deveria devolver o valor")
	}
}

func TestRenameMessage(t *testing.T) {
	if got := renameMessage(nil, slog.String(slog.MessageKey, "auth_issued")); got.Key != "message" {
		t.Fatalf("msg na raiz vira message: %v", got)
	}
	if got := renameMessage([]string{"g"}, slog.String(slog.MessageKey, "x")); got.Key != slog.MessageKey {
		t.Fatalf("dentro de grupo não muda: %v", got)
	}
	if got := renameMessage(nil, slog.String("cpf", "***")); got.Key != "cpf" {
		t.Fatalf("outras chaves não mudam: %v", got)
	}
}
