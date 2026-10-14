#!/bin/bash

# Copyright 2026 Docker, Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -euo pipefail

if [[ "${KEYCHAIN_REPRO_ISOLATED:-}" != 1 ]]; then
    echo "Run with make keychain-gnome-repro to use a disposable container" >&2
    exit 1
fi

mkdir -p "$XDG_DATA_HOME/keyrings" "$XDG_RUNTIME_DIR"
chmod 700 "$XDG_DATA_HOME" "$XDG_DATA_HOME/keyrings" "$XDG_RUNTIME_DIR"
cat > "$XDG_DATA_HOME/keyrings/login.keyring" <<'KEYRING'
[keyring]
display-name=login
ctime=0
mtime=0
lock-on-idle=false
lock-after=false
KEYRING
printf 'login\n' > "$XDG_DATA_HOME/keyrings/default"
dpkg-query -W gnome-keyring libglib2.0-0t64
uname -m

gnome-keyring-daemon --foreground --components=secrets > /tmp/keyring.log 2>&1 &
keyring_pid=$!

cleanup() {
    status=$?
    trap - EXIT
    if kill -0 "$keyring_pid" 2>/dev/null; then
        kill "$keyring_pid" 2>/dev/null || true
    fi
    keyring_status=0
    wait "$keyring_pid" || keyring_status=$?
    printf 'probe_exit=%s keyring_exit=%s\n' "$status" "$keyring_status"
    cat /tmp/keyring.log
    if (( status == 0 && keyring_status != 0 )); then
        status=1
    fi
    exit "$status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

ready=false
for ((attempt = 0; attempt < 100; attempt++)); do
    if ! kill -0 "$keyring_pid" 2>/dev/null; then
        echo "Keyring exited before becoming ready" >&2
        exit 1
    fi
    if gdbus call --session --dest org.freedesktop.DBus \
        --object-path /org/freedesktop/DBus \
        --method org.freedesktop.DBus.GetNameOwner org.freedesktop.secrets >/dev/null 2>&1; then
        ready=true
        break
    fi
    sleep 0.1
done
if [[ "$ready" != true ]]; then
    echo "Timed out waiting for the private Secret Service" >&2
    exit 1
fi

/usr/local/bin/keyring-repro "$@"
