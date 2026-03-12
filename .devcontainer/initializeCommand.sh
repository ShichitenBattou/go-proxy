#!/usr/bin/env bash
set -euo pipefail

ENV_FILE=".env"
KEY="HOST_WORKSPACE"
VALUE="$(pwd)"

if grep -q "^${KEY}=" "$ENV_FILE"; then
    sed -i "s|^${KEY}=.*|${KEY}=${VALUE}|" "$ENV_FILE"
else
    echo "${KEY}=${VALUE}" >> "$ENV_FILE"
fi