create_dirs:
	mkdir -p bin cmd/api internal migrations remote

go_mod_init:
	go mod init greenlight.zzh.net

go_install:
	go install github.com/rakyll/hey@latest

go_get:
	go get github.com/julienschmidt/httprouter@v1
	go get github.com/lib/pq@v1
	go get golang.org/x/time/rate@latest

postgres_run:
	docker run --name postgres17 -p 5432:5432 -e POSTGRES_USER=root -e POSTGRES_PASSWORD=root -d postgres:latest

postgres_start:
	docker container start postgres17

psql_root:
	docker exec -it postgres17 psql -U root

psql_greenlight:
	docker exec -it postgres17 psql --dbname=greenlight --username=greenlight

migrate_create:
	migrate create -seq -ext=.sql -dir=./migrations create_movie_table
	migrate create -seq -ext=.sql -dir=./migrations add_movie_check_constraints
	migrate create -seq -ext .sql -dir ./migrations add_movie_indexes

migrate_up:
	migrate -path ./migrations -database "$(GREENLIGHT_DB_DSN)" up

migrate_down:
	migrate -path ./migrations -database "$(GREENLIGHT_DB_DSN)" down

.PHONY: create_dirs go_mod_init go_install go_get postgres_run postgres_start psql_root psql_greenlight migrate_create migrate_up migrate_down