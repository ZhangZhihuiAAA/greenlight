CREATE DATABASE greenlight;

\c greenlight

CREATE ROLE greenlight WITH LOGIN PASSWORD 'greenlight';

ALTER DATABASE greenlight OWNER TO greenlight;