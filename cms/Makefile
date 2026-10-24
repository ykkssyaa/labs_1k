STATICCHECK := honnef.co/go/tools/cmd/staticcheck@2025.1.1
# Не скачивать молча новый тулчейн: go.mod обязан собираться Go 1.25 (см. журнал ИИ).
export GOTOOLCHAIN := local

.PHONY: run build test lint check fmt docker

run: ## стенд одной командой: миграции применяются при старте
	VESTNIK_SEED=$${VESTNIK_SEED:-1} go run ./cmd/vestnik serve

build:
	CGO_ENABLED=0 go build -o bin/vestnik ./cmd/vestnik

test:
	go test ./...

fmt:
	gofmt -w .

lint:
	@out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "gofmt:"; echo "$$out"; exit 1; fi
	go vet ./...
	go run $(STATICCHECK) ./...

check: lint test ## линтер и тесты одной командой

docker:
	docker compose up --build
