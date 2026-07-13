#!/bin/bash
# One-time migration: move irc-glines-api / abuse_glines off the cron+start.sh
# polling loop and onto systemd, and prepare the directories/permissions/sudoers
# that the "irc-glines-api-prod" GitHub Actions self-hosted runner needs to
# deploy to this host.
#
# Run this ONCE, as root, on the production host (ircbl). It is written to be
# safe to re-run (idempotent) if something fails partway through.
#
# Usage: sudo ./migrate-to-systemd-cd.sh

set -euo pipefail

if [ "$(id -u)" -ne 0 ]; then
  echo "Must be run as root (sudo ./migrate-to-systemd-cd.sh)" >&2
  exit 1
fi

REPO_DIR=/home/ircbl/go/irc-glines-api
ABUSE_DIR=$REPO_DIR/mod.abuse_glines
FRONTEND_DIST_DIR=$ABUSE_DIR/frontent-glines/dist
SERVICE_USER=ircbl
RUNNER_USER=gha-runner

for path in "$REPO_DIR" "$ABUSE_DIR"; do
  if [ ! -d "$path" ]; then
    echo "Expected directory $path not found -- aborting." >&2
    exit 1
  fi
done

if ! id "$RUNNER_USER" >/dev/null 2>&1; then
  echo "OS user '$RUNNER_USER' does not exist -- expected it to already exist on this host." >&2
  exit 1
fi

echo "== 1/7: disabling the cron polling loop =="
crontab -u "$SERVICE_USER" -l > /tmp/ircbl-crontab.bak
sed -E 's#^([^#].*go/irc-glines-api/start2?\.sh.*)#\#\1#' /tmp/ircbl-crontab.bak | crontab -u "$SERVICE_USER" -
echo "   backed up previous crontab to /tmp/ircbl-crontab.bak"

echo "== 2/7: stopping cron-managed processes (systemd will take over) =="
# start.sh/start2.sh invoke the binaries by relative path (./irc-glines-api),
# so match on the basename, not the absolute path, or this won't find them.
pkill -u "$SERVICE_USER" -f '(^|/)irc-glines-api$' 2>/dev/null || true
pkill -u "$SERVICE_USER" -f '(^|/)abuse_glines$' 2>/dev/null || true
sleep 1

echo "== 3/7: creating gha-runner-owned deploy directories =="
mkdir -p "$REPO_DIR/bin" "$ABUSE_DIR/bin"
# Seed with the currently-built binaries so the services can start immediately,
# before the first CI-driven deploy replaces them.
[ -f "$REPO_DIR/bin/irc-glines-api" ] || cp "$REPO_DIR/irc-glines-api" "$REPO_DIR/bin/irc-glines-api"
[ -f "$ABUSE_DIR/bin/abuse_glines" ] || cp "$ABUSE_DIR/abuse_glines" "$ABUSE_DIR/bin/abuse_glines"
chown -R "$RUNNER_USER:$RUNNER_USER" "$REPO_DIR/bin" "$ABUSE_DIR/bin"
chmod 755 "$REPO_DIR/bin/irc-glines-api" "$ABUSE_DIR/bin/abuse_glines"

mkdir -p "$FRONTEND_DIST_DIR"
chown -R "$RUNNER_USER:$RUNNER_USER" "$FRONTEND_DIST_DIR"

echo "== 4/7: installing systemd units =="
cat > /etc/systemd/system/irc-glines-api.service <<EOF
[Unit]
Description=irc-glines-api gline lookup API
After=network.target

[Service]
Type=simple
User=$SERVICE_USER
Group=$SERVICE_USER
WorkingDirectory=$REPO_DIR
ExecStart=$REPO_DIR/bin/irc-glines-api
Restart=always
RestartSec=2

[Install]
WantedBy=multi-user.target
EOF

cat > /etc/systemd/system/abuse-glines.service <<EOF
[Unit]
Description=abuse_glines self-service gline removal API
After=network.target irc-glines-api.service
Wants=irc-glines-api.service

[Service]
Type=simple
User=$SERVICE_USER
Group=$SERVICE_USER
WorkingDirectory=$ABUSE_DIR
ExecStart=$ABUSE_DIR/bin/abuse_glines
Restart=always
RestartSec=2

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload

echo "== 5/7: installing sudoers rule for $RUNNER_USER =="
SUDOERS_FILE=/etc/sudoers.d/gha-runner-irc-glines-api
cat > "$SUDOERS_FILE.tmp" <<EOF
$RUNNER_USER ALL=(root) NOPASSWD: /usr/bin/systemctl restart irc-glines-api.service
$RUNNER_USER ALL=(root) NOPASSWD: /usr/bin/systemctl restart abuse-glines.service
EOF
visudo -cf "$SUDOERS_FILE.tmp"
chmod 440 "$SUDOERS_FILE.tmp"
mv "$SUDOERS_FILE.tmp" "$SUDOERS_FILE"

echo "== 6/7: enabling and starting the new services =="
systemctl enable --now irc-glines-api.service
systemctl enable --now abuse-glines.service
sleep 2

echo "== 7/7: status =="
systemctl --no-pager status irc-glines-api.service abuse-glines.service | cat

cat <<'EOF'

Done.

Reminders:
  - mod.abuse_glines/config.json is currently world-readable (mode 644).
    Recommended: chmod 600 /home/ircbl/go/irc-glines-api/mod.abuse_glines/config.json
  - This host still needs its OWN GitHub Actions runner registration dedicated
    to hiddn/irc-glines-api (the existing gha-runner/actions-runner instance on
    this box belongs to a different repo). See the runner registration steps
    printed separately.

Rollback (if something is wrong):
  1. systemctl disable --now irc-glines-api.service abuse-glines.service
  2. crontab -u ircbl /tmp/ircbl-crontab.bak    # restores the old polling loop
  3. rm /etc/systemd/system/irc-glines-api.service /etc/systemd/system/abuse-glines.service
  4. rm /etc/sudoers.d/gha-runner-irc-glines-api
  5. systemctl daemon-reload
EOF
