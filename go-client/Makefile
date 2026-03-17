.PHONY: help sqlc build run clean

help: ## Mostra questo help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

sqlc: ## Genera codice da sqlc
	~/go/bin/sqlc generate

generate: sqlc build ## Rigenera sqlc e compila

build: ## Compila il progetto
	go build -o mqtt_client

run: build ## Compila ed esegue il progetto
	./mqtt_client

clean: ## Pulisce i file compilati
	rm -f mqtt_client

tidy: ## Aggiorna le dipendenze Go
	go mod tidy

tern-status: ## Mostra stato migrazioni
	tern status -m migrations/

tern-migrate: ## Esegui migrazioni pendenti
	tern migrate -m migrations/

test: ## Esegue i test
	go test ./...

lint: ## Esegue linting (richiede golangci-lint)
	golangci-lint run
