package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// calculateNextUploadTime вычисляет время следующей автоматической отправки
func calculateNextUploadTime() string {
	now := time.Now()
	uploadTime, err := time.Parse("15:04", config.UploadTime)
	if err != nil {
		return ""
	}

	nextUpload := time.Date(now.Year(), now.Month(), now.Day(),
		uploadTime.Hour(), uploadTime.Minute(), 0, 0, now.Location())

	if now.After(nextUpload) || (now.Weekday() != time.Weekday(config.UploadDay) && config.UploadDay >= 0) {
		daysToAdd := (config.UploadDay - int(now.Weekday()) + 7) % 7
		if daysToAdd == 0 && now.After(nextUpload) {
			daysToAdd = 7
		}
		nextUpload = nextUpload.AddDate(0, 0, daysToAdd)
	}

	return nextUpload.Format("2006-01-02 15:04:05")
}

// sendWebResult отправляет результат веб-загрузки
func sendWebResult(w http.ResponseWriter, success bool, message string, details ...string) {
	result := UploadResult{
		Success: success,
		Message: message,
	}

	if len(details) > 0 {
		result.Details = details[0]
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// validateCSVFile с логированием для отладки
func validateCSVFile(file io.Reader) error {
	content, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("ошибка чтения файла: %v", err)
	}

	log.Printf("Размер файла: %d байт", len(content))

	// Проверяем размер (10MB максимум)
	if len(content) > 10*1024*1024 {
		return fmt.Errorf("файл слишком большой (максимум 10MB)")
	}

	// Проверяем что не пустой
	if len(content) == 0 {
		return fmt.Errorf("файл пустой")
	}

	// Логируем первые 100 символов для отладки
	if len(content) > 100 {
		log.Printf("Первые 100 символов файла: %s", string(content[:100]))
	} else {
		log.Printf("Содержимое файла: %s", string(content))
	}

	// Проверяем на вредоносный код
	if containsMaliciousContent(string(content)) {
		return fmt.Errorf("обнаружено потенциально опасное содержимое")
	}

	log.Printf("Файл прошел проверку безопасности")
	return nil
}

// validateCSVFileFromPath проверяет CSV файл по пути
func validateCSVFileFromPath(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("не удалось открыть файл: %v", err)
	}
	defer file.Close()

	return validateCSVFile(file)
}

// containsMaliciousContent проверяет на опасный код для Linux
func containsMaliciousContent(s string) bool {
	lower := strings.ToLower(s)

	// Опасные паттерны для Linux сервера
	dangerousPatterns := []string{
		// Shell injection
		"$((", "`", "&&", "||", "|", ">", "<", ";",
		// Command execution
		"/bin/bash", "/bin/sh", "bash -c", "sh -c", "eval ", "exec(",
		// System commands
		"rm -rf", "rm -f", "chmod", "chown", "sudo", "su ",
		"wget", "curl", "nc ", "netcat", "ssh ", "scp ",
		// File system access
		"/etc/passwd", "/etc/shadow", "/etc/hosts", "/proc/",
		"../../", "../etc/", "/root/", "/home/",
		// Network
		"127.0.0.1", "localhost", "0.0.0.0",
		// Code injection
		"<script", "javascript:", "vbscript:", "onload=", "onerror=",
		"<iframe", "<object", "<embed",
		// SQL injection (базовые)
		"union select", "drop table", "insert into", "delete from",
		"update set", "create table", "alter table",
		// PHP injection
		"<?php", "<?=", "system(", "shell_exec(", "exec(",
		"passthru(", "proc_open", "popen(",
	}

	for _, pattern := range dangerousPatterns {
		if strings.Contains(lower, pattern) {
			log.Printf("Обнаружен опасный паттерн: %s", pattern)
			return true
		}
	}

	// Проверка на бинарные файлы (первые байты)
	if len(s) > 4 {
		// ELF binary
		if s[0] == 0x7f && s[1] == 'E' && s[2] == 'L' && s[3] == 'F' {
			return true
		}
		// PE executable (Windows)
		if s[0] == 'M' && s[1] == 'Z' {
			return true
		}
	}

	return false
}

// uploadFile отправляет файл в PIRELLI (для автоматической отправки)
func uploadFile(filePath string) error {
	// Сначала проверяем файл
	if err := validateCSVFileFromPath(filePath); err != nil {
		return fmt.Errorf("ошибка проверки файла: %v", err)
	}

	response, err := uploadFileToPirelli(filePath, filepath.Base(filePath))
	if err != nil {
		return err
	}

	log.Printf("Отправка успешна! Статус: %t, Сообщение: %s", response.Status, response.Message)
	if response.Status && len(response.Data) > 0 {
		lastUpload := response.Data[len(response.Data)-1]
		log.Printf("Последняя загрузка: %s (%s)", lastUpload.OriginalName, lastUpload.DateTime)
	}

	return nil
}

// uploadFileToPirelli отправляет файл на сервер PIRELLI
func uploadFileToPirelli(filePath, fileName string) (*PirelliResponse, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("не удалось открыть файл: %v", err)
	}
	defer file.Close()

	// Создаем буфер для multipart формы
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	// Устанавливаем ТОЧНЫЙ boundary как в 1С
	boundary := "----WebKitFormBoundary7MA4YWxkTrZu0gW"
	writer.SetBoundary(boundary)

	// Добавляем поля формы в ТОЧНОМ порядке как в примере
	fields := []struct {
		name  string
		value string
	}{
		{"action", "upload"},
		{"auth_login", config.AuthLogin},
		{"auth_token", config.AuthToken},
	}

	for _, field := range fields {
		err = writer.WriteField(field.name, field.value)
		if err != nil {
			return nil, fmt.Errorf("ошибка добавления %s: %v", field.name, err)
		}
	}

	// Создаем заголовок для файла с правильным Content-Type
	headers := make(textproto.MIMEHeader)
	headers.Set("Content-Disposition",
		fmt.Sprintf(`form-data; name="file"; filename="%s"`, fileName))
	headers.Set("Content-Type", "text/csv")

	// Создаем часть для файла
	part, err := writer.CreatePart(headers)
	if err != nil {
		return nil, fmt.Errorf("не удалось создать часть для файла: %v", err)
	}

	// Копируем содержимое файла
	_, err = io.Copy(part, file)
	if err != nil {
		return nil, fmt.Errorf("не удалось скопировать содержимое файла: %v", err)
	}

	// Закрываем writer для завершения формы
	err = writer.Close()
	if err != nil {
		return nil, fmt.Errorf("ошибка при закрытии writer: %v", err)
	}

	// Логируем первые 500 байт тела для отладки
	bodyPreview := requestBody.Bytes()
	previewLen := min(500, len(bodyPreview))
	log.Printf("Размер тела запроса: %d байт", len(bodyPreview))
	log.Printf("Первые %d байт тела: %s", previewLen, string(bodyPreview[:previewLen]))

	// Создаем HTTP запрос
	req, err := http.NewRequest("POST", config.BaseURL, &requestBody)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания запроса: %v", err)
	}

	// Устанавливаем Content-Type с boundary
	contentType := writer.FormDataContentType()
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("User-Agent", "Mozilla/5.0")

	// Выполняем запрос
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	log.Printf("Выполняем запрос к %s", config.BaseURL)
	log.Printf("Content-Type: %s", contentType)
	log.Printf("Имя файла: %s", fileName)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ошибка при выполнении запроса: %v", err)
	}
	defer resp.Body.Close()

	// Читаем ответ
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения ответа: %v", err)
	}

	log.Printf("Ответ от PIRELLI: статус %d, тело: %s", resp.StatusCode, string(body))

	// Парсим JSON ответ
	var response PirelliResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, fmt.Errorf("ошибка парсинга JSON ответа: %v", err)
	}

	return &response, nil
}

// generatePirelliFilename генерирует имя файла по формату PIRELLI
func generatePirelliFilename() string {
	now := time.Now()
	return fmt.Sprintf("ir_%s_%s.csv", config.AuthLogin, now.Format("20060102_150405"))
}

// embeddedFormTemplate возвращает встроенный HTML шаблон на случай отсутствия файла
func embeddedFormTemplate() string {
	// Простой fallback шаблон если файл form.html не найден
	return `<!DOCTYPE html>
<html>
<head><title>Загрузка отчетов</title></head>
<body>
	<h1>Загрузка отчетов {{.CompanyName}}</h1>
	<p>Файл шаблона не найден. Пожалуйста, создайте templates/form.html</p>
</body>
</html>`
}
