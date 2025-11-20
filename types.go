package main

import "time"

// PirelliResponse структура для ответа от PIRELLI
type PirelliResponse struct {
	Status  bool   `json:"status"`
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    []struct {
		DateTime     string `json:"datetime"`
		OriginalName string `json:"original_name"`
	} `json:"data"`
}

// ServerStatus структура для статуса сервера
type ServerStatus struct {
	Status     string    `json:"status"`
	Timestamp  time.Time `json:"timestamp"`
	Company    string    `json:"company"`
	Login      string    `json:"login"`
	NextUpload string    `json:"next_upload,omitempty"`
}

// UploadResult результат загрузки через веб-форму
type UploadResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}
