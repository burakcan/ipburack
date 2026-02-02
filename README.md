# ipburack

A high-performance IP geolocation HTTP API written in Go. Converts IP addresses to country codes with <5ms p99 latency.

## Features

- Fast MMDB-based lookups (microsecond latency)
- Automatic database download on first run
- Hot reload - updates without restart
- Background database updates (every 24h by default)
- Graceful shutdown
- Docker-ready with health checks

## Quick Start

### Using Docker Compose (Recommended)

```bash
docker compose up -d
```

The service will:
1. Build the container
2. Download the MMDB database on first start
3. Be available at http://localhost:3002

### Using Docker

```bash
docker build -t ipburack .
docker run -d -p 3002:3002 -v geo-data:/data ipburack
```

### Running Locally

```bash
# Build
go build -o server ./cmd/server

# Run (database will be downloaded to ./data/geo.mmdb)
MMDB_PATH=./data/geo.mmdb ./server
```

## API Endpoints

### Lookup IP Address

```
GET /lookup/{ip}
```

**Example:**
```bash
curl -H "X-API-Key: your-key" http://localhost:3002/lookup/8.8.8.8
```

**Response:**
```json
{
  "country_code": "US"
}
```

**Error Responses:**
- `401 Unauthorized` - Invalid or missing API key
- `400 Bad Request` - Invalid IP address format
- `404 Not Found` - IP not found in database

### Lookup Caller's IP

```
GET /lookup
```

Automatically detects the caller's IP from:
1. `X-Forwarded-For` header (first IP)
2. `X-Real-IP` header
3. Connection remote address

**Example:**
```bash
curl http://localhost:3002/lookup
```

### Health Check

```
GET /health
```

**Example:**
```bash
curl http://localhost:3002/health
```

**Response:**
```json
{
  "status": "healthy",
  "uptime": "2h30m15s"
}
```

## Authentication

Set `API_KEY` environment variable to enable authentication:

```bash
API_KEY=your-secret-key docker compose up -d
```

Requests must include the `X-API-Key` header:

```bash
curl -H "X-API-Key: your-secret-key" http://localhost:3002/lookup/8.8.8.8
```

The `/health` endpoint is always public (no auth required).

If `API_KEY` is not set, authentication is disabled.

## Configuration

All configuration is via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `HOST` | `0.0.0.0` | Host to bind to |
| `PORT` | `3002` | Port to listen on |
| `MMDB_PATH` | `/data/geo.mmdb` | Path to MMDB database file |
| `MMDB_URL` | jsdelivr URL | URL to download database from |
| `UPDATE_INTERVAL_HOURS` | `24` | Hours between database updates |
| `API_KEY` | _(empty)_ | API key for authentication (empty = disabled) |

## Performance

- Target: <5ms p99 latency, >10k requests/second
- MMDB format provides memory-mapped lookups
- RWMutex allows concurrent reads
- Hot reload swaps database pointer without blocking

## Database

Uses the GeoLite2 Country database from [ip-location-db](https://github.com/sapics/ip-location-db).

The database is:
- Downloaded automatically on first run
- Updated every 24 hours (configurable)
- Validated before swapping to prevent corrupted data

## Attribution

This product includes GeoLite2 data created by MaxMind, available from [https://www.maxmind.com](https://www.maxmind.com).

## License

MIT
