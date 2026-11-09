#!/bin/sh
set -e

# Submissions in /var/lib/mailbot are kept, even on purge.
if [ -d /run/systemd/system ]; then
    systemctl daemon-reload >/dev/null || true
fi
