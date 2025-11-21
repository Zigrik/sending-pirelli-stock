package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// handleWebForm отображает веб-форму для загрузки файлов
func handleWebForm(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		// Читаем HTML шаблон из файла
		htmlContent, err := os.ReadFile("templates/form.html")
		if err != nil {
			// Если файл не найден, используем встроенный шаблон
			htmlContent = []byte(embeddedFormTemplate())
		}

		tmplData := struct {
			CompanyName string
		}{
			CompanyName: config.CompanyName,
		}

		t, err := template.New("webform").Parse(string(htmlContent))
		if err != nil {
			http.Error(w, "Ошибка шаблона: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		t.Execute(w, tmplData)
	}
}

// handleStatus обрабатывает запрос статуса сервера
func handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	status := "running"
	if !serverRunning {
		status = "stopped"
	}

	nextUpload := ""
	if config.UploadTime != "" {
		nextUpload = calculateNextUploadTime()
	}

	response := ServerStatus{
		Status:     status,
		Timestamp:  time.Now(),
		Company:    config.CompanyName,
		Login:      config.AuthLogin,
		NextUpload: nextUpload,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleUpload обрабатывает загрузку файлов через API (только POST)
func handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	// Проверяем пароль из заголовка или формы
	password := r.Header.Get("X-Admin-Password")
	if password == "" {
		password = r.FormValue("password")
	}

	if password != config.AdminPassword {
		http.Error(w, "Неверный пароль", http.StatusUnauthorized)
		return
	}

	// Получаем файл из формы
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Ошибка чтения файла: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Проверяем расширение файла
	if !strings.HasSuffix(strings.ToLower(header.Filename), ".csv") {
		http.Error(w, "Можно загружать только CSV файлы", http.StatusBadRequest)
		return
	}

	// Сбрасываем позицию чтения файла на начало
	file.Seek(0, 0)

	// Проверяем содержимое файла на безопасность
	if err := validateCSVFile(file); err != nil {
		http.Error(w, "Файл содержит потенциально опасное содержимое: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Снова сбрасываем позицию чтения для сохранения
	file.Seek(0, 0)

	// Сохраняем файл временно
	tempFile, err := os.CreateTemp("", "upload-*.csv")
	if err != nil {
		http.Error(w, "Ошибка создания временного файла", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	_, err = io.Copy(tempFile, file)
	if err != nil {
		http.Error(w, "Ошибка сохранения файла", http.StatusInternalServerError)
		return
	}

	// Отправляем файл в PIRELLI
	filename := generatePirelliFilename()
	response, err := uploadFileToPirelli(tempFile.Name(), filename)

	if err != nil {
		http.Error(w, "Ошибка отправки в PIRELLI: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleWebUpload обрабатывает загрузку файлов через веб-форму
func handleWebUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	log.Println("Начало обработки загрузки файла через веб-форму")

	// Ограничиваем размер файла (10MB)
	r.ParseMultipartForm(10 << 20)

	// Проверяем пароль
	password := r.FormValue("password")
	if password != config.AdminPassword {
		log.Println("Неверный пароль")
		sendWebResult(w, false, "Неверный пароль")
		return
	}

	// Получаем файл из формы
	file, header, err := r.FormFile("file")
	if err != nil {
		log.Printf("Ошибка чтения файла: %v", err)
		sendWebResult(w, false, "Ошибка чтения файла: "+err.Error())
		return
	}
	defer file.Close()

	log.Printf("Получен файл: %s, размер: %d", header.Filename, header.Size)

	// Проверяем расширение файла
	if !strings.HasSuffix(strings.ToLower(header.Filename), ".csv") {
		log.Println("Неверное расширение файла")
		sendWebResult(w, false, "Можно загружать только CSV файлы")
		return
	}

	// Сбрасываем позицию чтения файла на начало
	file.Seek(0, 0)

	// Проверяем содержимое файла на безопасность
	if err := validateCSVFile(file); err != nil {
		log.Printf("Файл не прошел проверку безопасности: %v", err)
		sendWebResult(w, false, "Файл не прошел проверку безопасности: "+err.Error())
		return
	}

	// Снова сбрасываем позицию чтения для сохранения
	file.Seek(0, 0)

	// Создаем временный файл для отправки
	tempFile, err := os.CreateTemp("", "web-upload-*.csv")
	if err != nil {
		log.Printf("Ошибка создания временного файла: %v", err)
		sendWebResult(w, false, "Ошибка создания временного файла")
		return
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	_, err = io.Copy(tempFile, file)
	if err != nil {
		log.Printf("Ошибка сохранения файла: %v", err)
		sendWebResult(w, false, "Ошибка сохранения файла")
		return
	}

	log.Printf("Временный файл создан: %s", tempFile.Name())

	// Отправляем файл в PIRELLI
	log.Println("Начало отправки файла в PIRELLI")
	filename := generatePirelliFilename()
	response, err := uploadFileToPirelli(tempFile.Name(), filename)
	if err != nil {
		log.Printf("Ошибка отправки в PIRELLI: %v", err)
		sendWebResult(w, false, "Ошибка отправки в PIRELLI: "+err.Error())
		return
	}

	log.Printf("Ответ от PIRELLI: статус=%t, код=%d, сообщение=%s", response.Status, response.Code, response.Message)

	// Формируем детали ответа
	details := ""
	if response.Status && len(response.Data) > 0 {
		lastUpload := response.Data[len(response.Data)-1]
		details = fmt.Sprintf("Файл загружен: %s (%s)", lastUpload.OriginalName, lastUpload.DateTime)
	}

	sendWebResult(w, response.Status, response.Message, details)
}
