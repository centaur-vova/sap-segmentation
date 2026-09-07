# ============================================
# SAP Segmentation - Makefile
# ============================================

DOCKER_COMPOSE = docker compose
APP_NAME = sap_segmentation
POSTGRES_NAME = sap_postgres

GREEN  := $(shell tput -Txterm setaf 2)
YELLOW := $(shell tput -Txterm setaf 3)
RESET  := $(shell tput -Txterm sgr0)

.PHONY: help
help: ## Показать все команды
	@echo ''
	@echo 'Usage:'
	@echo '  ${YELLOW}make${RESET} ${GREEN}<target>${RESET}'
	@echo ''
	@echo 'Targets:'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  ${YELLOW}%-25s${RESET} ${GREEN}%s${RESET}\n", $$1, $$2}'

# ============================================
# Docker
# ============================================

.PHONY: up
up: ## Запустить (логи в консоли)
	$(DOCKER_COMPOSE) up

.PHONY: up-d
up-d: ## Запустить в фоне
	$(DOCKER_COMPOSE) up -d

.PHONY: down
down: ## Остановить
	$(DOCKER_COMPOSE) down

.PHONY: restart
restart: ## Перезапустить
	$(DOCKER_COMPOSE) restart

.PHONY: logs
logs: ## Логи приложения
	$(DOCKER_COMPOSE) logs -f $(APP_NAME)

.PHONY: logs-mock
logs-mock: ## Логи mock-сервера
	$(DOCKER_COMPOSE) logs -f mock_erp

.PHONY: ps
ps: ## Статус контейнеров
	$(DOCKER_COMPOSE) ps

.PHONY: build
build: ## Собрать образы
	$(DOCKER_COMPOSE) build

.PHONY: build-no-cache
build-no-cache: ## Собрать без кеша
	$(DOCKER_COMPOSE) build --no-cache

# ============================================
# База данных
# ============================================

.PHONY: db-shell
db-shell: ## Подключиться к БД
	docker exec -it $(POSTGRES_NAME) psql -U postgres -d mesh_group

.PHONY: db-count
db-count: ## Количество записей
	docker exec -it $(POSTGRES_NAME) psql -U postgres -d mesh_group -c "SELECT COUNT(*) FROM segmentation;"

.PHONY: db-reset
db-reset: ## Сбросить БД
	$(DOCKER_COMPOSE) down -v
	$(DOCKER_COMPOSE) up -d postgres
	@sleep 10

# ============================================
# Тестирование
# ============================================

.PHONY: test
test: ## Запустить все тесты
	go test ./... -v

.PHONY: test-cover
test-cover: ## Тесты с покрытием
	go test ./... -cover -coverprofile=coverage.out
	go tool cover -func=coverage.out
	@rm -f coverage.out

.PHONY: test-html
test-html: ## HTML отчет покрытия
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Отчет: coverage.html"

.PHONY: test-race
test-race: ## Тесты с race detector
	go test ./... -race -cover

# ============================================
# Линтеры
# ============================================

.PHONY: lint
lint: ## Запустить golangci-lint
	@command -v golangci-lint >/dev/null 2>&1 || { \
		echo "golangci-lint не установлен"; \
		exit 1; \
	}
	golangci-lint run ./...

.PHONY: fmt
fmt: ## Форматировать код
	go fmt ./...

.PHONY: vet
vet: ## Go vet
	go vet ./...

.PHONY: check-all
check-all: fmt vet lint test test-race ## Полная проверка
	@echo "${GREEN}✓ Все проверки пройдены${RESET}"

# ============================================
# Утилиты
# ============================================

.PHONY: clean
clean: ## Очистить
	rm -rf tmp/ bin/
	rm -f log/*.log coverage.out coverage.html

.PHONY: mock
mock: ## Запустить mock локально
	go run cmd/mock_erp/main.go

# ============================================
# Импорт
# ============================================

.PHONY: import
import: ## Запустить импорт заново
	$(DOCKER_COMPOSE) run --rm $(APP_NAME)

.PHONY: reimport
reimport: db-clean import ## Очистить БД и запустить импорт заново