# Spec: Backend Scaffold

Covers R-001 (backend directory) and R-002 (Go module + health handler).

## Requirements

### BS-01: Go Module Initialization

- `backend/go.mod` exists with module path matching the repo
- Go version: 1.26
- Dependencies: `github.com/go-chi/chi/v5` v5.2.5, `github.com/awslabs/aws-lambda-go-api-proxy`, `github.com/aws/aws-lambda-go` v1.54.0

### BS-02: Directory Layout

- `backend/cmd/api/main.go` exists
- `backend/internal/handler/health.go` exists
- `backend/internal/handler/health_test.go` exists
- `backend/.golangci.yml` exists with reasonable defaults

### BS-03: Lambda Entry Point

- `main.go` detects the Lambda environment (e.g., `AWS_LAMBDA_FUNCTION_NAME` env var)
- In Lambda: starts `lambda.Start` with chi adapter from aws-lambda-go-api-proxy
- Locally: starts `net/http` server on configurable port (default 8080, overridable via `PORT` env var)
- Both paths use the same chi router instance

### BS-04: Health Check Handler

- `GET /health` returns HTTP 200
- Response body: `{"status": "ok"}`
- Content-Type header: `application/json`
- Table-driven test in `health_test.go` verifying status code, body, and content type

### BS-05: Build Verification

- `go build ./...` succeeds with zero errors
- `go test ./...` passes with all tests green
- `go vet ./...` reports no issues
- `golangci-lint run` passes clean

## Acceptance Criteria

All BS-01 through BS-05 requirements pass. The health check handler is tested and the binary compiles for both local and Lambda execution.
