package main

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"time"
)

func logError(message string, err error) {
	if !EnableLogging {
		return
	}

	// Получаем текущее время и дату
	// Получаем текущее время и дату
	now := time.Now()

	// Получаем информацию о вызывающей функции
	_, file, line, ok := runtime.Caller(1)
	if ok {
		message = now.Format("15:04:05 02.01.2006") + " " + file + ":" + fmt.Sprint(line) + " " + message + ": " + err.Error()
	}

	// Определим путь к файлу лога
	logFilePath := "error.log"

	// Открываем файл для записи логов
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Printf("Error opening log file '%s': %v\n", logFilePath, err)
		return
	}
	defer logFile.Close()

	// Используем стандартный логгер для записи сообщения в файл
	logger := log.New(logFile, "", 0)
	logger.Println(message)
}
