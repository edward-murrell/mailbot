#!/bin/sh
set -e

if [ "$1" != "configure" ]; then
    exit 0
fi

if [ -d /run/systemd/system ]; then
    systemctl daemon-reload >/dev/null || true
fi

if [ -z "$2" ]; then
    # Fresh install. The service needs SMTP settings before it can start.
    echo "mailbot: edit /etc/mailbot/mailbot.env, then run:"
    echo "  systemctl enable --now mailbot"
elif [ -d /run/systemd/system ]; then
    # Upgrade. Restart any instances that were running.
    systemctl try-restart mailbot.service 'mailbot@*.service' >/dev/null || true
fi
