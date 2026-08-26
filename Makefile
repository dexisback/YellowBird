.PHONY: test test-unit test-integration test-e2e test-all vet tidy

# Fast, hermetic tests (no external services / no Docker).
test-unit:
	go test ./...

# PostgreSQL + Redis integration tests (requires Docker for testcontainers).
test-integration:
	go test -tags integration ./...

# Full end-to-end pipeline (requires Docker + ffmpeg).
test-e2e:
	go test -tags e2e ./tests/e2e/...

# Everything.
test-all: test-unit test-integration test-e2e

vet:
	go vet ./...

tidy:
	go mod tidy

test: test-unit
