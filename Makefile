# ==================================================================================== #
# DEVELOPMENT
# ==================================================================================== #
create_dirs:
	mkdir bin 
	mkdir -p cmd/api
	mkdir -p internal/data internal/config internal/validator internal/mail
	mkdir migrations
	mkdir remote

go_mod_init:
	go mod init greenlight.zzh.net

go_install:
	go install github.com/rakyll/hey@latest
	go install honnef.co/go/tools/cmd/staticcheck@latest

go_get:
	go get github.com/julienschmidt/httprouter
	go get github.com/jackc/pgx/v5
	go get golang.org/x/time/rate
	go get golang.org/x/crypto/bcrypt
	go get github.com/jordan-wright/email
	go get github.com/spf13/viper
	go get github.com/fsnotify/fsnotify
	go get github.com/tomasen/realip

postgres_run:
	docker run --name postgres17 -p 5432:5432 -e POSTGRES_USER=root -e POSTGRES_PASSWORD=root -d postgres:latest

postgres_start:
	docker container start postgres17

psql_root:
	docker exec -it postgres17 psql -U root

psql_greenlight:
	docker exec -it postgres17 psql --dbname=greenlight --username=greenlight

export_db_dsn:
	export GREENLIGHT_DB_DSN='postgres://greenlight:greenlight@localhost:5432/greenlight?sslmode=disable'

migrate_create:
	migrate create -seq -ext=.sql -dir=./migrations create_movie_table
	migrate create -seq -ext=.sql -dir=./migrations add_movie_check_constraints
	migrate create -seq -ext .sql -dir ./migrations add_movie_indexes
	migrate create -seq -ext=.sql -dir=./migrations create_users_table
	migrate create -seq -ext .sql -dir ./migrations create_token_table
	migrate create -seq -ext .sql -dir ./migrations add_permissions

confirm:
	@echo -n 'Are you sure? [y/N] ' && read ans && [ $${ans:-N} = y]

migrate_up: confirm
	migrate -path ./migrations -database "$(GREENLIGHT_DB_DSN)" up

migrate_down: confirm
	migrate -path ./migrations -database "$(GREENLIGHT_DB_DSN)" down

migrate_version:
	migrate -path ./migrations -database "$(GREENLIGHT_DB_DSN)" version

migrate_force: confirm
	migrate -path ./migrations -database "$(GREENLIGHT_DB_DSN)" force "$(force_version)"

# ==================================================================================== #
# QUALITY CONTROL
# ==================================================================================== #

## tidy: format all .go files and tidy module dependencies
# @echo 'Formatting .go files ...'
# go fmt ./...
# @echo 'Verifying and vendoring module dependencies ...'
# go mod verify
# go mod vendor
tidy:
	@echo 'Tidying module dependencies ...'
	go mod tidy

## audit: run quality control checks
audit:
	@echo 'Checking module dependencies'
	go mod tidy -diff
	go mod verify
	@echo 'Vetting code ...'
	go vet ./...
	staticcheck ./...
	@echo 'Running tests ...'
	go test -race -vet=off ./...

# ==================================================================================== #
# BUILD
# ==================================================================================== #

## build/api: build the cmd/api application
build/api:
	@echo 'Building cmd/api ...'
	go build -ldflags='-s' -o=./bin/api ./cmd/api

.PHONY: create_dirs go_mod_init go_install go_get postgres_run postgres_start psql_root psql_greenlight export_db_dsn \
        migrate_create confirm migrate_up migrate_down migrate_version migrate_force \
		tidy audit build/api