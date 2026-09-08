# SAP Segmentation Import

![CI/CD](https://github.com/centaur-vova/sap-segmentation/workflows/CI/CD/badge.svg)
[![Coverage](https://img.shields.io/badge/coverage-58.8%25-brightgreen)](https://github.com/centaur-vova/sap-segmentation/actions)
![Go Version](https://img.shields.io/badge/Go-1.26-blue.svg)
![License](https://img.shields.io/badge/License-MIT-green.svg)

Модуль импорта данных сегментации из ERP системы в PostgreSQL.

## Возможности

- 🔄 Импорт с пагинацией (batch processing)
- 💾 UPSERT (ON CONFLICT DO UPDATE)
- 🔁 Retry с exponential backoff
- 📉 Batch Degradation (деградация до поштучной вставки)
- 📝 Структурированное логирование (slog)
- 🧹 Автоматическая очистка старых логов
- 🚀 Graceful shutdown
- ⚙️ Конфигурация через env
- 🧪 Unit тесты
- 🐳 Docker & Docker Compose
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
│  ┌─────────┐  ┌─────────┐  ┌─────────────┐  │
│  │ Fetcher │ →│ Worker  │ →│ PostgreSQL  │  │
│  │ (HTTP)  │  │ (Batch) │  │ (UPSERT)    │  │
│  └─────────┘  └─────────┘  └─────────────┘  │
│        ↓           ↓                        │
│   CONN_INTERVAL  Batch Insert               │
└─────────────────────────────────────────────┘
```

## Batch Degradation

Паттерн «Пакетная обработка с деградацией до поштучной»:

1. **Попытка Batch Insert**: Все записи одним SQL-запросом
2. **При ошибке**: Деградация до поштучной вставки
3. **Битые строки**: Логируются, но не прерывают импорт

```
Batch Insert (50 rows)
    ↓ Ошибка?
    ├── Нет → Успех (O(1))
    └── Да → Row-by-row insert
              ├── Валидные → Успешно вставлены
              └── Битые → Залогированы и пропущены
```

**Гарантия**: 49 из 50 записей сохранятся при ошибке в одной.

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
cp .env.example .env
docker compose up -d
docker compose logs -f sap_segmentation
docker compose exec postgres psql -U postgres -d mesh_group -c "SELECT COUNT(*) FROM segmentation;"
```

### Локальный запуск

```bash
go mod download
psql -h localhost -U postgres -d mesh_group -f setup/install.sql
go run cmd/mock_erp/main.go  # В отдельном терминале
go run cmd/sap_segmentationd/main.go
```

## Запуск Production-артефакта

Готовый Docker-образ собирается автоматически после прохождения тестов.

```bash
docker run --rm \
    -e DB_HOST="your-db-host" \
    -e DB_PORT="5432" \
    -e DB_NAME="mesh_group" \
    -e DB_USER="postgres" \
    -e DB_PASSWORD="your-password" \
    -e CONN_URI="http://your-erp-api" \
    -e CONN_AUTH_LOGIN_PWD="login:password" \
    -e CONN_USER_AGENT="spacecount-test" \
    -e CONN_TIMEOUT="5" \
    -e CONN_INTERVAL="1500" \
    -e IMPORT_BATCH_SIZE="50" \
    -e LOG_DIR="/log" \
    -e LOG_FILE="segmentation_import.log" \
    -e LOG_TO_FILE="true" \
    -e LOG_TO_CONSOLE="true" \
    -v $(pwd)/logs:/log \
 ghcr.io/centaur-vova/sap-segmentation:latest
```

### Преимущества Production-артефакта:

- **Безопасность**: Multi-stage сборка, образ ~20МБ без исходников
- **Производительность**: Статическая компиляция (CGO_ENABLED=0)
- **Логирование**: Флаг `-v ./logs:/log` монтирует логи на хост-машину
- **Автоматический выход**: Контейнер сам остановится после завершения импорта

## Конфигурация

| Переменная          | Описание                      | По умолчанию                |
| ------------------- | ----------------------------- | --------------------------- |
| `DB_HOST`           | IP адрес БД                   | `127.0.0.1`                 |
| `DB_PORT`           | Порт БД                       | `5432`                      |
| `DB_NAME`           | Название БД                   | `mesh_group`                |
| `CONN_URI`          | URL ERP API                   | `http://bsm.api.iql.ru/...` |
| `CONN_TIMEOUT`      | Таймаут API (сек)             | `5`                         |
| `CONN_INTERVAL`     | Задержка между запросами (мс) | `1500`                      |
| `IMPORT_BATCH_SIZE` | Размер пачки                  | `50`                        |

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
- **Файл** — `/log/segmentation_import.log` (в контейнере)

Формат: JSON или Text (настраивается через `LOG_FORMAT`).

## Тестирование

```bash
make test
make test-cover
make test-race
make lint
```

## CI/CD

GitHub Actions автоматически:

- Запускает линтер (golangci-lint)
- Запускает тесты с race detector
- Проверяет покрытие
- Собирает и публикует Docker образ в GHCR

## Соответствие ТЗ и Go Best Practices

> **Важное примечание по структуре файлов:**
> В техническом задании было указано требование создать модель по пути `model/Segmentation.go` с большой буквы.
> В итоговой реализации имя файла и пакета было изменено на строчные буквы: `internal/model/segmentation.go`.

### Почему это было сделано:

1. **Каноны компилятора Go:** Согласно официальному CodeReviewComments и Effective Go, имена пакетов и файлов должны быть написаны строго в нижнем регистре, в одно слово, без использования Snake_Case или CamelCase.
2. **Кроссплатформенная сборка (Case-Sensitivity):** Файловые системы Windows/macOS нечувствительны к регистру, однако Docker-контейнеры на базе Linux (Alpine) чувствительны к нему на 100%. Написание путей с заглавной буквы часто приводит к скрытым ошибкам компиляции вида `undefined: model` или `UndeclaredName` при сборке артефактов в CI/CD пайплайнах.
3. **Изоляция бизнес-логики:** Модель перенесена внутрь папки `internal/`, что предотвращает несанкционированный импорт внутренней логики модуля сторонними внешними системами на уровне компилятора.

## Лицензия

MIT License. См. [LICENSE](LICENSE).
