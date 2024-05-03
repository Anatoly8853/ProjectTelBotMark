package main

import (
	"errors"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// CreateCategoryKeyboardReply Массив кнопки под меню.
func CreateCategoryKeyboardReply(categoryName string) ([][]tgbotapi.KeyboardButton, error) {
	// Получаем данные для выбранной категории
	categoryData, found := Categories[categoryName]
	if !found {
		return nil, errors.New("категория не найдена в createCategoryKeyboard")
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
	categoryData, found := Categories[categoryName]
	if !found {
		return nil, errors.New("категория не найдена в createCategoryKeyboard")
	}

	// Создаем первый ряд кнопок с кнопками категории
	var inlineKeyboard [][]tgbotapi.InlineKeyboardButton
	row := []tgbotapi.InlineKeyboardButton{}
	for _, button := range categoryData.Buttons {
		btn := tgbotapi.NewInlineKeyboardButtonData(button.Text, button.Callback)
		row = append(row, btn)
	}
	inlineKeyboard = append(inlineKeyboard, row)

	return inlineKeyboard, nil
}
