create_dirs:
	mkdir -p bin cmd/api internal migrations remote

go_mod_init:
	go mod init greenlight.zzh.net

go_install:
	go install github.com/rakyll/hey

go_get:
	go get github.com/julienschmidt/httprouter
	go get github.com/jackc/pgx/v5
	go get golang.org/x/time/rate
	go get golang.org/x/crypto/bcrypt
	go get github.com/jordan-wright/email
	go get github.com/spf13/viper
	go get github.com/fsnotify/fsnotify

postgres_run:
	docker run --name postgres17 -p 5432:5432 -e POSTGRES_USER=root -e POSTGRES_PASSWORD=root -d postgres:latest

postgres_start:
	docker container start postgres17

psql_root:
	docker exec -it postgres17 psql -U root

psql_greenlight:
	docker exec -it postgres17 psql --dbname=greenlight --username=greenlight

export_envs:
	export GREENLIGHT_ENV='development'
	export GREENLIGHT_DB_DSN='postgres://greenlight:greenlight@localhost:5432/greenlight?sslmode=disable'
	export GREENLIGHT_SMTP_USERNAME='XXXXXXXXXX'
	export GREENLIGHT_SMTP_PASSWORD='XXXXXXXXXX'

migrate_create:
	migrate create -seq -ext=.sql -dir=./migrations create_movie_table
	migrate create -seq -ext=.sql -dir=./migrations add_movie_check_constraints
	migrate create -seq -ext .sql -dir ./migrations add_movie_indexes
	migrate create -seq -ext=.sql -dir=./migrations create_users_table

migrate_up:
	@migrate -path ./migrations -database "$(GREENLIGHT_DB_DSN)" up

migrate_down:
	@migrate -path ./migrations -database "$(GREENLIGHT_DB_DSN)" down

migrate_version:
	@migrate -path ./migrations -database "$(GREENLIGHT_DB_DSN)" version

migrate_force:
	@migrate -path ./migrations -database "$(GREENLIGHT_DB_DSN)" force "$(force_version)"

.PHONY: create_dirs go_mod_init go_install go_get postgres_run postgres_start psql_root psql_greenlight export_db_dsn \
        migrate_create migrate_up migrate_down migrate_version migrate_force