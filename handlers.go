package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
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

	// Проверяем содержимое файла на безопасность
	if err := validateCSVFile(file); err != nil {
		http.Error(w, "Файл содержит потенциально опасное содержимое: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Сбрасываем позицию чтения файла
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
	response, err := uploadFileToPirelli(tempFile.Name(), header.Filename)
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

	// Ограничиваем размер файла (10MB)
	r.ParseMultipartForm(10 << 20)

	// Проверяем пароль
	password := r.FormValue("password")
	if password != config.AdminPassword {
		sendWebResult(w, false, "Неверный пароль")
		return
	}

	// Получаем файл из формы
	file, header, err := r.FormFile("file")
	if err != nil {
		sendWebResult(w, false, "Ошибка чтения файла: "+err.Error())
		return
	}
	defer file.Close()

	// Проверяем расширение файла
	if !strings.HasSuffix(strings.ToLower(header.Filename), ".csv") {
		sendWebResult(w, false, "Можно загружать только CSV файлы")
		return
	}

	// Проверяем содержимое файла на безопасность
	if err := validateCSVFile(file); err != nil {
		sendWebResult(w, false, "Файл содержит потенциально опасное содержимое: "+err.Error())
		return
	}

	// Сбрасываем позицию чтения файла
	file.Seek(0, 0)

	// Сохраняем файл временно
	tempFile, err := os.CreateTemp("", "web-upload-*.csv")
	if err != nil {
		sendWebResult(w, false, "Ошибка создания временного файла")
		return
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	_, err = io.Copy(tempFile, file)
	if err != nil {
		sendWebResult(w, false, "Ошибка сохранения файла")
		return
	}

	// Отправляем файл в PIRELLI
	response, err := uploadFileToPirelli(tempFile.Name(), header.Filename)
	if err != nil {
		sendWebResult(w, false, "Ошибка отправки в PIRELLI: "+err.Error())
		return
	}

	// Формируем детали ответа
	details := ""
	if response.Status && len(response.Data) > 0 {
		lastUpload := response.Data[len(response.Data)-1]
		details = fmt.Sprintf("Файл загружен: %s (%s)", lastUpload.OriginalName, lastUpload.DateTime)
	}

	sendWebResult(w, response.Status, response.Message, details)
}
