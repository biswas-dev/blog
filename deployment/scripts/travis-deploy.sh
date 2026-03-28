#!/bin/bash
# Travis CI deployment script for Blog (anshumanbiswas.com)
# Usage: bash deployment/scripts/travis-deploy.sh <environment>
#
# Staging: deploys the digest from the build stage
# UAT/Production: promotes the previous environment's digest, then deploys

set -euo pipefail

ENV="${1:?Usage: travis-deploy.sh <staging|uat|production>}"

case "$ENV" in
  staging)
    SSH_KEY_VAR="STAGING_SSH_KEY_BASE64"
    SERVER_IP="129.213.82.37"
    SERVER_USER="ubuntu"
    DEPLOY_DIR="~/blog-staging"
    DOMAIN="staging.anshumanbiswas.com"
    NGINX_SCRIPT="scripts/setup-nginx-staging.sh"
    SSL_DOMAIN="staging.anshumanbiswas.com"
    APP_URL="https://staging.anshumanbiswas.com"
    ENCRYPTION_KEY_VAR="STAGING_ENCRYPTION_KEY"
    DD_PROFILING="true"
    PROMOTE_FROM=""
    ;;
  uat)
    SSH_KEY_VAR="UAT_SSH_KEY_BASE64"
    SERVER_IP="92.4.83.28"
    SERVER_USER="ubuntu"
    DEPLOY_DIR="~/blog-uat"
    DOMAIN="uat.anshumanbiswas.com"
    NGINX_SCRIPT="scripts/setup-nginx-uat.sh"
    SSL_DOMAIN="uat.anshumanbiswas.com"
    APP_URL="https://uat.anshumanbiswas.com"
    ENCRYPTION_KEY_VAR="UAT_ENCRYPTION_KEY"
    DD_PROFILING="false"
    PROMOTE_FROM="staging"
    ;;
  production)
    SSH_KEY_VAR="PRODUCTION_SSH_KEY_BASE64"
    SERVER_IP="31.97.102.48"
    SERVER_USER="ubuntu"
    DEPLOY_DIR="~/blog-production"
    DOMAIN="anshumanbiswas.com"
    NGINX_SCRIPT="scripts/setup-nginx-production.sh"
    SSL_DOMAIN="anshumanbiswas.com"
    APP_URL="https://anshumanbiswas.com"
    ENCRYPTION_KEY_VAR="PRODUCTION_ENCRYPTION_KEY"
    DD_PROFILING="true"
    PROMOTE_FROM="staging"
    ;;
  *)
    echo "ERROR: Unknown environment: $ENV"
    exit 1
    ;;
esac

ENCRYPTION_KEY="${!ENCRYPTION_KEY_VAR:-}"

VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "v0.0.0-$(git rev-parse --short HEAD)")
GIT_COMMIT=$(git rev-parse HEAD)
BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
GO_VERSION=$(grep '^go ' go.mod | awk '{print $2}')

# Decode Harbor credentials (base64 avoids Travis $ expansion issues)
HARBOR_USERNAME=$(echo "$HARBOR_AUTH" | base64 -d | cut -d: -f1)
HARBOR_PASSWORD=$(echo "$HARBOR_AUTH" | base64 -d | cut -d: -f2)

# Determine image digest
if [ -n "$PROMOTE_FROM" ]; then
  # UAT/Production: promote from previous environment
  echo "=== Promoting blog image: $PROMOTE_FROM -> $ENV ==="
  IMAGE_DIGEST=$(bash deployment/scripts/harbor-promote.sh biswas blog "$PROMOTE_FROM" "$ENV")
else
  # Staging: use digest from build stage (shared via Travis workspace)
  if [ -f .image-digest.txt ]; then
    IMAGE_DIGEST=$(cat .image-digest.txt)
  else
    # Fallback: look up staging-latest from Harbor API
    IMAGE_DIGEST=$(curl -sf -u "$HARBOR_USERNAME:$HARBOR_PASSWORD" \
      "https://harbor.biswas.me/api/v2.0/projects/biswas/repositories/blog/artifacts?q=tags%3Dstaging-latest" \
      | jq -r '.[0].digest')
  fi
fi

if [ -z "$IMAGE_DIGEST" ] || [ "$IMAGE_DIGEST" = "null" ]; then
  echo "ERROR: Could not determine image digest for $ENV deployment"
  exit 1
fi

echo "=== Blog Travis CI Deploy ==="
echo "Environment: $ENV"
echo "Server:      $SERVER_USER@$SERVER_IP"
echo "Version:     $VERSION"
echo "Digest:      $IMAGE_DIGEST"
echo ""

# SSH setup
SSH_KEY_VALUE="${!SSH_KEY_VAR:-}"
if [ -z "$SSH_KEY_VALUE" ]; then
  echo "ERROR: $SSH_KEY_VAR is not set"
  exit 1
fi

mkdir -p ~/.ssh
# Support both raw PEM keys (GitHub Actions) and base64-encoded keys (Travis CI)
if printf '%s' "$SSH_KEY_VALUE" | head -c 10 | grep -q '^\-\-\-\-\-'; then
  printf '%s\n' "$SSH_KEY_VALUE" > ~/.ssh/deploy_key
else
  echo "$SSH_KEY_VALUE" | base64 --decode > ~/.ssh/deploy_key
fi
chmod 600 ~/.ssh/deploy_key
ssh-keyscan "$SERVER_IP" >> ~/.ssh/known_hosts 2>/dev/null

SSH_CMD="ssh -o StrictHostKeyChecking=no -i ~/.ssh/deploy_key $SERVER_USER@$SERVER_IP"
SCP_CMD="scp -o StrictHostKeyChecking=no -i ~/.ssh/deploy_key"

# Create deploy directory and subdirectories on remote server
$SSH_CMD "mkdir -p $DEPLOY_DIR/scripts $DEPLOY_DIR/migrations"

# SCP compose files, otel config, nginx script, and migrations to the server
$SCP_CMD docker-compose.yml docker-compose.hub.yml otel-config.yaml "$SERVER_USER@$SERVER_IP:$DEPLOY_DIR/"
$SCP_CMD "$NGINX_SCRIPT" "$SERVER_USER@$SERVER_IP:$DEPLOY_DIR/$NGINX_SCRIPT"
$SCP_CMD migrations/*.up.sql "$SERVER_USER@$SERVER_IP:$DEPLOY_DIR/migrations/" 2>/dev/null || true

$SSH_CMD "bash -s" <<REMOTE_EOF
  set -e

  cd $DEPLOY_DIR
  rm -rf .git controllers models views templates middleware 2>/dev/null || true

  # Login to Harbor
  echo '${HARBOR_PASSWORD}' | docker login harbor.biswas.me -u '${HARBOR_USERNAME}' --password-stdin || true

  # Ensure Docker volume exists
  docker volume inspect blog-postgres-data >/dev/null 2>&1 || docker volume create blog-postgres-data

  # Setup nginx
  chmod +x $NGINX_SCRIPT
  ./$NGINX_SCRIPT

  # Create uploads directory
  mkdir -p static/uploads

  # Environment variables
  export PG_PASSWORD='${BLOG_PG_PASSWORD}'
  export SESSION_SECRET='${BLOG_SESSION_SECRET}'
  export API_TOKEN='${BLOG_API_TOKEN}'
  export BLOG_UPLOADS_PATH="$DEPLOY_DIR/static/uploads"
  export ENCRYPTION_KEY='$ENCRYPTION_KEY'
  export GH_CLIENT_ID='${GH_CLIENT_ID}'
  export GH_CLIENT_SECRET='${GH_CLIENT_SECRET}'
  export OAUTH_STATE_SECRET='${OAUTH_STATE_SECRET}'
  export APP_URL='$APP_URL'
  export VERSION='$VERSION'
  export GIT_COMMIT='$GIT_COMMIT'
  export BUILD_TIME='$BUILD_TIME'
  export GO_VERSION='go$GO_VERSION'
  export APP_ENV='$ENV'

  # Version endpoint protection
  export VERSION_TOKEN='${VERSION_TOKEN:-}'

  # Datadog APM
  export DD_API_KEY='${DD_API_KEY:-}'
  export DD_SITE='${DD_SITE:-datadoghq.com}'
  export APM_ENABLED='true'
  export DD_PROFILING_ENABLED='$DD_PROFILING'
  export DD_AGENT_HOST='dd-agent'

  # Image digest for docker-compose overlay
  export IMAGE_DIGEST='$IMAGE_DIGEST'

  # Stop existing containers
  docker compose down --remove-orphans || true
  docker rm -f blog-postgres blog-app blog-dd-agent blog-otel-collector 2>/dev/null || true

  # Disk space management
  DISK_USAGE=\$(df / | tail -1 | awk '{print \$5}' | tr -d '%')
  if [ "\$DISK_USAGE" -gt 80 ]; then
    docker builder prune -f --filter "until=48h" || true
    docker image prune -f || true
  fi

  # Pull pre-built image from Harbor by digest
  docker compose -f docker-compose.yml -f docker-compose.hub.yml pull app

  # Final cleanup before start
  for c in blog-postgres blog-app blog-dd-agent blog-otel-collector; do
    docker stop \$c 2>/dev/null || true
    docker rm -f \$c 2>/dev/null || true
  done
  docker network rm ${ENV//-/_}_blog-network 2>/dev/null || true
  sleep 2

  docker compose -f docker-compose.yml -f docker-compose.hub.yml up -d --no-build
  sudo systemctl reload nginx || true

  # Wait for postgres
  sleep 10

  # Sync postgres password
  docker exec blog-postgres psql -U blog -d blog -c "ALTER USER blog WITH PASSWORD '\$PG_PASSWORD';" || true

  # Run database migrations
  if [ -d "migrations" ] && ls migrations/*.up.sql 1>/dev/null 2>&1; then
    docker exec blog-postgres psql -U blog -d blog -c "
      CREATE TABLE IF NOT EXISTS schema_migrations (
        version bigint NOT NULL PRIMARY KEY,
        dirty boolean NOT NULL DEFAULT false
      );" 2>/dev/null

    CURRENT_VERSION=\$(docker exec blog-postgres psql -U blog -d blog -tAc "SELECT COALESCE(MAX(version), 0) FROM schema_migrations" 2>/dev/null || echo "0")
    CURRENT_VERSION=\$(echo "\$CURRENT_VERSION" | tr -d ' ')
    HAS_USERS=\$(docker exec blog-postgres psql -U blog -d blog -tAc "SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_name='users')" 2>/dev/null || echo "f")

    if [ "\$HAS_USERS" = "t" ] && [ "\$CURRENT_VERSION" -lt 6 ] 2>/dev/null; then
      docker exec blog-postgres psql -U blog -d blog -c "DELETE FROM schema_migrations; INSERT INTO schema_migrations (version, dirty) VALUES (6, false);"
    fi

    CURRENT_VERSION=\$(docker exec blog-postgres psql -U blog -d blog -tAc "SELECT COALESCE(MAX(version), 0) FROM schema_migrations" 2>/dev/null || echo "0")
    CURRENT_VERSION=\$(echo "\$CURRENT_VERSION" | tr -d ' ')

    for f in \$(ls migrations/*.up.sql | sort); do
      FILE_VERSION=\$(basename "\$f" | grep -o '^[0-9]*')
      if [ "\$FILE_VERSION" -gt "\$CURRENT_VERSION" ] 2>/dev/null; then
        docker cp "\$f" blog-postgres:/tmp/migration.sql
        if docker exec blog-postgres psql -U blog -d blog -f /tmp/migration.sql; then
          docker exec blog-postgres psql -U blog -d blog -c "DELETE FROM schema_migrations; INSERT INTO schema_migrations (version, dirty) VALUES (\$FILE_VERSION, false);"
        else
          docker exec blog-postgres psql -U blog -d blog -c "DELETE FROM schema_migrations; INSERT INTO schema_migrations (version, dirty) VALUES (\$FILE_VERSION, true);"
          break
        fi
      fi
    done
  fi

  # Cloudinary cleanup (one-time)
  if [ ! -f static/uploads/.cloudinary_cleanup_done ]; then
    rm -rf static/uploads/featured static/uploads/post 2>/dev/null || true
    touch static/uploads/.cloudinary_cleanup_done
  fi

  docker compose ps
REMOTE_EOF

# Verify deployment
echo ""
echo "=== Verifying deployment ==="
sleep 10

echo "Health check: http://$SERVER_IP (Host: $DOMAIN)"
if curl -f -H "Host: $DOMAIN" "http://$SERVER_IP" 2>/dev/null; then
  echo ""
  echo "Health check passed"
else
  echo "WARNING: Health check failed (server may still be starting)"
fi

echo ""
echo "=== Deploy to $ENV complete ==="

rm -f ~/.ssh/deploy_key
