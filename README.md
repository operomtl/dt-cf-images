# dt-cloudflare-images

[![CI](https://github.com/operomtl/dt-cf-images/actions/workflows/ci.yml/badge.svg)](https://github.com/operomtl/dt-cf-images/actions/workflows/ci.yml)

A digital twin of the Cloudflare Images API for local development, integration testing, and end-to-end testing.

## Quick Start

```bash
docker pull ghcr.io/operomtl/dt-cf-images:latest
docker run -p 8080:8080 -e DT_AUTH_TOKEN=my-token ghcr.io/operomtl/dt-cf-images:latest
```

The API is now available at `http://localhost:8080`.

## Configuration

All configuration is via environment variables:

| Variable | Description | Default |
|---|---|---|
| `DT_LISTEN_ADDR` | Address the server listens on | `:8080` |
| `DT_DB_PATH` | Path to the SQLite database file | `/data/db/images.db` |
| `DT_STORAGE_PATH` | Root directory for image file storage | `/data/images` |
| `DT_AUTH_TOKEN` | API authentication token (empty = accept any token) | `""` |
| `DT_BASE_URL` | Base URL for generated URLs (e.g. direct upload URLs) | `http://localhost:8080` |
| `DT_IMAGE_ALLOWANCE` | Maximum number of images allowed per account | `100000` |
| `DT_ENFORCE_SIGNED_URLS` | Enable signed URL enforcement for image delivery | `""` (off) |

## Docker Compose

```yaml
services:
  dt-cloudflare-images:
    image: ghcr.io/operomtl/dt-cf-images:latest
    ports:
      - "8080:8080"
    environment:
      - DT_LISTEN_ADDR=:8080
      - DT_DB_PATH=/data/db/images.db
      - DT_STORAGE_PATH=/data/images
      - DT_AUTH_TOKEN=test-api-token
      - DT_BASE_URL=http://localhost:8080
    volumes:
      - images-data:/data/images
      - db-data:/data/db
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:8080/health"]
      interval: 10s
      timeout: 3s
      start_period: 5s
      retries: 3

volumes:
  images-data:
  db-data:
```

## API Endpoints

All API routes are under `/accounts/{account_id}/images` and require authentication (except where noted).

### Images (V1)

| Method | Path | Description |
|---|---|---|
| `POST` | `/accounts/{account_id}/images/v1` | Upload an image |
| `GET` | `/accounts/{account_id}/images/v1` | List images (paginated) |
| `GET` | `/accounts/{account_id}/images/v1/{image_id}` | Get image details |
| `PATCH` | `/accounts/{account_id}/images/v1/{image_id}` | Update image metadata |
| `DELETE` | `/accounts/{account_id}/images/v1/{image_id}` | Delete an image |
| `GET` | `/accounts/{account_id}/images/v1/{image_id}/blob` | Download original image bytes |

### Images (V2)

| Method | Path | Description |
|---|---|---|
| `GET` | `/accounts/{account_id}/images/v2` | List images with cursor pagination |

### Direct Upload

| Method | Path | Description |
|---|---|---|
| `POST` | `/accounts/{account_id}/images/v2/direct_upload` | Create a direct upload URL |
| `POST` | `/upload/{upload_id}` | Upload to a direct upload URL (no auth) |

### Variants

| Method | Path | Description |
|---|---|---|
| `POST` | `/accounts/{account_id}/images/v1/variants` | Create a variant |
| `GET` | `/accounts/{account_id}/images/v1/variants` | List all variants |
| `GET` | `/accounts/{account_id}/images/v1/variants/{variant_id}` | Get a variant |
| `PATCH` | `/accounts/{account_id}/images/v1/variants/{variant_id}` | Update a variant |
| `DELETE` | `/accounts/{account_id}/images/v1/variants/{variant_id}` | Delete a variant |

### Signing Keys

| Method | Path | Description |
|---|---|---|
| `GET` | `/accounts/{account_id}/images/v1/keys` | List signing keys |
| `PUT` | `/accounts/{account_id}/images/v1/keys/{signing_key_name}` | Create a signing key |
| `DELETE` | `/accounts/{account_id}/images/v1/keys/{signing_key_name}` | Delete a signing key |

### Stats

| Method | Path | Description |
|---|---|---|
| `GET` | `/accounts/{account_id}/images/v1/stats` | Get image usage stats |

### Image Delivery

| Method | Path | Description |
|---|---|---|
| `GET` | `/cdn/{account_id}/{image_id}/{variant_name}` | Deliver a transformed image (no auth) |

When `DT_ENFORCE_SIGNED_URLS=true`, images with `requireSignedURLs: true` require
`?sig={hmac_hex}&exp={unix_timestamp}` query parameters (unless the variant has
`neverRequireSignedURLs: true`). The signature is HMAC-SHA256 of the URL path +
expiry using a signing key's value.

### Health

| Method | Path | Description |
|---|---|---|
| `GET` | `/health` | Health check (no auth required) |

## Authentication

All `/accounts/...` endpoints require one of:

- **Bearer token**: `Authorization: Bearer <token>`
- **API key + email**: `X-Auth-Key: <token>` and `X-Auth-Email: <email>`

When `DT_AUTH_TOKEN` is set, the provided token must match exactly. When empty, any token is accepted.

## Response Format

All responses use the Cloudflare envelope format:

```json
{
  "result": { ... },
  "success": true,
  "errors": [],
  "messages": []
}
```

## Testing

```bash
# Unit tests
make test

# End-to-end tests
make test-e2e

# Conformance tests
make test-conformance

# All tests
make test-all
```

## Development

```bash
# Build binary
make build

# Lint
make lint

# Docker build and run
make docker-build
make docker-up

# Docker smoke test
make docker-test

# Clean build artifacts
make clean
```
