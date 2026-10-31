package requester_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Kc1t/postech-tc4-lambda-auth/internal/requester"
)

func server(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func TestFindByDocument_Active(t *testing.T) {
	srv := server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/requesters/52998224725" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.Header.Get(requester.TokenHeader) != "s3cr3t" {
			t.Errorf("token interno ausente")
		}
		if r.Header.Get(requester.CorrelationHeader) != "corr-1" {
			t.Errorf("correlation id não propagado")
		}
		_, _ = w.Write([]byte(`{"id":"req-1","name":"João Silva","email":"joao@email.com","document":"52998224725","status":"active"}`))
	})

	ctx := requester.WithCorrelationID(context.Background(), "corr-1")
	found, err := requester.NewClient(srv.URL+"/", "s3cr3t", srv.Client()).FindByDocument(ctx, "52998224725")
	if err != nil {
		t.Fatalf("esperava sucesso, veio %v", err)
	}
	if found.ID != "req-1" || found.Name != "João Silva" || found.Email != "joao@email.com" {
		t.Errorf("dados inesperados: %+v", found)
	}
}

func TestFindByDocument_Errors(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   error
		calls  int32
	}{
		{"nao encontrado", http.StatusNotFound, `{"error":{"code":"NOT_FOUND"}}`, requester.ErrNotFound, 1},
		{"inativo", http.StatusOK, `{"id":"req-1","status":"inactive"}`, requester.ErrInactive, 1},
		{"5xx tenta de novo", http.StatusServiceUnavailable, ``, requester.ErrUnavailable, 2},
		{"token errado nao repete", http.StatusUnauthorized, ``, nil, 1},
		{"json invalido", http.StatusOK, `{`, nil, 1},
		{"sem id", http.StatusOK, `{"status":"active"}`, nil, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			srv := server(t, func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			})
			c := requester.NewClient(srv.URL, "s3cr3t", srv.Client(), requester.WithRetry(2, time.Millisecond))
			_, err := c.FindByDocument(context.Background(), "52998224725")
			if err == nil {
				t.Fatal("esperava erro")
			}
			if tc.want != nil && !errors.Is(err, tc.want) {
				t.Errorf("erro = %v, esperava %v", err, tc.want)
			}
			if got := calls.Load(); got != tc.calls {
				t.Errorf("chamadas = %d, esperava %d", got, tc.calls)
			}
		})
	}
}

func TestFindByDocument_RecuperaNaSegundaTentativa(t *testing.T) {
	var calls atomic.Int32
	srv := server(t, func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(`{"id":"req-1","status":"active"}`))
	})
	c := requester.NewClient(srv.URL, "s3cr3t", srv.Client(), requester.WithRetry(2, time.Millisecond))
	if _, err := c.FindByDocument(context.Background(), "52998224725"); err != nil {
		t.Fatalf("esperava sucesso na segunda tentativa: %v", err)
	}
}

func TestFindByDocument_RedeFora(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()

	c := requester.NewClient(url, "s3cr3t", http.DefaultClient, requester.WithRetry(2, time.Millisecond))
	_, err := c.FindByDocument(context.Background(), "52998224725")
	if !errors.Is(err, requester.ErrUnavailable) {
		t.Fatalf("esperava ErrUnavailable, veio %v", err)
	}
}

func TestFindByDocument_ContextoCancelado(t *testing.T) {
	srv := server(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	ctx, cancel := context.WithCancel(context.Background())
	c := requester.NewClient(srv.URL, "s3cr3t", srv.Client(), requester.WithRetry(3, time.Hour))
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	_, err := c.FindByDocument(ctx, "52998224725")
	if !errors.Is(err, requester.ErrUnavailable) || !errors.Is(err, context.Canceled) {
		t.Fatalf("esperava indisponível por cancelamento, veio %v", err)
	}
}
