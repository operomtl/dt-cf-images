# Cloudflare Images Digital Twin

Digital twin of the Cloudflare Images API for local/integration/e2e testing. Written in Go.

## Commands

```bash
go test -race ./internal/...           # unit tests
go test -race -tags=e2e ./test/e2e/... # e2e tests
golangci-lint run ./...                # lint (must pass clean)
```

## Code Standards

- Follow [Effective Go](https://go.dev/doc/effective_go).
- **Always check errors.** Never discard with `_ =`. Log errors when recovery isn't possible (e.g. HTTP response writes after headers are sent). In tests, use `require.NoError` or `t.Errorf`.
- `golangci-lint run ./...` must produce zero warnings before committing.
- All tests must pass with `-race`.
