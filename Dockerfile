# Stage 1: Build Go backend
FROM golang:1.26-alpine AS go-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /app/mees-server ./cmd/server

# Stage 2: Build Next.js frontend (static export)
FROM node:22-alpine AS frontend-builder
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

# Binary and pre-built frontend from earlier stages
COPY --from=go-builder /app/mees-server .
COPY --from=frontend-builder /app/dist ./dist

# Runtime data lives on volumes:
#   content/  — markdown content files
#   uploads/  — user-uploaded images
#   mees.db   — SQLite database
#   .env      — environment config

EXPOSE 8081
CMD ["./mees-server"]
