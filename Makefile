.PHONY: up
up:
	docker-compose up -d db
	while true ; do docker-compose exec db pg_isready && break ; sleep 0.1; done
	GO111MODULE=on go run main.go --config=./config/config.yml

# Apply new migrations (if exist)
.PHONY: migrate
migrate:
	docker-compose up -d db
	while true ; do docker-compose exec db pg_isready && break ; sleep 0.1; done
	GO111MODULE=on go run main.go migrate --config=./config/config.yml

# drop old and create a new database
.PHONY: recreate_db
recreate_db:
	docker-compose rm -sf db
	make migrate

# console access to database
.PHONY: psql
psql:
	docker-compose exec db psql -U backoffice backoffice