#!/bin/bash
set -e
mkdir -p /run/mongo-keyfile
if [ ! -s /run/mongo-keyfile/key ]; then
  head -c 756 /dev/urandom | base64 > /run/mongo-keyfile/key
fi
chown mongodb:mongodb /run/mongo-keyfile/key
chmod 400 /run/mongo-keyfile/key
exec /usr/local/bin/docker-entrypoint.sh "$@"
