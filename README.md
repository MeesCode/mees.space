# mees.space

Personal website and blog built with Go + Next.js static export.

## Prerequisites

- [Go](https://go.dev/) 1.26+
- [Node.js](https://nodejs.org/) 20+

## Setup

Install frontend dependencies:

```bash
cd frontend && npm install && cd ..
```

Copy the example environment file and edit the values:

```bash
cp .env.example .env
```

## Build & Run

```bash
make build
make run
```

Or without make:

```bash
cd frontend && npm run build && cd ..
go build -o mees-server ./cmd/server
./mees-server
```

The server starts at `http://localhost:8080`.

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `MEES_JWT_SECRET` | *required* | Secret key for JWT signing |
| `MEES_ADMIN_PASSWORD` | *required* | Admin login password |
| `MEES_PORT` | `8080` | Server port |
| `MEES_DATABASE_PATH` | `./mees.db` | SQLite database path |
| `MEES_CONTENT_DIR` | `./content` | Markdown content directory |
| `MEES_UPLOADS_DIR` | `./uploads` | Uploaded images directory |
| `MEES_DIST_DIR` | `./dist` | Frontend build output |

## Admin

Login at `/admin/login` with username `admin` and the password from `MEES_ADMIN_PASSWORD`.
