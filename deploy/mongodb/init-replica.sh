#!/bin/bash
set -e
for attempt in {1..60}; do
  if mongosh --host mongodb --quiet --username "$MONGO_INITDB_ROOT_USERNAME" --password "$MONGO_INITDB_ROOT_PASSWORD" --authenticationDatabase admin --eval 'db.adminCommand({ping:1})' >/dev/null 2>&1; then
    mongosh --host mongodb --quiet --username "$MONGO_INITDB_ROOT_USERNAME" --password "$MONGO_INITDB_ROOT_PASSWORD" --authenticationDatabase admin --eval '
      try { rs.status() } catch (e) { if (e.code === 94) { rs.initiate({_id:"rs0",members:[{_id:0,host:"mongodb:27017"}]}) } else { throw e } }
    '
    exit 0
  fi
  sleep 2
done
exit 1
