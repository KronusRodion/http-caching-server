# HTTP Caching Server

## Описание
HTTP-сервер для хранения и раздачи электронных документов с поддержкой кэширования через Redis и аутентификацией. Сервер реализует REST API для управления документами, включая загрузку, получение, удаление и кэширование данных.

---

## Основные функции
- **Регистрация и аутентификация пользователей**
- **Загрузка документов с метаданными и JSON-данными**
- **Получение списка документов (GET/HEAD)**
- **Получение одного документа (GET/HEAD)**
- **Удаление документа**
- **Завершение авторизованной сессии**
- **Кэширование данных в Redis**
- **Инвалидация кэша при изменении данных**

---

## Технологии
- **Go** (язык программирования)
- **PostgreSQL** (основное хранилище данных)
- **Redis** (кэширование данных)
- **Gorilla Mux** (маршрутизация)
- **pgx** (PostgreSQL драйвер)
- **go-redis** (Redis клиент)
- **JWT** (аутентификация)

---

## Установка и запуск

### 1. Установите зависимости
```bash
go mod download
```

### 2. Создайте файл с зависимостями
```bash
    # PostgreSQL - Строка подключения к PostgreSQL
    DATABASE_URL=postgres://postgres:1234@localhost:5432/postgres?sslmode=disable 

    # JWT - Секретный ключ, ADMIN_TOKEN - токен для проверки прав админа
    JWT=secret_key_JWT
    ADMIN_TOKEN=SECURITY_ADMIN_TOKEN_SDKMLJKAISI

    # Redis
    REDIS_ADDRESS=localhost:6379
    REDIS_PASSWORD=YOUR_Password
    REDIS_USER=default
    REDIS_DB=0
    MAX_RETRIES=3
    DIAL_TIMEOUT=5s
    TIMEOUT=10s
```

### 3. Запустите PostgreSQL и Redis
```bash
    # Пример
    # Запуск PostgreSQL
    docker run --name postgres -e POSTGRES_PASSWORD=1234 -p 5432:5432 -d postgres

    # Запуск Redis
    docker run --name redis -p 6379:6379 -d redis
```

### 4. Запустите сервер
```bash
    go run cmd/main.go
```


## API Документация
### 1. Регистрация пользователя
POST /api/register

Параметры:
```bash
json

{
  "token": "admin_token",
  "login": "user123",
  "pswd": "Password!123"
}
```
Ответ:
```bash
json
{
  "response": {
    "login": "user123"
  }
}
```
### 2. Аутентификация
POST /api/auth

Параметры:
```bash
json

{
  "login": "user123",
  "pswd": "Password!123"
}
```
Ответ:
```bash
json

{
  "response": {
    "token": "generated_jwt_token"
  }
}
```
### 3. Загрузка документа
POST /api/docs
Формат запроса (multipart/form-data):

meta: JSON-строка с метаданными
json: JSON-данные документа (опционально)
file: Бинарный файл
Пример метаданных:

```bash

json

{
  "name": "photo.jpg",
  "file": true,
  "public": false,
  "token": "user_token",
  "mime": "image/jpeg",
  "grant": ["login1", "login2"]
}
```
Ответ:
```bash
json

{
  "data": {
    "json": { /* JSON-данные */ },
    "file": "photo.jpg"
  }
}
```
### 4. Получение списка документов
GET/HEAD /api/docs

Параметры запроса:

```bash
token: Токен пользователя
login (опционально): Логин владельца
key, value (опционально): Фильтры
limit: Ограничение на количество документов
```
Пример ответа:
```bash
json

{
  "data": {
    "docs": [
      {
        "id": "1",
        "name": "photo.jpg",
        "mime": "image/jpeg",
        "file": true,
        "public": false,
        "created": "2025-07-11 12:00:00",
        "grant": ["login1", "login2"]
      }
    ]
  }
}
```

### 5. Получение документа
GET/HEAD /api/docs/{id}

Параметры запроса:
```bash
token: Токен пользователя
```

Ответ:
```bash
Если JSON + file:

Бинарный файл отдаётся с правильным Content-Type и mime
json

{
  "data": { /* JSON-данные */ }
}
Если только бинарный файл: Отдаётся с правильным Content-Type и mime
```
### 6. Удаление документа
DELETE /api/docs/{id}

Параметры запроса:
```bash
token: Токен пользователя
```
Ответ:
```bash
json

{
  "response": {
    "qwdj1q4o34u34ih759ou1": true
  }
}
```
### 7. Завершение сессии
DELETE /api/auth/{token}

Ответ:
```bash
json

{
  "response": {
    "qwdj1q4o34u34ih759ou1": true
  }
}
```

Стандартный формат ответа
```bash
json

{
  "error": {
    "code": 123,
    "text": "so sad"
  },
  "response": {
    "token": "sfuqwejqjoiu93e29"
  },
  "data": {
    "docs": [ /* Список документов */ ]
  }
}
```

Поля:
```bash
error: Присутствует при ошибках
response: Информация об успешной операции
data: Данные (файлы, JSON)
