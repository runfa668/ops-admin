# Production deployment

The backend uses PostgreSQL via `DATABASE_URL` and is intended for Neon + Render.

## Render
- Runtime: Go
- Build: `go build -o bin/server ./cmd/server`
- Start: `./bin/server -demo=true`
- Health: `/api/health`
- Environment: `DATABASE_URL=<Neon pooled PostgreSQL URL>`
- Environment after Vercel is created: `PUBLIC_ORIGIN=https://<vercel-domain>`

The server reads Render's `PORT` automatically.

## Frontend
The React/Vite frontend is hosted on Vercel. Browser API requests stay under `/api/*`; Vercel rewrites them to the Render service so session cookies, CSRF, and same-origin write protection continue to work.
