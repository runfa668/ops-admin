# Production deployment

The production topology is Vercel React -> same-origin `/api/*` rewrite -> Render Go -> Neon PostgreSQL.

## Render

- Service: `ops-admin-go`
- Runtime: Go
- Plan: Free
- Region: Virginia
- Build: `go build -o ./bin/ops-admin ./cmd/server`
- Start: `./bin/ops-admin`
- Health: `/api/health`
- Environment: `DATABASE_URL`, `OPS_DEMO=true`, `OPS_COOKIE_SECURE=true`, `PUBLIC_ORIGIN=https://ops-admin-eta.vercel.app`

The server reads Render's `PORT` automatically. `PUBLIC_ORIGIN` explicitly permits production browser writes proxied through the Vercel origin while retaining the application's origin check.

## Vercel

- Project: `ops-admin`
- Production alias: `https://ops-admin-eta.vercel.app`
- `vercel.json` rewrites `/api/:path*` to the Render service.

Keeping browser API calls under the Vercel origin preserves the application's same-origin Cookie and CSRF model while the API executes on Render.

## Neon

The Go service connects to the existing Neon PostgreSQL project using the pooled `DATABASE_URL`. Do not commit the connection string or database credentials.
