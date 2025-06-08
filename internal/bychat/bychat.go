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

// FormatDateString для очистки строки от пробелов и знаков препинания, а также форматирования даты.
func FormatDateString(app *slogavp.Application, dateStr string) (string, string) {

	// Удаляем все пробелы и знаки препинания, кроме точки
	re := regexp.MustCompile(`[^\d.]`)
	cleaned := re.ReplaceAllString(dateStr, "")

	// Разделяем строку на день, месяц и год
	parts := strings.Split(cleaned, ".")
	if len(parts) != 3 {

		err := fmt.Errorf("")
		app.Log.Infof("неправильный формат даты %v ошибка %v", parts, err)
		return "", "неправильный формат даты"

	}

	// Приводим день и месяц к формату с ведущим нулем
	day, err := strconv.Atoi(parts[0])
	if err != nil {

		app.Log.Infof("неправильный формат дня %v ошибка %v", day, err)
		return "", "неправильный формат дня"

	}

	month, err := strconv.Atoi(parts[1])
	if err != nil {

		app.Log.Infof("неправильный формат месяца %v ошибка %v", month, err)
		return "", "неправильный формат месяца"

	}

	year, err := strconv.Atoi(parts[2])
	if err != nil {

		app.Log.Infof("неправильный формат года %v ошибка %v", year, err)
		return "", "неправильный формат года"

	}

	// Форматируем день и месяц с ведущими нулями
	formattedDate := fmt.Sprintf("%02d.%02d.%04d", day, month, year)

	// Проверяем правильность формата даты
	parsedDate, err := time.Parse("02.01.2006", formattedDate)
	if err != nil {

		app.Log.Infof("неправильный формат даты %v ошибка %v", formattedDate, err)
		return "", "неправильный формат даты"

	}

	// Получаем текущую дату
	currentTime := time.Now()
	currentYear := currentTime.Year()
	currentMonth := int(currentTime.Month())

	// Проверка, что год совпадает с текущим годом
	if year != currentYear {

		err = fmt.Errorf("год должен быть текущим")
		app.Log.Infof("год %v ошибка %v", year, err)
		return "", "год должен быть текущим"

	}

	// Проверка, что месяц текущий или следующий
	if month != currentMonth && month != currentMonth+1 {

		err = fmt.Errorf("месяц должен быть текущим или следующим")
		app.Log.Infof("месяц %v ошибка %v", month, err)
		return "", "месяц должен быть текущим или следующим"

	}

	// Проверка, что дата не прошедшая и не сегодняшняя
	if parsedDate.Before(currentTime) || parsedDate.Equal(currentTime) {

		err = fmt.Errorf("дата не должна быть прошедшей или сегодняшней")
		app.Log.Infof("дата %v ошибка %v", currentTime, err)
		return "", "дата не должна быть прошедшей или сегодняшней"

	}

	return formattedDate, ""
}

// FormatPhoneNumber Функция для проверки и форматирования номера телефона
func FormatPhoneNumber(app *slogavp.Application, phone string) (string, string) {

	// Удаляем все пробелы и знаки препинания
	re := regexp.MustCompile(`[^\d+]`)
	cleaned := re.ReplaceAllString(phone, "")

	// Проверка и добавление + перед 7, если нужно
	if strings.HasPrefix(cleaned, "7") && !strings.HasPrefix(cleaned, "+") {

		cleaned = "+" + cleaned

	}

	// Замена 8 на +7
	if strings.HasPrefix(cleaned, "8") {

		cleaned = "+7" + cleaned[1:]

	}

	// Проверка формата номера
	if strings.HasPrefix(cleaned, "+7") {

		if len(cleaned) != 12 {

			err := fmt.Errorf("номер должен содержать 10 цифр после +7 ")
			app.Log.Infof("номер телефона %v ошибка %v", cleaned, err)
			return "", "номер должен содержать 10 цифр после +7"

		}

	} else {

		err := fmt.Errorf("номер должен начинаться с +7 ")
		app.Log.Infof("номер телефона %v ошибка %v", cleaned, err)
		return "", "номер должен начинаться с +7"

	}

	if cleaned == "+79345678901" {

		err := fmt.Errorf("это номер телефона для примера введите свой номер телефона ")
		app.Log.Infof("%v", err)
		return "", "это номер телефона для примера введите свой номер телефона"

	}

	return cleaned, ""
}
