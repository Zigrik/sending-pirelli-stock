package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
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

// validateCSVFile проверяет CSV файл на безопасность
func validateCSVFile(file io.Reader) error {
	// Создаем CSV reader
	reader := csv.NewReader(file)
	reader.Comma = ';' // или ',' в зависимости от формата
	reader.LazyQuotes = true

	// Читаем и проверяем первые несколько строк
	for i := 0; i < 100; i++ {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("ошибка чтения CSV: %v", err)
		}

		// Проверяем каждое поле в записи
		for _, field := range record {
			if containsMaliciousContent(field) {
				return fmt.Errorf("обнаружено потенциально опасное содержимое")
			}

			if len(field) > 10000 {
				return fmt.Errorf("поле слишком длинное")
			}
		}

		if len(record) > 100 {
			return fmt.Errorf("слишком много колонок в CSV")
		}
	}

	return nil
}

// containsMaliciousContent проверяет строку на наличие потенциально опасного содержимого
func containsMaliciousContent(s string) bool {
	lower := strings.ToLower(s)

	dangerousPatterns := []string{
		"<script", "javascript:", "vbscript:", "onload=", "onerror=", "onclick=",
		"eval(", "exec(", "union select", "drop table", "insert into", "<iframe",
		"<object", "<embed", "\\x00", "../",
	}

	for _, pattern := range dangerousPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}

	if len(s) > 1000 && strings.Count(s, s[0:1]) > len(s)*9/10 {
		return true
	}

	return false
}

// uploadFile отправляет файл в PIRELLI (для автоматической отправки)
func uploadFile(filePath string) error {
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

	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	_ = writer.WriteField("action", "upload")
	_ = writer.WriteField("auth_login", config.AuthLogin)
	_ = writer.WriteField("auth_token", config.AuthToken)

	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		return nil, fmt.Errorf("не удалось создать форму для файла: %v", err)
	}

	_, err = io.Copy(part, file)
	if err != nil {
		return nil, fmt.Errorf("не удалось скопировать содержимое файла: %v", err)
	}

	err = writer.Close()
	if err != nil {
		return nil, fmt.Errorf("ошибка при закрытии writer: %v", err)
	}

	req, err := http.NewRequest("POST", config.BaseURL, &requestBody)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания запроса: %v", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ошибка при выполнении запроса: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения ответа: %v", err)
	}

	var response PirelliResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, fmt.Errorf("ошибка парсинга JSON ответа: %v", err)
	}

	return &response, nil
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
