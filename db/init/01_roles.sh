#!/bin/bash
# Runs once, on first start of an empty Postgres volume (docker-entrypoint-initdb.d).
# Creates the NON-owner role the API connects as. Table owners bypass row-level
# security, so the API must never connect as POSTGRES_USER.
#
# Default privileges mean every table, sequence and function that migrations
# create later is automatically usable by the app role.
set -euo pipefail

psql -v ON_ERROR_STOP=1 -v app_pw="${APP_DB_PASSWORD:?APP_DB_PASSWORD must be set}" \
     --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<"SQL"
CREATE ROLE photobooth_app LOGIN PASSWORD :'app_pw' NOSUPERUSER NOBYPASSRLS;
GRANT CONNECT ON DATABASE :"DBNAME" TO photobooth_app;
GRANT USAGE ON SCHEMA public TO photobooth_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO photobooth_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT USAGE, SELECT ON SEQUENCES TO photobooth_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT EXECUTE ON FUNCTIONS TO photobooth_app;
SQL
