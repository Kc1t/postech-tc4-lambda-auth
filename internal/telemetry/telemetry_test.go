package telemetry

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/newrelic/go-agent/v3/newrelic"
)

func TestSemTransacao(t *testing.T) {
	var buf bytes.Buffer
	base := slog.New(slog.NewJSONHandler(&buf, nil))
	ctx := context.Background()

	Logger(ctx, base).Info("x")
	Attr(ctx, "k", "v")
	NoticeError(ctx, errors.New("boom"))

	if strings.Contains(buf.String(), "trace.id") {
		t.Fatalf("sem transação não há trace: %s", buf.String())
	}
}

func TestComTransacao(t *testing.T) {
	app, err := newrelic.NewApplication(
		newrelic.ConfigAppName("teste"),
		newrelic.ConfigLicense(strings.Repeat("0", 40)),
		newrelic.ConfigDistributedTracerEnabled(true),
		newrelic.ConfigEnabled(false),
	)
	if err != nil {
		t.Fatal(err)
	}
	txn := app.StartTransaction("teste")
	defer txn.End()
	ctx := newrelic.NewContext(context.Background(), txn)

	var buf bytes.Buffer
	Logger(ctx, slog.New(slog.NewJSONHandler(&buf, nil))).Info("x")
	Attr(ctx, "auth.outcome", "issued")
	NoticeError(ctx, errors.New("boom"))

	if !strings.Contains(buf.String(), "trace.id") {
		t.Fatalf("esperava trace.id no log: %s", buf.String())
	}
}
