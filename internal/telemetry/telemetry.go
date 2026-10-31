// Package telemetry liga os logs à transação do New Relic (logs in context) e guarda os
// atributos que os dashboards e alertas usam.
package telemetry

import (
	"context"
	"log/slog"

	"github.com/newrelic/go-agent/v3/newrelic"
)

// Logger devolve base com trace.id e span.id da transação em ctx, quando houver. Com isso o
// New Relic mostra o log junto do trace distribuído.
func Logger(ctx context.Context, base *slog.Logger) *slog.Logger {
	txn := newrelic.FromContext(ctx)
	if txn == nil {
		return base
	}
	md := txn.GetLinkingMetadata()
	if md.TraceID == "" {
		return base
	}
	return base.With("trace.id", md.TraceID, "span.id", md.SpanID, "entity.guid", md.EntityGUID)
}

// Attr grava um atributo na transação atual (sem efeito sem New Relic).
func Attr(ctx context.Context, key string, value any) {
	if txn := newrelic.FromContext(ctx); txn != nil {
		txn.AddAttribute(key, value)
	}
}

// NoticeError marca a transação como erro para o Errors Inbox e os alertas.
func NoticeError(ctx context.Context, err error) {
	if txn := newrelic.FromContext(ctx); txn != nil && err != nil {
		txn.NoticeError(err)
	}
}
