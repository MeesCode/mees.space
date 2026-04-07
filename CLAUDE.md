# mees.space

Personal website/blog platform with Go backend + Next.js static export frontend, SQLite database, and AI-assisted content editing via Claude API.

## Quick Commands

```bash
make build           # Build frontend + backend
make build-run       # Build and run
make test            # Go tests with coverage
make clean           # Remove artifacts
cd frontend && npm run dev   # Frontend dev server
air                          # Go hot reload
```

## Architecture

- **Backend**: Go 1.26+, standard library router (Go 1.22+ method routing), SQLite (modernc.org/sqlite), JWT auth
- **Frontend**: Next.js 15 (static export to `dist/`), React 19, TypeScript, Tailwind CSS 4
- **Content**: Markdown files on filesystem (`content/`), metadata in SQLite
- **Auth**: Single admin user, JWT access + refresh tokens, bcrypt passwords

## Project Structure

```
cmd/server/main.go       # Entry point, routing, static file serving
internal/
  ai/                    # Claude API streaming (SSE) with tool use
  auth/                  # JWT auth, login/refresh handlers, middleware
  config/                # Env config loader (MEES_* vars)
  database/              # SQLite setup, migrations
  folders/               # Folder CRUD handlers
  httputil/              # JSON responses, path sanitization
  images/                # Image upload/serve handlers
  middleware/            # Security headers, request logging
  pages/                 # Page CRUD, tree building, RSS feed
  settings/              # Key-value settings store
frontend/src/
  app/[[...slug]]/       # Dynamic catch-all content pages
  app/admin/             # Admin editor, settings, login
  components/            # React components
  lib/                   # API client, auth, types
  hooks/                 # Custom React hooks
migrations/              # Numbered SQL up/down migrations
content/                 # Markdown content files
uploads/                 # User-uploaded images
dist/                    # Frontend build output
```

## Environment Variables

See `.env.example`. Key vars: `MEES_JWT_SECRET`, `MEES_ADMIN_PASSWORD` (required), `MEES_PORT` (default 8080), `MEES_DATABASE_PATH`, `MEES_CONTENT_DIR`, `MEES_UPLOADS_DIR`, `MEES_DIST_DIR`.

## API Routes

- Auth: `POST /api/auth/login`, `POST /api/auth/refresh`
- Pages: `GET|POST|PUT|PATCH|DELETE /api/pages/{path}`, `GET /api/pages/tree`
- Folders: `POST|PUT|DELETE /api/folders/{path}`
- Images: `GET|POST /api/images`, `DELETE /api/images/{filename}`
- Settings: `GET|PUT /api/settings`
- AI: `POST /api/ai/complete` (streaming SSE)
- Public: `GET /feed.xml`, `POST /api/views/{path}`, `GET /health`

## Conventions

- Error handling: custom error types (ErrNotFound, ErrInvalidPath, ErrExists), JSON errors via `httputil.JSONError()`
- Path security: `httputil.SanitizePath()` prevents directory traversal
- Tests: Go `_test.go` files with coverage for config, auth, database, handlers
- Frontend: App Router, client-side API calls, tokens in localStorage
- Database: WAL mode, foreign keys enabled, auto-migration on startup
