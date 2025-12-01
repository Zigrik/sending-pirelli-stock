# Сервер загрузки отчетов PIRELLI

Сервер для автоматизированной загрузки CSV отчетов в систему PIRELLI.

## Быстрый запуск

### 1. Установка
```bash
go mod tidy
go build -o report-server

Версия: 1.0.0
Лицензия: MIT

##```env
# Конфигурация сервера
SERVER_PORT=8080
ADMIN_PASSWORD=change_this_password

# Данные аутентификации PIRELLI
BASE_URL=https://reports.pirelli.ru/local/templates/dealer/ajax/api.php
COMPANY_NAME=YOUR_COMPANY_NAME
AUTH_LOGIN=your_login
AUTH_TOKEN=your_token_here

# Настройки автоматической отправки
# UPLOAD_TIME=09:00
# UPLOAD_DAY=1
# CSV_FILE_PATH=./report.csv

1. Статус сервера
GET /api/status

2. Загрузка файла через API
POST /api/upload

3. Веб-интерфейс
GET / - веб-форма для загрузки файлов
