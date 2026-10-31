FUNCTIONS := issuer authorizer
COVER_PKGS := ./internal/...

# Integração: precisa do os-service do docker compose (postech-tc4-os-service) rodando.
OS_SERVICE_URL ?= http://localhost:8080
INTERNAL_API_TOKEN ?= local-internal-token
JWT_SECRET ?= change-me-in-production
TEST_CPF ?= 52998224725

.PHONY: tidy test cover integration lint build package clean

tidy:
	go mod tidy

test:
	go test ./... -race -cover

cover:
	go test $(COVER_PKGS) -race -coverprofile=coverage.out
	go tool cover -func=coverage.out | tail -1

integration:
	OS_SERVICE_URL=$(OS_SERVICE_URL) INTERNAL_API_TOKEN=$(INTERNAL_API_TOKEN) \
	JWT_SECRET=$(JWT_SECRET) TEST_CPF=$(TEST_CPF) \
	go test -tags integration -count=1 -v ./test/integration/...

lint:
	golangci-lint run ./...

build:
	@for fn in $(FUNCTIONS); do \
		GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -tags lambda.norpc -o bin/$$fn/bootstrap ./cmd/$$fn; \
	done

package: build
	@for fn in $(FUNCTIONS); do \
		(cd bin/$$fn && zip -q ../../$$fn.zip bootstrap); \
	done

clean:
	rm -rf bin issuer.zip authorizer.zip coverage.out
