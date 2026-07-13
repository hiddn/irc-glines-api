#!/bin/bash
# Move irc-glines-api / abuse-glines logs out of the general journal ->
# /var/log/syslog path and into their own dedicated files, via systemd
# drop-in overrides (StandardOutput/StandardError=append:...).
#
# Run this ONCE, as root, on the production host (ircbl). It is written to be
# safe to re-run (idempotent).
#
# Usage: sudo ./separate-service-logs.sh

set -euo pipefail

if [ "$(id -u)" -ne 0 ]; then
  echo "Must be run as root (sudo ./separate-service-logs.sh)" >&2
  exit 1
fi

SERVICE_USER=ircbl

declare -A SERVICE_LOG=(
  [irc-glines-api.service]=/var/log/irc-glines-api.log
  [abuse-glines.service]=/var/log/abuse-glines.log
)

for svc in "${!SERVICE_LOG[@]}"; do
  if [ ! -f "/etc/systemd/system/$svc" ]; then
    echo "Expected unit /etc/systemd/system/$svc not found -- aborting." >&2
    exit 1
  fi
done

echo "== 1/4: creating log files =="
for svc in "${!SERVICE_LOG[@]}"; do
  log="${SERVICE_LOG[$svc]}"
  touch "$log"
  chown "$SERVICE_USER:$SERVICE_USER" "$log"
  chmod 640 "$log"
  echo "   $log"
done

echo "== 2/4: installing systemd drop-in overrides =="
for svc in "${!SERVICE_LOG[@]}"; do
  log="${SERVICE_LOG[$svc]}"
  dropin_dir="/etc/systemd/system/$svc.d"
  mkdir -p "$dropin_dir"
  cat > "$dropin_dir/logging.conf" <<EOF
[Service]
StandardOutput=append:$log
StandardError=append:$log
EOF
  echo "   $dropin_dir/logging.conf -> $log"
done

echo "== 3/4: installing logrotate config =="
cat > /etc/logrotate.d/irc-glines-api-services <<EOF
${SERVICE_LOG[irc-glines-api.service]} ${SERVICE_LOG[abuse-glines.service]} {
    daily
    rotate 14
    compress
    delaycompress
    missingok
    notifempty
    copytruncate
    su $SERVICE_USER $SERVICE_USER
}
EOF

echo "== 4/4: reloading and restarting services =="
systemctl daemon-reload
systemctl restart irc-glines-api.service abuse-glines.service
sleep 2
systemctl --no-pager status irc-glines-api.service abuse-glines.service | cat

cat <<'EOF'

Done.

From now on:
  - tail -f /var/log/irc-glines-api.log
  - tail -f /var/log/abuse-glines.log
  - These no longer flow into /var/log/syslog (journald still keeps a copy
    reachable via `journalctl -u irc-glines-api` / `-u abuse-glines` unless
    you also disable journal storage for them).

Rollback (if something is wrong):
  1. rm -rf /etc/systemd/system/irc-glines-api.service.d /etc/systemd/system/abuse-glines.service.d
  2. rm /etc/logrotate.d/irc-glines-api-services
  3. systemctl daemon-reload
  4. systemctl restart irc-glines-api.service abuse-glines.service
EOF
