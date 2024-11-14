create_dirs:
	mkdir -p bin cmd/api internal migrations remote

go_mod_init:
	go mod init greenlight.zzh.net

go_install:
	go install github.com/rakyll/hey@latest

go_get:
	go get github.com/julienschmidt/httprouter@v1

.PHONY: create_dirs go_mod_init go_install go_get