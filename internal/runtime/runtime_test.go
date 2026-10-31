package runtime

import "testing"

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
