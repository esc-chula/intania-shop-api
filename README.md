# Intania Shop API

Backend API for the ESC Chula Intania Shop. It provides authentication and authorization, product and variant management, carts, favorites, orders, promotions, banners, inventory tracking, and media uploads.

## Technology

- Go 1.26.5
- Chi v5 routing with standard `net/http` handlers and middleware
- PostgreSQL with pgx 5
- Goose database migrations
- Cobra CLI commands
- JWT authentication using HS256
- Google OAuth 2.0 with PKCE
- Google Cloud Storage for uploaded media
- Structured JSON logging with `log/slog`

## Requirements

- Go 1.26.5, as pinned by `go.mod`
- PostgreSQL
- Docker and Docker Compose if you use the included local database
- Google OAuth client credentials
- A Google Cloud Storage bucket
- Google Application Default Credentials; local development can use the service-account JSON key
- Air for optional hot reload
- `golangci-lint` for the complete check workflow

Install Air:

```bash
go install github.com/air-verse/air@latest
export PATH="$(go env GOPATH)/bin:$PATH"
```

## Configuration

Copy the example configuration:

```bash
cp .env.example .env
```

Configure `.env` for local development:

```dotenv
SERVER_ADDR=0.0.0.0:8080
DATABASE_URL=postgres://postgres:postgres@localhost:5432/intania_shop?sslmode=disable
DB_MAX_CONNS=10
DB_MIN_CONNS=0
DB_MAX_CONN_LIFETIME=30m
DB_MAX_CONN_IDLE_TIME=5m
SERVER_READ_HEADER_TIMEOUT=5s
SERVER_READ_TIMEOUT=15s
SERVER_WRITE_TIMEOUT=30s
SERVER_IDLE_TIMEOUT=60s
SERVER_SHUTDOWN_TIMEOUT=15s
CORS_ALLOWED_ORIGINS=http://localhost:3000,http://localhost:8080
LOG_LEVEL=INFO
JWT_SECRET=replace-with-a-random-secret-at-least-32-bytes
JWT_ISSUER=intania-shop-api
JWT_TTL=24h
GOOGLE_CLIENT_ID=your-google-client-id
GOOGLE_CLIENT_SECRET=your-google-client-secret
GOOGLE_REDIRECT_URL=http://localhost:8080/auth/google/callback
AUTH_COOKIE_SECURE=false
GCS_BUCKET=your-bucket-name
GOOGLE_APPLICATION_CREDENTIALS=/absolute/path/to/service-account.json
```

| Variable | Required | Purpose |
| --- | --- | --- |
| `DATABASE_URL` | Yes | PostgreSQL connection string |
| `JWT_SECRET` | Yes | HS256 signing and OAuth-cookie secret; minimum 32 bytes |
| `GOOGLE_CLIENT_ID` | Yes | Google OAuth client ID |
| `GOOGLE_CLIENT_SECRET` | Yes | Google OAuth client secret |
| `GOOGLE_REDIRECT_URL` | Yes | Registered Google OAuth callback URL |
| `GCS_BUCKET` | Yes | Google Cloud Storage bucket used for uploads |
| `GOOGLE_APPLICATION_CREDENTIALS` | Environment-dependent | Local service-account JSON path; Google Cloud runtimes should use an attached service account |
| `SERVER_ADDR` | No | Listen address; defaults to `0.0.0.0:8080` |
| `DB_MAX_CONNS` / `DB_MIN_CONNS` | No | PostgreSQL pool limits; defaults to `10` / `0` |
| `DB_MAX_CONN_LIFETIME` | No | Maximum connection lifetime; defaults to `30m` |
| `DB_MAX_CONN_IDLE_TIME` | No | Maximum idle time; defaults to `5m` |
| `JWT_ISSUER` | No | Required token issuer; defaults to `intania-shop-api` |
| `JWT_TTL` | No | Access-token lifetime; defaults to `24h` |
| `AUTH_COOKIE_SECURE` | No | Require HTTPS for the OAuth-state cookie; use `false` only for local HTTP |
| `CORS_ALLOWED_ORIGINS` | No | Comma-separated browser origins; local defaults are provided |
| `LOG_LEVEL` | No | Structured log level; defaults to `INFO` |
| `SERVER_*_TIMEOUT` | No | HTTP lifecycle timeouts; see `.env.example` |

The server fails fast when required database, authentication, OAuth, or storage configuration is missing. Keep `.env` and service-account JSON files out of version control.

`make run` and `make migrate` load `.env`. Air loads it through `.air.toml`, so `make dev` and `air` work directly. Raw Go commands, tests, containers, and deployments require ordinary exported environment variables.

## Local development

Start PostgreSQL and initialize an empty database:

```bash
docker compose up -d db
make migrate
```

Run once or with hot reload:

```bash
make run
make dev
```

The default base URL is `http://localhost:8080`. `0.0.0.0` is the bind address, not the browser URL.

```bash
curl http://localhost:8080/
curl http://localhost:8080/health
```

## Authentication and authorization

Google OAuth with PKCE is the supported login flow. For browser login, open:

```text
http://localhost:8080/auth/google/redirect
```

The API sets a signed, short-lived, HttpOnly OAuth-state cookie, redirects to Google, verifies the returned state and PKCE verifier, then returns the user and a JWT from `GET /auth/google/callback`.

Browser clients may instead request `GET /auth/google`. It returns `auth_url` and sets the same cookie, so the request must include credentials before navigation:

```javascript
const response = await fetch("http://localhost:8080/auth/google", {
  credentials: "include",
});
const { data } = await response.json();
window.location.assign(data.auth_url);
```

Keep the host consistent. With the default callback, use `localhost`, not `0.0.0.0` or `127.0.0.1`. `GOOGLE_REDIRECT_URL` must exactly match Google Cloud Console.

### Testing authentication with Bruno, Postman, or curl

API clients do not share their cookie jar with your browser:

1. Open `http://localhost:8080/auth/google/redirect` in a browser.
2. Complete Google sign-in.
3. Copy the JWT from the callback JSON.
4. Configure it as a Bearer token in the API client.

```http
Authorization: Bearer <token>
```

JWTs contain user ID and role and expire according to `JWT_TTL`. User-specific operations derive ownership from the JWT.

Public access covers service checks, OAuth, catalog reads, and promotion/banner reads. Cart, favorite, and user-order operations require authentication. Catalog mutations, inventory, uploads, content mutations, and administrative order operations require an `Admin` token.

## API endpoints

### General and authentication

| Method | Path | Access | Description |
| --- | --- | --- | --- |
| `GET` | `/` | Public | Service name |
| `GET` | `/health` | Public | Database-backed health check |
| `GET` | `/openapi.yaml` | Public | OpenAPI contract |
| `GET` | `/docs` | Public | Interactive API reference |
| `GET` | `/auth/google` | Public | Create an authorization URL and set state cookie |
| `GET` | `/auth/google/redirect` | Public | Set state cookie and redirect to Google |
| `GET` | `/auth/google/callback` | Public | Complete login and return a JWT |

### Products and variants

| Method | Path | Access | Description |
| --- | --- | --- | --- |
| `GET` | `/products` | Public | List; supports `page`, `page_size`, `include_variants=true` |
| `GET` | `/products/search` | Public | Search by `q`; supports pagination |
| `GET` | `/products/{id}` | Public | Get product details |
| `POST` | `/products` | Admin | Create product |
| `PUT` | `/products/{id}` | Admin | Update product |
| `DELETE` | `/products/{id}` | Admin | Delete product |
| `POST` | `/products/{id}/variants` | Admin | Create variant |
| `PUT` | `/variants/{id}` | Admin | Update variant |
| `DELETE` | `/variants/{id}` | Admin | Delete variant |

Product pagination defaults to page 1 and 10 items, with a maximum page size of 100.

### Cart, favorites, and orders

| Method | Path | Access | Description |
| --- | --- | --- | --- |
| `GET` | `/cart` | User | Get caller's cart |
| `PUT` | `/cart/items` | User | Add or increment a cart variant |
| `PUT` | `/favorites` | User | Add a favorite product |
| `POST` | `/orders` | User | Create caller's order |
| `GET` | `/orders` | User/Admin | List caller's orders; admins receive all |
| `GET` | `/orders/{id}` | Owner/Admin | Get an owned order or any as admin |
| `PUT` | `/orders/{id}` | Admin | Update order |
| `DELETE` | `/orders/{id}` | Admin | Delete order |

### Inventory

| Method | Path | Access | Description |
| --- | --- | --- | --- |
| `POST` | `/products/{id}/stock/adjust` | Admin | Increment or decrement stock |
| `GET` | `/products/{id}/stock/transactions` | Admin | Product stock history |
| `GET` | `/variants/{id}/stock/transactions` | Admin | Variant stock history |
| `GET` | `/stock/transactions` | Admin | Grouped stock history |
| `POST` | `/stock/bulk-reduction` | Admin | Atomically reduce several items |
| `POST` | `/stock/upload-proof-images` | Admin | Upload one proof image |

History defaults to 20 items per page and is capped at 100. Inventory updates lock rows and record history in the same transaction.

### Promotions and banners

| Method | Path | Access | Description |
| --- | --- | --- | --- |
| `GET` | `/promos` | Public | List promotions |
| `GET` | `/promos/active` | Public | List active promotions |
| `GET` | `/promos/{id}` | Public | Get promotion |
| `POST` | `/promos` | Admin | Create promotion |
| `PUT` | `/promos/{id}` | Admin | Update promotion |
| `DELETE` | `/promos/{id}` | Admin | Delete promotion |
| `GET` | `/banners` | Public | List banners |
| `GET` | `/banners/active` | Public | List active banners |
| `GET` | `/banners/{id}` | Public | Get banner |
| `POST` | `/banners` | Admin | Create banner |
| `PUT` | `/banners/{id}` | Admin | Update banner |
| `DELETE` | `/banners/{id}` | Admin | Delete banner |

### Uploads

| Method | Path | Access | Description |
| --- | --- | --- | --- |
| `POST` | `/upload/product-images` | Admin | Upload product images |
| `POST` | `/upload/product-videos` | Admin | Upload product videos |
| `POST` | `/stock/upload-proof-images` | Admin | Upload one proof image |

Uploads use `multipart/form-data`. Product uploads use the `files` field and allow 100 MiB bodies. Proof uploads accept the first file and allow 10 MiB.

See [`docs/openapi.yaml`](docs/openapi.yaml) for the machine-readable contract.

## Responses and errors

Successful JSON responses:

```json
{
  "success": true,
  "data": {}
}
```

Application errors:

```json
{
  "success": false,
  "error": "Product not found"
}
```

Common statuses:

- `400` malformed or invalid input
- `401` missing, invalid, or expired authentication
- `403` insufficient role or ownership
- `404` missing resource
- `409` conflicting data where applicable
- `413` oversized upload
- `500` unexpected application failure
- `502` upstream Google OAuth failure
- `503` failed PostgreSQL health check

Root and health return plain text. Successful deletes return `204 No Content`.

## Database migrations

Migrations live in `internal/migrations/sql/`, are embedded, and use Goose. They do not run at API startup.

For an empty database:

```bash
make migrate
```

For an existing legacy database, do not replay the baseline. Back up the database, verify that its schema matches `internal/migrations/sql/00001_legacy_baseline.sql`, then run:

```bash
go run . adopt-baseline --confirm-legacy-schema
```

After adoption, apply later migrations with `make migrate`. Production migrations should run as one deployment job.

## Quality checks

```bash
make check
```

Individual commands:

```bash
make fmt
make vet
make test
make lint
make build
```

`make test` includes the race detector and coverage. Integration tests require an isolated database:

```bash
TEST_DATABASE_URL="postgres://postgres:postgres@localhost:55433/intania_shop_test?sslmode=disable" make test-integration
```

Never use a database containing data you need to preserve.

## Docker

Build:

```bash
docker build -t intania-shop-api .
```

Run locally with the credential mounted:

```bash
docker run --rm -p 8080:8080 \
  --env-file .env \
  -v "$PWD/service-account.json:/run/secrets/gcp.json:ro" \
  -e GOOGLE_APPLICATION_CREDENTIALS=/run/secrets/gcp.json \
  intania-shop-api
```

The final image is distroless and non-root. On Cloud Run, attach a least-privilege runtime service account with access to `GCS_BUCKET` instead of deploying a JSON key.

## Project structure

```text
cmd/                    CLI commands: serve, migrate, adopt-baseline
internal/
├── auth/               Google OAuth and PKCE
├── config/             Environment loading and validation
├── database/           PostgreSQL pool
├── handlers/           HTTP transport
├── middlewares/        Auth, CORS, logging, recovery, request IDs
├── migrations/         Goose and embedded SQL
├── models/             Domain and API models
├── repositories/       PostgreSQL access and transactions
├── server/             Chi route groups, dependency wiring, and lifecycle
├── storage/            Google Cloud Storage
└── usecases/           Business rules and ownership
docs/                   OpenAPI contract and embedded documentation assets
main.go                 CLI entry point
Dockerfile              Distroless image
docker-compose.yaml     Local PostgreSQL
Makefile                Development commands
.air.toml               Hot reload
```

Dependency direction:

```text
handlers -> usecases -> repository interfaces
                         ^
                  PostgreSQL repositories

server -> constructs and connects dependencies
```

