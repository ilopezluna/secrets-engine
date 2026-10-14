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

ARG GO_VERSION=1.27.2
FROM golang:${GO_VERSION} AS build
WORKDIR /src
RUN --mount=type=bind,target=. \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -mod=vendor -tags filestore -o /keyring-repro ./store/cmd/gnome-keyring-repro

FROM ubuntu:24.04
RUN apt-get update && \
    DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
        gnome-keyring dbus libglib2.0-bin && \
    rm -rf /var/lib/apt/lists/* && \
    rm -f /usr/share/dbus-1/services/org.freedesktop.secrets.service && \
    mkdir -p /tmp/keyring-runtime && chmod 700 /tmp/keyring-runtime
COPY --from=build /keyring-repro /usr/local/bin/keyring-repro
COPY store/scripts/gnome-keyring-repro/run.sh /usr/local/bin/run-reproducer
ENV XDG_DATA_HOME=/tmp/keyring-data \
    XDG_RUNTIME_DIR=/tmp/keyring-runtime \
    KEYCHAIN_REPRO_ISOLATED=1
ENTRYPOINT ["timeout", "--signal=TERM", "--kill-after=10s", "5m", "dbus-run-session", "--", "bash", "/usr/local/bin/run-reproducer"]
