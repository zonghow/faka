#!/usr/bin/env bash
set -euo pipefail

# Production deploy helper for key.whistlelads.com
# Usage on server:
#   cd /opt/faka && bash deploy/deploy.sh

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

if [[ ! -f .env ]]; then
  echo "missing .env — copy from .env.example and set secrets"
  exit 1
fi

# shellcheck disable=SC1091
set -a
source .env
set +a

DOMAIN="${DOMAIN:-key.whistlelads.com}"
APP_PORT="${APP_PORT:-18745}"

mkdir -p /var/www/html
cp -f deploy/nginx-key.whistlelads.com.conf "/etc/nginx/sites-available/${DOMAIN}"
ln -sfn "/etc/nginx/sites-available/${DOMAIN}" "/etc/nginx/sites-enabled/${DOMAIN}"
nginx -t
systemctl reload nginx

docker compose up -d --build

# Issue/renew cert if missing
if [[ ! -d "/etc/letsencrypt/live/${DOMAIN}" ]]; then
  certbot --nginx -d "$DOMAIN" --non-interactive --agree-tos --register-unsafely-without-email --redirect
else
  certbot --nginx -d "$DOMAIN" --non-interactive --agree-tos --register-unsafely-without-email --redirect || true
fi

nginx -t
systemctl reload nginx

echo "deployed: https://${DOMAIN} (app :${APP_PORT})"
