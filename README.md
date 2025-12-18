# PR Allocation System

Микросервисная система для управления pull request'ами и версионированием кода.

## Архитектура

```
┌──────────┐     ┌─────────────────┐     ┌────────────────────┐     ┌─────────────┐
│ Frontend │────▶│  User Gateway   │────▶│  PR Allocation     │────▶│  PostgreSQL │
│ (React)  │     │  (API Gateway)  │     │  (назначение PR)   │     │   (общая)   │
└──────────┘     └────────┬────────┘     └────────────────────┘     └──────┬──────┘
                          │                                                │
                          ▼                                                │
                 ┌────────────────────┐                                    │
                 │   Code Storage     │────────────────────────────────────┘
                 │ (хранение кода)    │
                 └────────────────────┘
```

## Сервисы

| Сервис | Порт | Назначение |
|--------|------|------------|
| Frontend | 3000 | React UI |
| User Gateway | 8082 | API Gateway, агрегация запросов |
| PR Allocation | 8080 | Управление командами, PR, ревьюверами |
| Code Storage | 8081 | Хранение коммитов и кода |

## Быстрый старт

```bash
# Клонировать и запустить
cp .env.example .env
docker compose up

# Открыть http://localhost:3000
```

## API Endpoints (User Gateway :8082)

### Команды
- `POST /api/team/create` — создать команду
- `GET /api/team/get?team_name=...` — получить команду

### Репозитории
- `POST /api/repo/init` — инициализировать репо (multipart: team_name, repo_name, commit_name, code)
- `POST /api/repo/push` — push коммита
- `GET /api/repo/checkout` — скачать код коммита
- `GET /api/repo/commits` — список коммитов

### Pull Requests
- `POST /api/pr/create` — создать PR
- `GET /api/pr/my` — мои PR
- `GET /api/pr/reviews` — PR на ревью
- `POST /api/pr/approve` — одобрить PR
- `POST /api/pr/reject` — отклонить PR

## Переменные окружения

```env
POSTGRES_USER=postgres
POSTGRES_PASSWORD=postgres
```

## Структура проекта

```
├── pr-allocation-service/   # Управление PR и командами
├── code-storage-service/    # Хранение кода и коммитов
├── user-gateway-service/    # API Gateway
├── frontend/                # React + Vite + shadcn
├── shared/                  # Общие пакеты (logger)
└── compose.yaml
```

