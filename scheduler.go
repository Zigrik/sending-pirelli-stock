package main

import (
	"log"
	"time"
)

// startScheduler запускает планировщик автоматической отправки
func startScheduler() {
	log.Printf("Планировщик запущен. Отправка в %s, день недели: %d", config.UploadTime, config.UploadDay)

	for {
		now := time.Now()

		// Парсим время отправки
		uploadTime, err := time.Parse("15:04", config.UploadTime)
		if err != nil {
			log.Printf("Ошибка парсинга времени: %v", err)
			time.Sleep(1 * time.Hour)
			continue
		}

		// Создаем время следующей отправки
		nextUpload := time.Date(now.Year(), now.Month(), now.Day(),
			uploadTime.Hour(), uploadTime.Minute(), 0, 0, now.Location())

		// Если время уже прошло сегодня, планируем на следующий день
		if now.After(nextUpload) || (now.Weekday() != time.Weekday(config.UploadDay) && config.UploadDay >= 0) {
			daysToAdd := (config.UploadDay - int(now.Weekday()) + 7) % 7
			if daysToAdd == 0 && now.After(nextUpload) {
				daysToAdd = 7
			}
			nextUpload = nextUpload.AddDate(0, 0, daysToAdd)
		}

		// Ждем до времени отправки
		duration := nextUpload.Sub(now)
		log.Printf("Следующая автоматическая отправка: %s", nextUpload.Format("2006-01-02 15:04:05"))

		time.Sleep(duration)

		// Выполняем отправку
		log.Println("Выполняется автоматическая отправка отчета...")
		if err := uploadFile(config.CSVFilePath); err != nil {
			log.Printf("Ошибка автоматической отправки: %v", err)
		}
	}
}
