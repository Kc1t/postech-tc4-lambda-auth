// Package requester consulta o cliente (dono do CPF) no os-service.
//
// Na Fase 3 a Lambda lia a tabela requesters direto no banco da aplicação. Com os
// microsserviços, cada serviço é dono dos próprios dados: a Lambda passa a chamar a rota
// interna GET /internal/requesters/{document} do os-service, protegida por X-Internal-Token.
package requester

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	ErrNotFound    = errors.New("cliente nao encontrado")
	ErrInactive    = errors.New("cliente inativo")
	ErrUnavailable = errors.New("os-service indisponivel")
)

const (
	statusActive = "active"

	TokenHeader       = "X-Internal-Token"
	CorrelationHeader = "X-Correlation-ID"
)

type Requester struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Document string `json:"document"`
	Status   string `json:"status"`
}

// Doer é o que o cliente precisa de um *http.Client (troca fácil nos testes e no New Relic).
type Doer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Client chama o os-service. Erros de rede e 5xx são tentados de novo (a consulta é
// idempotente); 4xx não.
type Client struct {
	baseURL  string
	token    string
	http     Doer
	attempts int
	backoff  time.Duration
}

type Option func(*Client)

// WithRetry define quantas tentativas no total e o intervalo entre elas.
func WithRetry(attempts int, backoff time.Duration) Option {
	return func(c *Client) {
		if attempts > 0 {
			c.attempts = attempts
		}
		c.backoff = backoff
	}
}

func NewClient(baseURL, token string, doer Doer, opts ...Option) *Client {
	c := &Client{
		baseURL:  strings.TrimRight(baseURL, "/"),
		token:    token,
		http:     doer,
		attempts: 2,
		backoff:  150 * time.Millisecond,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

type correlationKey struct{}

// WithCorrelationID guarda o correlation id no contexto para ir no header da chamada.
func WithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, correlationKey{}, id)
}

// FindByDocument devolve o cliente ativo dono do documento (só dígitos).
func (c *Client) FindByDocument(ctx context.Context, document string) (Requester, error) {
	var lastErr error
	for attempt := 1; attempt <= c.attempts; attempt++ {
		found, retry, err := c.find(ctx, document)
		if err == nil || !retry {
			return found, err
		}
		lastErr = err
		if attempt < c.attempts {
			select {
			case <-ctx.Done():
				return Requester{}, fmt.Errorf("%w: %w", ErrUnavailable, ctx.Err())
			case <-time.After(c.backoff):
			}
		}
	}
	return Requester{}, lastErr
}

func (c *Client) find(ctx context.Context, document string) (Requester, bool, error) {
	endpoint := c.baseURL + "/internal/requesters/" + url.PathEscape(document)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Requester{}, false, err
	}
	req.Header.Set(TokenHeader, c.token)
	req.Header.Set("Accept", "application/json")
	if id, ok := ctx.Value(correlationKey{}).(string); ok && id != "" {
		req.Header.Set(CorrelationHeader, id)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return Requester{}, ctx.Err() == nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch {
	case resp.StatusCode == http.StatusOK:
	case resp.StatusCode == http.StatusNotFound:
		return Requester{}, false, ErrNotFound
	case resp.StatusCode >= 500:
		_, _ = io.Copy(io.Discard, resp.Body)
		return Requester{}, true, fmt.Errorf("%w: status %d", ErrUnavailable, resp.StatusCode)
	default:
		// 400/401: token interno errado ou documento recusado. Não adianta repetir.
		_, _ = io.Copy(io.Discard, resp.Body)
		return Requester{}, false, fmt.Errorf("os-service respondeu %d", resp.StatusCode)
	}

	var found Requester
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&found); err != nil {
		return Requester{}, false, fmt.Errorf("resposta do os-service invalida: %w", err)
	}
	if found.ID == "" {
		return Requester{}, false, errors.New("resposta do os-service sem id")
	}
	if found.Status != statusActive {
		return found, false, ErrInactive
	}
	return found, false, nil
}
