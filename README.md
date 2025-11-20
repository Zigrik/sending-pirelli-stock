# sending-pirelli-stock
Программа для автоматизации отправкм остатков в Pirelli


# Переменные окружения в .env :

# Конфигурация сервера
SERVER_PORT=8080
ADMIN_PASSWORD=admin123

# Данные аутентификации PIRELLI
BASE_URL=https://reports.pirelli.ru/local/templates/dealer/ajax/api.php
COMPANY_NAME=SEMISOTNOV
AUTH_LOGIN=5700097
AUTH_TOKEN=c9f5f90185a7eae42557cf298188614144bcfcfd6ac806aa6fefb63c3e814456

# Настройки автоматической отправки
UPLOAD_TIME=09:00
UPLOAD_DAY=1
CSV_FILE_PATH=./report.csv