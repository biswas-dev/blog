<div align="center">

# Blog Platform

[![Build Status](https://app.travis-ci.com/anchoo2kewl/blog.svg?branch=main)](https://app.travis-ci.com/anchoo2kewl/blog)
[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-336791?logo=postgresql&logoColor=white)](https://www.postgresql.org/)
[![Docker](https://img.shields.io/badge/Docker-ready-2496ED?logo=docker&logoColor=white)](https://www.docker.com/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

Modern, fast, and secure blog platform written in Go with role-based access control, analytics, slide presentations, and a clean admin panel.

</div>

---

## Features

- **Go backend** with [chi](https://github.com/go-chi/chi) router and clean MVC architecture
- **Embedded HTML templates** using `html/template` + `embed.FS` (no runtime I/O)
- **Role-based access control** — 4 roles (Commenter, Administrator, Editor, Viewer) with granular permissions
- **Admin panel** — Posts, slides, categories, analytics, user management, system settings
- **Editor access** — Editors see a scoped admin panel (posts, slides, categories, analytics, formatting guide) without system/user management
- **Slide presentations** — Create and present [Reveal.js](https://revealjs.com/) slides with categories
- **Analytics dashboard** — Page views, visitor tracking, engagement metrics, 404 slug monitoring
- **API token management** — Create, revoke, and delete Bearer tokens for API access
- **Blog posts** — Markdown editing via [go-wiki](https://github.com/anchoo2kewl/go-wiki), categories, featured images, reading time
- **User management** — Signup/signin, profile, password/email updates, OAuth (GitHub)
- **Security** — IP ban/allow rules, CSRF protection, bcrypt password hashing
- **Theming** — Light/dark mode with a modern tailwind-based theme
- **Observability** — OpenTelemetry tracing, Datadog APM integration
- **Image hosting** — Cloudinary integration for image uploads
- **Email** — Brevo (Sendinblue) integration for transactional emails
- **Database** — PostgreSQL with 22 migrations via [golang-migrate](https://github.com/golang-migrate/migrate)
- **Search** — Full-text post search
- **External sync** — Pull/push posts between blog instances

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Language | Go 1.26 |
| HTTP Router | chi v5 |
| Templates | html/template (embed.FS) |
| Database | PostgreSQL 16 |
| Migrations | golang-migrate |
| Auth | bcrypt, session cookies, API tokens, GitHub OAuth |
| CSS | Tailwind-based custom themes |
| Presentations | Reveal.js |
| Markdown | blackfriday v2, go-wiki |
| Tracing | OpenTelemetry, Datadog |
| E2E Tests | Playwright |
| CI/CD | Travis CI |
| Containerization | Docker, Docker Compose |
| Code Quality | SonarQube |

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

For deployment details, see `docs/deployments.md`.

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

# Development (live reload with Air)
./scripts/server dev --force

# Rebuild binary + restart (refresh embedded templates)
./scripts/server restart-blog

# Tests
./scripts/server test-go
./scripts/server test-playwright
./scripts/server test-all
```

## Role-Based Access Control

| Feature | Commenter | Viewer | Editor | Administrator |
|---------|:---------:|:------:|:------:|:-------------:|
| Comment on posts | Yes | Yes | Yes | Yes |
| View unpublished posts | - | Yes | Yes | Yes |
| Edit/create posts | - | - | Yes | Yes |
| Edit/create slides | - | - | Yes | Yes |
| Admin panel (posts, slides, categories, analytics) | - | - | Yes | Yes |
| System settings | - | - | - | Yes |
| User management | - | - | - | Yes |
| Security (IP rules) | - | - | - | Yes |

## API Overview

Authenticated via API tokens (Bearer) with role-based permissions:

| Endpoint | Method | Access |
|----------|--------|--------|
| `/api/posts` | GET | Public |
| `/api/posts/{id}` | GET | Public |
| `/api/posts` | POST | Editor, Admin |
| `/api/categories` | GET | Public |
| `/api/categories` | POST/PUT/DELETE | Editor, Admin |
| `/api/users` | GET | Admin |
| `/api/users` | POST | Admin |

Create and manage API tokens under "API Access" in your profile.

## CI/CD Pipeline

Powered by **Travis CI** with the following stages:

```
Push to main ──> Tests ──> SonarQube ──> Build Docker Images ──> Deploy Staging
Push to uat  ──> Tests ──────────────────────────────────────> Deploy UAT
Push to production ──> Tests ────────────────────────────────> Deploy Production
```

- **Tests** — `go vet`, `go test` with coverage, PostgreSQL service
- **SonarQube** — Static analysis and coverage reporting (main branch)
- **Docker** — Multi-arch images built and pushed to Docker Hub
- **Deployments** — Automated SSH-based deploys per environment

## Project Structure

```
.
├── controllers/       # HTTP handlers (MVC controllers)
├── middleware/         # Auth, IP filtering, CSRF, analytics middleware
├── models/            # Data models, services, and database logic
├── views/             # Template rendering
├── templates/         # Embedded HTML templates (tailwind theme)
├── themes/            # Alternative themes (modern)
├── migrations/        # PostgreSQL schema migrations (22 migrations)
├── tests/             # Playwright E2E tests
├── deployment/        # Travis CI deploy scripts, Docker configs
├── scripts/           # Dev/ops helper scripts
├── static/            # CSS, JS, images
└── main.go            # Application entry point
```

## Testing

- **304 unit/integration tests** across 48 test files
- **12 Playwright E2E tests** for browser-level validation
- **~35% code coverage** (progressive improvement target)

```bash
# Unit tests
go test ./...

# With coverage
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out

# E2E tests
npx playwright test
```

## Configuration

Environment variables (via `.env`):

```
# Database
PG_USER, PG_PASSWORD, PG_DB, PG_HOST, PG_PORT

# Application
API_TOKEN                # required for API endpoints
APP_DISABLE_SIGNUP=true  # disable public signups
SESSION_SECRET           # session encryption key

# OAuth (optional)
GH_CLIENT_ID, GH_CLIENT_SECRET, OAUTH_STATE_SECRET

# Observability (optional)
DD_API_KEY, DD_SITE      # Datadog APM
OTEL_EXPORTER_OTLP_ENDPOINT  # OpenTelemetry
```

## Deployment

Three environments with automated deployment via Travis CI:

| Environment | Branch | Trigger |
|-------------|--------|---------|
| Staging | `main` | Push to main |
| UAT | `uat` | Push to uat |
| Production | `production` | Push to production |

Docker Compose handles the application and PostgreSQL containers. Pre-built images are pushed to Docker Hub for fast deployments.

## Contributing

Issues and PRs are welcome. Please include clear steps to reproduce and target minimal, focused changes where possible.

## License

MIT
