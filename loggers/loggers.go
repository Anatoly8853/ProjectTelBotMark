package loggers

import (
	"fmt"
	"github.com/gookit/slog"
	"github.com/gookit/slog/handler"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// глобальные переменные
const (
	IsDebugMode = true // Переключатель режима отладки
	IsInfoMode  = true
	IsWarnMode  = true
)

var logger *slog.Logger // Глобальный логгер

// CustomFormatter - кастомный формат для изменения формата даты
type CustomFormatter struct{}

// Format реализует метод интерфейса slog.Formatter
func (f *CustomFormatter) Format(record *slog.Record) ([]byte, error) {
	formattedTime := record.Time.Format("15:04:05 02.01.2006")
	caller := record.Caller
	fileName := filepath.Base(caller.File) // Получаем только имя файла
	funcName := getFunctionName(caller.PC) // Получаем имя функции
	logMessage := fmt.Sprintf("[%s] [application] [%s] [%s:%d,%s] %s\n", formattedTime, record.Level.String(), fileName, caller.Line, funcName, record.Message)
	return []byte(logMessage), nil
}

// getFunctionName возвращает короткое имя функции по Program Counter (PC)
func getFunctionName(pc uintptr) string {
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return "unknown"
	}

	// Извлекаем только короткое имя функции
	fullName := fn.Name()
	parts := strings.Split(fullName, "/")
	shortName := parts[len(parts)-1]

	if dotIndex := strings.LastIndex(shortName, "."); dotIndex != -1 {
		shortName = shortName[dotIndex+1:]
	}

	return shortName
}

// SetupLogger настраивает и возвращает логгер
func SetupLogger() *slog.Logger {
	// Получаем текущее время и дату
	now := time.Now()

	// Определим путь к файлу лога с учетом текущей даты в формате DD-MM-YYYY
	logFilePath := fmt.Sprintf("log/error-%s.log", now.Format("02-01-2006"))

	// Создаем директорию для логов, если её нет
	err := os.MkdirAll(filepath.Dir(logFilePath), 0755)
	if err != nil {
		panic(fmt.Sprintf("Error creating log directory: %v", err))
	}

	// Открываем файл для записи логов
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic(fmt.Sprintf("Error opening log file: %v", err))
	}

	// Создаем хэндлер для записи в файл с кастомным форматтером
	//fileHandler := handler.NewIOWriterHandler(logFile, slog.AllLevels)
	fileHandler := handler.NewIOWriterHandler(logFile, getLogLevels())
	fileHandler.SetFormatter(&CustomFormatter{})

	// Создаем логгер и добавляем хэндлер
	logger = slog.New()
	logger.AddHandler(fileHandler)

	return logger
}

func getLogLevels() []slog.Level {
	levels := []slog.Level{slog.ErrorLevel, slog.FatalLevel}

	if IsWarnMode {
		levels = append(levels, slog.WarnLevel)
	}
	if IsInfoMode {
		levels = append(levels, slog.InfoLevel)
	}
	if IsDebugMode {
		levels = append(levels, slog.DebugLevel)
	}

	return levels
}
