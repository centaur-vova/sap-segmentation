
# SAP Segmentation Import

![CI](https://github.com/centaur-vova/sap-segmentation/workflows/CI/badge.svg)
![Go Version](https://img.shields.io/badge/Go-1.26-blue.svg)
![License](https://img.shields.io/badge/License-MIT-green.svg)

Модуль импорта данных сегментации из ERP системы в PostgreSQL.

## Возможности

- 🔄 Импорт данных с пагинацией (batch processing)
- 💾 UPSERT — обновление существующих записей (ON CONFLICT)
- 📝 Структурированное логирование (slog) в консоль и файл
- 🧹 Автоматическая очистка старых логов
- 🔁 Retry с exponential backoff
- 🚀 Graceful shutdown (обработка SIGTERM/SIGINT)
- ⚙️ Конфигурация через environment variables
- 🧪 Unit тесты с покрытием 57%+
- 🐳 Docker и Docker Compose
- 🎭 Mock-сервер для тестирования

## Архитектура

```
                    ┌─────────────┐
                    │   ERP API   │
                    └──────┬──────┘
                           │ HTTP (Basic Auth)
                           ▼
┌─────────────────────────────────────────────┐
│              Importer (Go)                  │
│  ┌─────────┐  ┌─────────┐  ┌─────────────┐ │
│  │ Fetcher │→│ Worker  │→│ PostgreSQL   │ │
│  │ (HTTP)  │  │ (Batch) │  │ (UPSERT)    │ │
│  └─────────┘  └─────────┘  └─────────────┘ │
│        ↓           ↓                        │
│   CONN_INTERVAL  Batch Insert               │
└─────────────────────────────────────────────┘
```

## Структура проекта

```
.
├── cmd/
│   ├── sap_segmentationd/    # Основное приложение
│   └── mock_erp/             # Mock ERP сервер
├── internal/
│   ├── config/               # Конфигурация (envconfig)
│   ├── importer/             # Логика импорта
│   ├── logger/               # Логирование (slog)
│   └── model/                # Модели БД (sqlx)
├── setup/
│   └── install.sql           # SQL миграция
├── .github/
│   └── workflows/            # CI/CD
├── Dockerfile                # Production образ
├── Dockerfile.mock            # Mock сервер образ
├── docker-compose.yaml        # Оркестрация
├── Makefile                   # Команды управления
└── .golangci.yaml            # Конфигурация линтера
```

## Быстрый старт

### Через Docker Compose

```bash
# Скопировать конфигурацию
cp .env.example .env

# Запустить (PostgreSQL + Mock API + Importer)
docker compose up -d

# Логи
docker compose logs -f sap_segmentation

# Проверить данные
docker compose exec postgres psql -U postgres -d mesh_group -c "SELECT COUNT(*) FROM segmentation;"
```

### Локальный запуск

```bash
# Установить зависимости
go mod download

# Применить миграцию
psql -h localhost -U postgres -d mesh_group -f setup/install.sql

# Запустить mock-сервер (в отдельном терминале)
go run cmd/mock_erp/main.go

# Запустить импортер
go run cmd/sap_segmentationd/main.go
```

## Конфигурация

Все настройки через переменные окружения. См. `.env.example`.

### Основные параметры:

| Переменная | Описание | По умолчанию |
|-----------|----------|--------------|
| `DB_HOST` | IP адрес БД | `127.0.0.1` |
| `DB_PORT` | Порт БД | `5432` |
| `DB_NAME` | Название БД | `mesh_group` |
| `CONN_URI` | URL ERP API | `http://bsm.api.iql.ru/...` |
| `CONN_TIMEOUT` | Таймаут API (сек) | `5` |
| `CONN_INTERVAL` | Задержка между запросами (мс) | `1500` |
| `IMPORT_BATCH_SIZE` | Размер пачки | `50` |

## Как работает импорт

1. **Запрос данных**: Импортер запрашивает данные с пагинацией `p_limit` и `p_offset`
2. **Retry**: При ошибке повторяет запрос с exponential backoff (1s, 2s, 4s)
3. **Обработка**: Полученные данные отправляются в канал для воркера
4. **Вставка в БД**: Воркер выполняет UPSERT пачками
5. **Пауза**: Между запросами выдерживается `CONN_INTERVAL`
6. **Завершение**: Когда API возвращает пустой массив — импорт завершается

## Логирование

Логи пишутся в:
- **Консоль** — `stdout` (для Docker)
- **Файл** — `log/segmentation_import.log`

Формат: JSON или Text (настраивается через `LOG_FORMAT`).

## Тестирование

```bash
# Запустить все тесты
make test

# С покрытием
make test-cover

# Race detector
make test-race

# Линтер
make lint
```

## CI/CD

GitHub Actions автоматически:
- Запускает линтер (golangci-lint)
- Запускает тесты с race detector
- Проверяет покрытие

## Лицензия

MIT License. См. [LICENSE](LICENSE).
