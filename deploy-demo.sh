#!/usr/bin/env bash
#
# ShareAPI demo deploy script (Ubuntu, root)
# - Installs Docker if missing
# - Builds the image FROM SOURCE (so branding changes are included)
# - Picks a currently free host port (prefers 3000)
# - Randomizes postgres/redis passwords + session secret (never 123456)
# - Prints the URL and first-login instructions
#
# Usage: sudo bash deploy-demo.sh
#
set -euo pipefail

REPO_URL="https://github.com/alvindingpeng/shareapi.git"
APP_DIR="/opt/shareapi"

log() { echo "[deploy] $*"; }
die() { echo "[deploy][ERROR] $*" >&2; exit 1; }

[ "$(id -u)" = "0" ] || die "please run as root (sudo bash deploy-demo.sh)"

# ---- 1. Docker ----
if ! command -v docker >/dev/null 2>&1; then
  log "installing docker..."
  apt-get update -qq
  apt-get install -y -qq docker.io docker-compose-plugin openssl curl > /dev/null
  systemctl enable --now docker
fi
docker compose version >/dev/null 2>&1 || die "docker compose plugin missing"

# ---- 2. Code ----
if [ -d "$APP_DIR/.git" ]; then
  log "updating $APP_DIR ..."
  git -C "$APP_DIR" fetch --depth 1 origin main
  git -C "$APP_DIR" reset --hard origin/main
else
  log "cloning $REPO_URL ..."
  git clone --depth 1 "$REPO_URL" "$APP_DIR"
fi
cd "$APP_DIR"

# ---- 3. Free port (prefer 3000, else first free candidate) ----
pick_port() {
  for p in 3000 8080 8000 9000 8888 3001 5000; do
    if ! ss -tln 2>/dev/null | grep -qE "[:.]$p[[:space:]]"; then echo "$p"; return 0; fi
  done
  python3 -c "import socket; s=socket.socket(); s.bind(('',0)); print(s.getsockname()[1])"
}
DEMO_PORT="$(pick_port)"
log "using host port: $DEMO_PORT"

# ---- 4. Secrets (generated once, kept in .env.demo) ----
if [ ! -f .env.demo ]; then
  PG_PASS="$(openssl rand -hex 16)"
  REDIS_PASS="$(openssl rand -hex 16)"
  SESSION_SECRET="$(openssl rand -hex 32)"
  cat > .env.demo <<EOF
# generated $(date -u +%FT%TZ) - do not commit, do not share
DEMO_PORT=$DEMO_PORT
PG_PASS=$PG_PASS
REDIS_PASS=$REDIS_PASS
SESSION_SECRET=$SESSION_SECRET
EOF
  chmod 600 .env.demo
  log "secrets generated -> .env.demo"
else
  # keep existing passwords, but allow a new port choice
  sed -i "s/^DEMO_PORT=.*/DEMO_PORT=$DEMO_PORT/" .env.demo
  log "reusing existing .env.demo (new port $DEMO_PORT)"
fi
set -a; . ./.env.demo; set +a

# ---- 5. Compose override: build from source + free port + real secrets ----
cat > docker-compose.demo.yml <<'EOF'
# Demo overlay: extends docker-compose.yml
#  - builds the app image from this repo (branding changes included)
#  - maps a free host port
#  - uses generated secrets instead of 123456 defaults
services:
  new-api:
    image: shareapi:demo
    build:
      context: .
      dockerfile: Dockerfile
    ports:
      - "${DEMO_PORT}:3000"
    environment:
      - SQL_DSN=postgresql://root:${PG_PASS}@postgres:5432/new-api
      - REDIS_CONN_STRING=redis://:${REDIS_PASS}@redis:6379
      - SESSION_SECRET=${SESSION_SECRET}
      - TZ=Asia/Shanghai
      - ERROR_LOG_ENABLED=true
      - BATCH_UPDATE_ENABLED=true
  postgres:
    environment:
      POSTGRES_USER: root
      POSTGRES_PASSWORD: ${PG_PASS}
      POSTGRES_DB: new-api
  redis:
    command: ["redis-server", "--requirepass", "${REDIS_PASS}"]
EOF

# ---- 6. Build & up ----
log "building image from source (this takes a while, ~10-20 min on 4C8G)..."
# Workaround: the default builder can fail on pinned image digests
# ("failed commit on ref ... unexpected commit digest"); a docker-container
# driver builder does not hit this bug.
docker buildx create --name shareapi-builder --driver docker-container --use 2>/dev/null \
  || docker buildx use shareapi-builder 2>/dev/null || true
docker compose -f docker-compose.yml -f docker-compose.demo.yml build new-api
log "starting services..."
docker compose -f docker-compose.yml -f docker-compose.demo.yml up -d

# ---- 7. Wait for healthy ----
log "waiting for the app to become healthy..."
for i in $(seq 1 40); do
  if docker exec new-api wget -q -O - http://localhost:3000/api/status 2>/dev/null | grep -q '"success":[[:space:]]*true'; then
    log "app is UP"
    break
  fi
  sleep 10
  [ "$i" = "40" ] && die "app did not become healthy; check: docker logs new-api"
done

# ---- 8. Firewall best-effort ----
if command -v ufw >/dev/null 2>&1 && ufw status 2>/dev/null | grep -q "Status: active"; then
  ufw allow "$DEMO_PORT"/tcp >/dev/null && log "ufw: opened $DEMO_PORT/tcp"
fi

SERVER_IP="$(curl -s --max-time 8 ifconfig.me 2>/dev/null || hostname -I 2>/dev/null | awk '{print $1}')"
echo
echo "=================== ShareAPI demo is live ==================="
echo "URL:      http://$SERVER_IP:$DEMO_PORT"
echo "Admin:    root / 123456   <-- CHANGE THIS PASSWORD ON FIRST LOGIN"
echo "Compose:  docker compose -f docker-compose.yml -f docker-compose.demo.yml"
echo "Logs:     docker logs -f new-api"
echo "============================================================="
echo "Next: log in as root, change the password, then add your API key"
echo "channels (Channel management) to start relaying."
