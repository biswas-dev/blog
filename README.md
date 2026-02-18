<div align="center">

# Blog Platform (Go 1.25)

Modern, fast, and secure blog platform written in Go. Includes API token management, user roles, embedded templates, and a minimal, clean UI.

</div>

## Features

- Go 1.25 backend with chi router and clean controllers
- Embedded HTML templates using the standard library (no runtime I/O)
- Role-aware navigation and pages (Commenter, Viewer, Editor, Admin)
- User management (signup/signin, profile, password/email updates)
- API token management (create, revoke, delete)
- Blog posts (top posts, view single post, user’s posts)
- Tailored light/dark styling with a modern theme
- Scripts to build, run, seed the DB, and test (unit + Playwright)

## Quickstart

```bash
# 1) Copy environment
cp .env.sample .env

# 2) Start DB + App
./scripts/server start --force

# 3) Seed roles, users, and sample posts (optional)
./scripts/server db-seed

# Visit the app
open http://localhost:22222
```

For more details, see docs/deployments.md.

## Scripts

Helpful commands from `./scripts/server`:

```bash
# App lifecycle
./scripts/server start|stop|restart|status
./scripts/server start-blog|restart-blog|start-db

# Database
./scripts/server db "SELECT * FROM users LIMIT 5;"
./scripts/server db-migrate|db-seed|db-users|db-posts

# Logs
./scripts/server logs blog|db

# Tests
./scripts/server test-go
./scripts/server test-playwright
./scripts/server test-all
```

## Tech Stack

- Language: Go 1.25+
- HTTP: chi
- Templates: html/template (embed.FS)
- DB: Postgres, migrations via golang-migrate
- Crypto: bcrypt for password/session hashing
- E2E: Playwright

## API Overview

Authenticated via API tokens (Bearer) with role-based permissions:

- GET /api/posts – List posts
- GET /api/posts/{id} – Get a post
- POST /api/posts – Create a post (Editor/Admin)
- GET /api/users – List users (Admin)
- POST /api/users – Create user (Admin)

Create and manage API tokens under “API Access”.

## Development

```bash
# Live reloading (Air)
./scripts/server dev --force

# Rebuild binary + restart (refresh embedded templates)
./scripts/server restart-blog
```

Unit tests and E2E:

```bash
./scripts/server test-go
./scripts/server test-playwright
./scripts/server test-all
```

## Configuration

Environment variables (via `.env`):

```
PG_USER, PG_PASSWORD, PG_DB, PG_HOST, PG_PORT
API_TOKEN                # required for API endpoints
APP_DISABLE_SIGNUP=true  # disable public signups
```

## Deployment

Automated deployment via GitHub Actions and webhook integration. Pushing to main triggers:
- CI tests (unit + E2E)
- SonarQube code quality analysis
- Automatic deployment to production server

## Contributing

Issues and PRs are welcome. Please include clear steps to reproduce and target minimal, focused changes where possible.

## License

MIT
