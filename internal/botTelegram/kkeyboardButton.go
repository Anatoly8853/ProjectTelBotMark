package botTelegram

import (
	"ProjectTelBotMark/internal/dataSources"
	"ProjectTelBotMark/internal/structs"
	"errors"
	"fmt"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// createStartKeyboard возвращает главное меню в виде inline-клавиатуры.
func createStartKeyboard() tgbotapi.InlineKeyboardMarkup {
	button := tgbotapi.NewInlineKeyboardButtonData("Старт", "cat_Старт")
	return tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(button))
}

// CreateCategoryKeyboardReply строит клавиатуру с учётом пагинации и возврата
func CreateCategoryKeyboardReply(categoryName string) ([][]tgbotapi.InlineKeyboardButton, error) {
	// Получаем данные для выбранной категории
	cat := dataSources.StripEmojis(categoryName)
	categoryData, found := structs.GetCategory(cat)
	if !found {
		return nil, errors.New("категория не найдена в CreateCategoryKeyboardReply")
	}

	// Создаем первый ряд кнопок с кнопками категории
	var keyboard [][]tgbotapi.InlineKeyboardButton
	row := []tgbotapi.InlineKeyboardButton{}
	for _, button := range categoryData.Buttons {
		btn := tgbotapi.NewInlineKeyboardButtonData(button.Text, button.Callback)
		row = append(row, btn)
		if len(row) == 3 {
			// Если в ряду уже три кнопки, добавляем его к клавиатуре и создаем новый ряд
			keyboard = append(keyboard, row)
			row = []tgbotapi.InlineKeyboardButton{}
		}
	}
	// Если остались кнопки, добавляем последний ряд
	if len(row) > 0 {
		keyboard = append(keyboard, row)
	}

	favoriteRow := []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("⭐ Избранное", "cat_Избранное"),
	}
	keyboard = append(keyboard, favoriteRow)

	secondRow := []tgbotapi.InlineKeyboardButton{}
	// Создаем второй ряд с кнопкой "Назад" и корзина
	for _, buttonM := range categoryData.ButtonsM {
		btnM := tgbotapi.NewInlineKeyboardButtonData(buttonM.Text, buttonM.Callback)
		secondRow = append(secondRow, btnM)
	}

	keyboard = append(keyboard, secondRow)

	return keyboard, nil
}

// CreateCategoryKeyboard строит кнопки фасовки, избранного и навигации.
func CreateCategoryKeyboard(categoryName string) ([][]tgbotapi.InlineKeyboardButton, error) {
	categoryData, found := structs.GetCategory(categoryName)
	if !found {
		return nil, errors.New("категория не найдена в createCategoryKeyboard")
	}

	var keyboard [][]tgbotapi.InlineKeyboardButton
	row := make([]tgbotapi.InlineKeyboardButton, 0, 4)
	for _, button := range categoryData.Buttons {
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(button.Text, button.Callback))
		if button.FavoriteCallback != "" {
			row = append(row, tgbotapi.NewInlineKeyboardButtonData("⭐", button.FavoriteCallback))
		}
		if len(row) >= 4 {
			keyboard = append(keyboard, row)
			row = make([]tgbotapi.InlineKeyboardButton, 0, 4)
		}
	}
	if len(row) > 0 {
		keyboard = append(keyboard, row)
	}

	keyboard = append(keyboard, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("⭐ Избранное", "cat_Избранное"),
	))
	if len(categoryData.ButtonsM) > 0 {
		bottom := make([]tgbotapi.InlineKeyboardButton, 0, len(categoryData.ButtonsM))
		for _, button := range categoryData.ButtonsM {
			bottom = append(bottom, tgbotapi.NewInlineKeyboardButtonData(button.Text, button.Callback))
		}
		keyboard = append(keyboard, bottom)
	}
	return keyboard, nil
}

// createCalendarKeyboard строит календарь доставки с быстрым выбором ближайших дат.
func (m *MessageService) createCalendarKeyboard(month time.Time) tgbotapi.InlineKeyboardMarkup {
	currentYear, currentMonth, _ := month.Date()
	currentLocation := month.Location()
	firstOfMonth := time.Date(currentYear, currentMonth, 1, 0, 0, 0, 0, currentLocation)
	lastOfMonth := firstOfMonth.AddDate(0, 1, -1)
	today := time.Now().In(currentLocation)
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, currentLocation)

	var buttons [][]tgbotapi.InlineKeyboardButton

	// Три быстрых варианта всегда расположены над календарём.
	quickRow := make([]tgbotapi.InlineKeyboardButton, 0, 3)
	quickLabels := []string{"Завтра", "Через 2 дня", "Через 3 дня"}
	for i, label := range quickLabels {
		date := today.AddDate(0, 0, i+1)
		quickRow = append(quickRow, tgbotapi.NewInlineKeyboardButtonData(label, "deliverydate_"+date.Format("02.01.2006")))
	}
	buttons = append(buttons, quickRow)

	monthName := m.getMonthName(month.Format("01"))
	buttons = append(buttons, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("%s %d", monthName, currentYear), "ignore"),
	))

	// Дни недели оставлены в прежнем порядке: понедельник — воскресенье.
	weekdays := []string{"Пн", "Вт", "Ср", "Чт", "Пт", "Сб", "Вс"}
	weekdayRow := make([]tgbotapi.InlineKeyboardButton, 0, len(weekdays))
	for _, day := range weekdays {
		weekdayRow = append(weekdayRow, tgbotapi.NewInlineKeyboardButtonData(day, "ignore"))
	}
	buttons = append(buttons, weekdayRow)

	row := make([]tgbotapi.InlineKeyboardButton, 0, 7)
	offset := (int(firstOfMonth.Weekday()) + 6) % 7 // переводим Sunday=0 в Monday-first
	for i := 0; i < offset; i++ {
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(" ", "ignore"))
	}

	for day := 1; day <= lastOfMonth.Day(); day++ {
		date := time.Date(currentYear, currentMonth, day, 0, 0, 0, 0, currentLocation)
		if date.After(today) {
			row = append(row, tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("%02d", day), "deliverydate_"+date.Format("02.01.2006"),
			))
		} else {
			row = append(row, tgbotapi.NewInlineKeyboardButtonData("·", "ignore"))
		}
		if len(row) == 7 {
			buttons = append(buttons, row)
			row = make([]tgbotapi.InlineKeyboardButton, 0, 7)
		}
	}
	if len(row) > 0 {
		for len(row) < 7 {
			row = append(row, tgbotapi.NewInlineKeyboardButtonData(" ", "ignore"))
		}
		buttons = append(buttons, row)
	}

	// Можно листать месяцы вперёд; назад — только до текущего месяца.
	navigation := make([]tgbotapi.InlineKeyboardButton, 0, 2)
	currentMonthStart := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, currentLocation)
	if firstOfMonth.After(currentMonthStart) {
		previous := firstOfMonth.AddDate(0, -1, 0)
		navigation = append(navigation, tgbotapi.NewInlineKeyboardButtonData("⬅️", "nextmonth_"+previous.Format("01-2006")))
	}
	next := firstOfMonth.AddDate(0, 1, 0)
	navigation = append(navigation, tgbotapi.NewInlineKeyboardButtonData("➡️", "nextmonth_"+next.Format("01-2006")))
	buttons = append(buttons, navigation)

	return tgbotapi.InlineKeyboardMarkup{InlineKeyboard: buttons}
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
