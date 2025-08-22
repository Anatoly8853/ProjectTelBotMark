package botTelegram

import (
	"ProjectTelBotMark/internal/dataSources"
	"ProjectTelBotMark/internal/structs"
	"errors"
	"fmt"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// CreateCategoryKeyboardReply строит клавиатуру с учётом пагинации и возврата
func CreateCategoryKeyboardReply(categoryName string) ([][]tgbotapi.KeyboardButton, error) {
	// Получаем данные для выбранной категории
	cat := dataSources.StripEmojis(categoryName)
	categoryData, found := structs.Categories[cat]
	if !found {
		return nil, errors.New("категория не найдена в CreateCategoryKeyboardReply")
	}

	// Создаем первый ряд кнопок с кнопками категории
	var keyboard [][]tgbotapi.KeyboardButton
	row := []tgbotapi.KeyboardButton{}
	for _, button := range categoryData.Buttons {
		btn := tgbotapi.NewKeyboardButton(button.Text)
		row = append(row, btn)
		if len(row) == 3 {
			// Если в ряду уже три кнопки, добавляем его к клавиатуре и создаем новый ряд
			keyboard = append(keyboard, row)
			row = []tgbotapi.KeyboardButton{}
		}
	}
	// Если остались кнопки, добавляем последний ряд
	if len(row) > 0 {
		keyboard = append(keyboard, row)
	}

	secondRow := []tgbotapi.KeyboardButton{}
	// Создаем второй ряд с кнопкой "Назад" и корзина
	for _, buttonM := range categoryData.ButtonsM {
		btnM := tgbotapi.NewKeyboardButton(buttonM.Text)
		secondRow = append(secondRow, btnM)
	}

	keyboard = append(keyboard, secondRow)

	return keyboard, nil
}

// CreateCategoryKeyboard Массив кнопки фасовки продукта InlineKeyboardButtonData.
func CreateCategoryKeyboard(categoryName string) ([][]tgbotapi.InlineKeyboardButton, error) {
	// Получаем данные для выбранной категории
	categoryData, found := structs.Categories[categoryName]
	if !found {
		return nil, errors.New("категория не найдена в createCategoryKeyboard")
	}

	// Создаем первый ряд кнопок с кнопками категории
	var inlineKeyboard [][]tgbotapi.InlineKeyboardButton
	row := []tgbotapi.InlineKeyboardButton{}
	for _, button := range categoryData.Buttons {
		btn := tgbotapi.NewInlineKeyboardButtonData(button.Text, button.Callback)
		row = append(row, btn)
		if len(row) == 3 {
			// Если в ряду уже три кнопки, добавляем его к клавиатуре и создаем новый ряд
			inlineKeyboard = append(inlineKeyboard, row)
			row = []tgbotapi.InlineKeyboardButton{}
		}
	}
	// Если остались кнопки, добавляем последний ряд
	if len(row) > 0 {
		inlineKeyboard = append(inlineKeyboard, row)
	}

	return inlineKeyboard, nil
}

// createCalendarKeyboard календать клавиатура.
func (m *MessageService) createCalendarKeyboard(month time.Time) tgbotapi.InlineKeyboardMarkup {
	currentYear, currentMonth, _ := month.Date()
	currentLocation := month.Location()

	// Первый день месяца
	firstOfMonth := time.Date(currentYear, currentMonth, 1, 0, 0, 0, 0, currentLocation)
	// Последний день месяца
	lastOfMonth := firstOfMonth.AddDate(0, 1, -1)

	var buttons [][]tgbotapi.InlineKeyboardButton
	var row []tgbotapi.InlineKeyboardButton

	// Заголовок с названием месяца
	monthYear := month.Format("01")
	monthName := m.getMonthName(monthYear)
	row = append(row, tgbotapi.NewInlineKeyboardButtonData(monthName, "ignore"))
	buttons = append(buttons, row)

	row = []tgbotapi.InlineKeyboardButton{}

	// Заголовок дней недели
	daysOfWeek := []string{"Пн", "Вт", "Ср", "Чт", "Пт", "Сб", "Вс"}
	for _, day := range daysOfWeek {
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(day, "ignore"))
	}
	buttons = append(buttons, row)

	row = []tgbotapi.InlineKeyboardButton{}

	// Добавить пустые кнопки до первого дня месяца
	for i := 1; i < int(firstOfMonth.Weekday()); i++ {
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(" ", "ignore"))
	}
	if firstOfMonth.Weekday() == time.Sunday {
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(" ", "ignore"))
	}

	for day := 1; day <= lastOfMonth.Day(); day++ {
		date := time.Date(currentYear, currentMonth, day, 0, 0, 0, 0, currentLocation)
		dateStr := date.Format("02.01.2006")
		callbackData := fmt.Sprintf("deliverydate_%s", dateStr)
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("%02d", day), callbackData))

		if date.Weekday() == time.Sunday {
			buttons = append(buttons, row)
			row = []tgbotapi.InlineKeyboardButton{}
		}
	}

	// Добавить оставшиеся кнопки
	if len(row) > 0 {
		buttons = append(buttons, row)
	}

	// Кнопки для перехода между месяцами
	row = []tgbotapi.InlineKeyboardButton{}
	currentDate := time.Now()
	if currentYear > currentDate.Year() || (currentYear == currentDate.Year() && currentMonth > currentDate.Month()) {
		prevMonth := month.AddDate(0, -1, 0)
		prevMonthStr := prevMonth.Format("01-2006")
		row = append(row, tgbotapi.NewInlineKeyboardButtonData("⬅️", "nextmonth_"+prevMonthStr))
	}

	nextMonth := month.AddDate(0, 1, 0)
	nextMonthStr := nextMonth.Format("01-2006")
	row = append(row, tgbotapi.NewInlineKeyboardButtonData("➡️", "nextmonth_"+nextMonthStr))
	buttons = append(buttons, row)

	return tgbotapi.InlineKeyboardMarkup{
		InlineKeyboard: buttons,
	}
}

func (m *MessageService) getMonthName(monthStr string) string {
	monthMap := map[string]string{
		"01": "Январь",
		"02": "Февраль",
		"03": "Март",
		"04": "Апрель",
		"05": "Май",
		"06": "Июнь",
		"07": "Июль",
		"08": "Август",
		"09": "Сентябрь",
		"10": "Октябрь",
		"11": "Ноябрь",
		"12": "Декабрь",
	}

	monthName, exists := monthMap[monthStr]
	if !exists {
		m.app.Log.Errorf("invalid month format: %s", monthStr)
		return ""
	}

	return monthName
}
