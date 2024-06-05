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

var (
	IsDebugMode = true // Переключатель режима отладки
	IsInfoMode  = true
	IsWarnMode  = true

	logger *slog.Logger // Глобальный логгер
)

// CustomFormatter - кастомный формат для изменения формата даты
type CustomFormatter struct{}

// Format реализует метод интерфейса slog.Formatter
func (f *CustomFormatter) Format(record *slog.Record) ([]byte, error) {
	formattedTime := record.Time.Format("15:04:05 02.01.2024")
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

	// Открываем файл для записи логов
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}

	// Создаем хэндлер для записи в файл с кастомным форматтером
	fileHandler := handler.NewIOWriterHandler(logFile, getLogLevels())
	fileHandler.SetFormatter(&CustomFormatter{})

	// Создаем логгер и добавляем хэндлер
	logger = slog.New()
	logger.AddHandler(fileHandler)

	return logger
}

// getLogLevels возвращает уровни логирования в зависимости от режима
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
