# Stage 1: Build Go backend
FROM golang:1.26-alpine AS go-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /app/mees-server ./cmd/server

# Stage 2: Build Next.js frontend (static export)
# Debian-based (glibc): Tailwind 4's @tailwindcss/oxide ships a native binding
# that npm picks per-platform. The build only emits a static export, so the
# builder OS is irrelevant at runtime. (The committed package-lock.json must
# list every @tailwindcss/oxide-* platform variant, or `npm ci` installs no
# binding at all — see npm/cli#4828.)
FROM node:22-slim AS frontend-builder
WORKDIR /app
COPY frontend/package.json frontend/package-lock.json ./frontend/
RUN cd frontend && npm ci
COPY frontend/ ./frontend/
# prebuild script needs Go testdata for slug parity verification
COPY internal/render/testdata/ ./internal/render/testdata/
RUN cd frontend && NODE_ENV=production npm run build

# Stage 3: Runtime
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app

# Binary, SQL migrations, and pre-built frontend from earlier stages.
# Migrations are loaded from ./migrations at runtime (cmd/server/main.go).
COPY --from=go-builder /app/mees-server .
COPY --from=go-builder /app/migrations ./migrations
COPY --from=frontend-builder /app/dist ./dist

# Runtime data lives on volumes:
#   content/  — markdown content files
#   uploads/  — user-uploaded images
#   mees.db   — SQLite database
#   .env      — environment config

EXPOSE 8081
CMD ["./mees-server"]
