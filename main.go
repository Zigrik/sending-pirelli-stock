package main

import (
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config структура для конфигурации
type Config struct {
	BaseURL       string
	CompanyName   string
	AuthLogin     string
	AuthToken     string
	ServerPort    string
	AdminPassword string
	UploadTime    string
	UploadDay     int
	CSVFilePath   string
}

var (
	config        Config
	serverRunning bool = true
)

func main() {
	// Загружаем конфигурацию из .env файла
	if err := loadConfig(); err != nil {
		log.Printf("Ошибка загрузки конфигурации: %v", err)
		log.Println("Продолжаем с настройками по умолчанию")
	}

	// Запускаем планировщик автоматической отправки
	if config.UploadTime != "" && config.UploadDay >= 0 && config.UploadDay <= 6 {
		go startScheduler()
	}

	// Настраиваем HTTP маршруты
	http.HandleFunc("/", handleWebForm)
	http.HandleFunc("/api/status", handleStatus)
	http.HandleFunc("/api/upload", handleUpload)
	http.HandleFunc("/api/web-upload", handleWebUpload)

	// Статические файлы
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// Запускаем сервер
	log.Printf("Сервер %s запущен на порту %s", config.CompanyName, config.ServerPort)
	log.Printf("Веб-форма доступна по: http://localhost:%s", config.ServerPort)
	log.Printf("Статус доступен по: http://localhost:%s/api/status", config.ServerPort)
	log.Printf("API загрузки: http://localhost:%s/api/upload", config.ServerPort)

	if config.UploadTime != "" {
		log.Printf("Автоматическая отправка: %s в день недели %d", config.UploadTime, config.UploadDay)
	}

	if err := http.ListenAndServe(":"+config.ServerPort, nil); err != nil {
		log.Fatalf("Ошибка запуска сервера: %v", err)
	}
}

// loadConfig загружает конфигурацию из .env файла
func loadConfig() error {
	// Пытаемся загрузить .env файл
	_ = godotenv.Load()

	// Устанавливаем значения по умолчанию
	config = Config{
		BaseURL:       getEnv("BASE_URL", "https://reports.pirelli.ru/local/templates/dealer/ajax/api.php"),
		CompanyName:   getEnv("COMPANY_NAME", "SEMISOTNOV"),
		AuthLogin:     getEnv("AUTH_LOGIN", "5700097"),
		AuthToken:     getEnv("AUTH_TOKEN", "c9f5f90185a7eae42557cf298188614144bcfcfd6ac806aa6fefb63c3e814456"),
		ServerPort:    getEnv("SERVER_PORT", "8080"),
		AdminPassword: getEnv("ADMIN_PASSWORD", "admin123"),
		UploadTime:    getEnv("UPLOAD_TIME", "09:00"),
		UploadDay:     getEnvInt("UPLOAD_DAY", 1),
		CSVFilePath:   getEnv("CSV_FILE_PATH", "./report.csv"),
	}

	return nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
