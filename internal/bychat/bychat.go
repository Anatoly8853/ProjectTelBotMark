package bychat

import (
	"ProjectTelBotMark/internal/structs"
	"errors"
	"fmt"
	slogavp "github.com/Anatoly8853/slog-avp/v2"

	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Orders Map для хранения данных по chatID.
var Orders = make(map[int64]structs.OrderDb)

// LogValidationError логирует ошибку и возвращает стандартный формат для валидаторов.
func LogValidationError(app *slogavp.Application, msg string, val interface{}) (string, string) {
	app.Log.Infof("%s %v", msg, val)
	return "", msg
}

// GetFieldByChatID для получения значения поля по chatID и имени поля.
func GetFieldByChatID(app *slogavp.Application, chatID int64, fieldName string) (interface{}, error) {

	if chatID == 0 {
		app.Log.Errorf("нет данных для обработки chatID = %v", chatID)
		return nil, errors.New("нет chatID")
	}

	order, exists := Orders[chatID]

	if !exists {
		app.Log.Errorf("заказ с таким chatID = %v не найден", chatID)
		return nil, errors.New("заказ с таким chatID не найден")
	}

	val := reflect.ValueOf(order)
	fieldVal := val.FieldByName(fieldName)

	if !fieldVal.IsValid() {
		app.Log.Error("такого поля не существует")
		return nil, errors.New("такого поля не существует")
	}

	return fieldVal.Interface(), nil
}

// FormatDateString очищает строку от лишних символов и проверяет корректность даты
func FormatDateString(app *slogavp.Application, dateStr string) (string, string) {
	// Убираем всё, кроме цифр и точки
	cleaned := regexp.MustCompile(`[^\d.]`).ReplaceAllString(dateStr, "")
	parts := strings.Split(cleaned, ".")
	if len(parts) != 3 {
		return LogValidationError(app, "неправильный формат даты", parts)
	}

	// Парсинг чисел с проверкой
	toInt := func(val string, msg string) (int, string) {
		num, err := strconv.Atoi(val)
		if err != nil {
			_, s := LogValidationError(app, msg, val)
			return 0, s
		}
		return num, ""
	}

	day, errMsg := toInt(parts[0], "неправильный формат дня")
	if errMsg != "" {
		return "", errMsg
	}
	month, errMsg := toInt(parts[1], "неправильный формат месяца")
	if errMsg != "" {
		return "", errMsg
	}
	year, errMsg := toInt(parts[2], "неправильный формат года")
	if errMsg != "" {
		return "", errMsg
	}

	formattedDate := fmt.Sprintf("%02d.%02d.%04d", day, month, year)

	// Проверка существования даты
	parsedDate, err := time.Parse("02.01.2006", formattedDate)
	if err != nil {
		return LogValidationError(app, "неправильный формат даты", formattedDate)
	}

	now := time.Now()

	// Год должен быть текущим
	if year != now.Year() {
		return LogValidationError(app, "год должен быть текущим", year)
	}

	// Месяц должен быть текущим или следующим
	if month != int(now.Month()) && month != int(now.Month())+1 {
		return LogValidationError(app, "месяц должен быть текущим или следующим", month)
	}

	// Дата должна быть в будущем (не сегодня)
	if !parsedDate.After(now) {
		return LogValidationError(app, "дата должна быть в будущем", parsedDate)
	}

	return formattedDate, ""
}

// FormatPhoneNumber проверяет и форматирует российский номер телефона в формате +7XXXXXXXXXX
func FormatPhoneNumber(app *slogavp.Application, phone string) (string, string) {
	// Очищаем от пробелов и символов, кроме цифр и +
	cleaned := regexp.MustCompile(`[^\d+]`).ReplaceAllString(phone, "")

	// Если начинается с 8 → заменяем на +7
	if strings.HasPrefix(cleaned, "8") {
		cleaned = "+7" + cleaned[1:]
	}
	// Если начинается с 7 (без плюса) → добавляем +
	if strings.HasPrefix(cleaned, "7") && !strings.HasPrefix(cleaned, "+") {
		cleaned = "+" + cleaned
	}

	// Проверка формата — должен быть +7XXXXXXXXXX (12 символов)
	if !strings.HasPrefix(cleaned, "+7") {
		return LogValidationError(app, "Номер должен начинаться с +7", cleaned)
	}
	if len(cleaned) != 12 {
		return LogValidationError(app, "Номер должен содержать 10 цифр после +7", cleaned)
	}
	if cleaned == "+79345678901" {
		return LogValidationError(app, "Это номер телефона для примера, введите свой номер", cleaned)
	}

	return cleaned, ""
}
