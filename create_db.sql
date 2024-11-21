CREATE DATABASE greenlight;

\c greenlight

CREATE ROLE greenlight WITH LOGIN PASSWORD 'greenlight';

ALTER DATABASE greenlight OWNER TO greenlight;

CREATE EXTENSION IF NOT EXISTS citext;

ALTER USER greenlight WITH PASSWORD 'greenlight2';

ALTER USER greenlight WITH PASSWORD 'greenlight';