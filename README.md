# PR Allocation Service

Микросервис для автоматического назначения ревьюеров на Pull Request'ы.

## Описание
Старался придерживаться английского языка в комментариях к коду (меньше свитчей раскладки), хотя не уверен, что это принято в больших компаниях.
Сервис предоставляет HTTP API для управления командами, пользователями и Pull Request'ами с автоматическим назначением ревьюверов из команды автора.

## Запуск

```bash
docker compose up --build
```

Сервис доступен на http://localhost:8080

Остановка:
```bash
docker compose down -v
```

## API Endpoints

### Команды

**POST /team/add** - Создание команды с участниками
```bash
curl -X POST http://localhost:8080/team/add \
  -H "Content-Type: application/json" \
  -d '{
    "team_name": "backend",
    "members": [
      {"user_id": "u1", "username": "Alice", "is_active": true},
      {"user_id": "u2", "username": "Bob", "is_active": true}
    ]
  }'
```

**GET /team/get?team_name=<name>** - Получение команды

**POST /team/deactivateUsers** - Массовая деактивация пользователей команды с переназначением открытых PR
```bash
curl -X POST http://localhost:8080/team/deactivateUsers \
  -H "Content-Type: application/json" \
  -d '{"team_name": "backend"}'
```

### Пользователи

**POST /users/setIsActive** - Изменение активности пользователя
```bash
curl -X POST http://localhost:8080/users/setIsActive \
  -H "Content-Type: application/json" \
  -d '{"user_id": "u1", "is_active": false}'
```

**GET /users/getReview?user_id=<id>** - Получение PR'ов где пользователь назначен ревьювером

### Pull Requests

**POST /pullRequest/create** - Создание PR с автоматическим назначением до 2 ревьюверов из команды автора
```bash
curl -X POST http://localhost:8080/pullRequest/create \
  -H "Content-Type: application/json" \
  -d '{
    "pull_request_id": "pr-001",
    "pull_request_name": "Add feature",
    "author_id": "u1"
  }'
```

**POST /pullRequest/merge** - Merge PR (идемпотентная операция)
```bash
curl -X POST http://localhost:8080/pullRequest/merge \
  -H "Content-Type: application/json" \
  -d '{"pull_request_id": "pr-001"}'
```

**POST /pullRequest/reassign** - Переназначение ревьювера на другого из его команды
```bash
curl -X POST http://localhost:8080/pullRequest/reassign \
  -H "Content-Type: application/json" \
  -d '{
    "pull_request_id": "pr-001",
    "old_user_id": "u2"
  }'
```

### Статистика

**GET /statistics** - Получение статистики системы (количество PR, команд, пользователей, назначений по пользователям)

### Служебные

**GET /health** - Проверка работоспособности

## Бизнес-правила

### Автоназначение ревьюверов
- Выбираются только активные (isActive = true) пользователи из команды автора
- Автор исключается из ревьюверов
- Назначается до 2 ревьюверов случайным образом
- Если доступных меньше двух, назначается доступное количество

### Переназначение
- Невозможно после статуса MERGED
- Новый ревьювер выбирается из команды заменяемого ревьювера
- Исключаются автор и текущие ревьюверы

### Ограничения
- После MERGED изменение ревьюверов запрещено
- Неактивные пользователи не назначаются на ревью
- Операция merge идемпотентная

## Коды ошибок

- `TEAM_EXISTS` - команда уже существует
- `PR_EXISTS` - PR уже существует
- `PR_MERGED` - нельзя изменять после merge
- `NOT_ASSIGNED` - пользователь не назначен ревьювером
- `NO_CANDIDATE` - нет доступных кандидатов
- `NOT_FOUND` - ресурс не найден
- `INVALID_REQUEST` - некорректный запрос

## Технологии

- Go 1.23
- PostgreSQL 15.9
- Docker Compose
- gorilla/mux, uber-go/zap, spf13/viper

## Конфигурация

Переменные окружения в `config/.env`:
- `DATABASE_HOST` - хост БД (postgres)
- `DATABASE_PORT` - порт БД (5432)
- `SERVER_PORT` - порт сервиса (8080)

Миграции применяются автоматически из `init.sql`.

## Makefile

```bash
make build        # Собрать бинарник
make test         # Запустить тесты
make docker-up    # Запустить в Docker
make docker-down  # Остановить Docker
```

## Принятые решения

### Алгоритм выбора ревьюверов
Случайный выбор для равномерного распределения нагрузки без хранения состояния.

### Недостаток кандидатов
Назначается доступное количество (0, 1 или 2) для поддержки маленьких команд.

### Идемпотентность merge
При повторном вызове возвращается текущее состояние PR без ошибки.

### Хранение ревьюверов
TEXT[] массив в PostgreSQL с GIN индексом для быстрого поиска.

### Graceful Shutdown
Обработка SIGTERM/SIGINT с таймаутом 30 секунд для завершения активных запросов.

### Retry-логика БД
Экспоненциальный backoff (1s → 30s) до 10 попыток вместо фиксированного sleep.

## OpenAPI

Полная спецификация в `pkg/api/openapi.yml`.

