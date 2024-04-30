package main

import (
	"errors"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"log"
)

type MenuState int


var numericKeyboard = tgbotapi.NewReplyKeyboard(
	tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton("🥕Морковь🥕"),
		tgbotapi.NewKeyboardButton("🥬Капуста🥬"),
		tgbotapi.NewKeyboardButton("Лобио"),
	),
	tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton("Черемша"),
		tgbotapi.NewKeyboardButton("🍆Баклажаны🍆"),
		tgbotapi.NewKeyboardButton("🍄Грибы🍄"),
	),
)

var numericKeyboard2 = tgbotapi.NewInlineKeyboardMarkup(
	tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("⚖️1кг\n💵200 рублей", "add_🥕Морковь ⚖️1кг_1_200"),
		tgbotapi.NewInlineKeyboardButtonData("⚖ведёрко 0,9\n💵110 рублей", "add_🥕Морковь ⚖ведёрко 0,9_1_110"),
		//tgbotapi.NewInlineKeyboardButtonData("3", "3"),
	),
	//tgbotapi.NewInlineKeyboardRow(
	//	tgbotapi.NewInlineKeyboardButtonData("4", "4"),
	//	tgbotapi.NewInlineKeyboardButtonData("5", "5"),
	//	tgbotapi.NewInlineKeyboardButtonData("6", "6"),
	//),
)

func PotoPraisBot(bot *tgbotapi.BotAPI, update tgbotapi.Update, s string, jpeg string) error {
	var err error
	// Создаем медиа-объект для изображения
	photo := tgbotapi.NewInputMediaPhoto(tgbotapi.FilePath(jpeg))

	// Создаем медиагруппу с изображением и URL
	mediaGroup := tgbotapi.NewMediaGroup(update.Message.Chat.ID, []interface{}{photo})

	// Отправляем медиагруппу
	if _, err = bot.SendMediaGroup(mediaGroup); err != nil {
		log.Println("Error sending media group:", err)
		return err
	}

	// Устанавливаем всплывающую клавиатуру с надписями 1-6 в два ряда
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, s)
	msg.ReplyMarkup = numericKeyboard2
	//msg.ReplyToMessageID = update.Message.MessageID // Устанавливаем ID сообщения, на которое отвечаем
	if _, err = bot.Send(msg); err != nil {
		panic(err)
	}
	return err
}

// Функция создания основного меню
func mainMenuKeyboard() tgbotapi.ReplyKeyboardMarkup {
	return tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("🥕Морковь🥕"),
			tgbotapi.NewKeyboardButton("🥬Капуста🥬"),
			tgbotapi.NewKeyboardButton("Лобио"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Черемша"),
			tgbotapi.NewKeyboardButton("🍆Баклажаны🍆"),
			tgbotapi.NewKeyboardButton("🍄Грибы🍄"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("🛒Корзина🛒"),
		),
	)
}

// Функция создания клавиатуры для меню капусты
func cabbageMenuKeyboard() tgbotapi.ReplyKeyboardMarkup {
	return tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Пекинская капуста"),
			tgbotapi.NewKeyboardButton("Цветная капуста"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Капуста сырая"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("🔙Назад🔙"),
		),
	)
}

// Функция для отправки команды при нажатии кнопки "Капуста"
func sendCabbageCommand(bot *tgbotapi.BotAPI, chatID int64) {
	command := "cabbage"
	msg := tgbotapi.NewMessage(chatID, "/"+command)
	bot.Send(msg)
}

// Отправка меню с вариантами капусты
func sendCabbageMenu(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "Выберите вид капусты:")
	// Создаем клавиатуру с вариантами капусты
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Капуста квашеная", "cabbage_pickled"),
			tgbotapi.NewInlineKeyboardButtonData("Капуста соленая", "cabbage_salted"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Капуста сырая", "cabbage_raw"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "back_to_main_menu"),
		),
	)
	msg.ReplyMarkup = keyboard
	_, err := bot.Send(msg)
	if err != nil {
		log.Println("Ошибка отправки сообщения:", err)
	}
}

// ///////////////////////////////////////////////////////////////////
// Функция создания клавиатуры для меню лобио
func beansMenuKeyboard() tgbotapi.ReplyKeyboardMarkup {
	return tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Лобио маринованное"),
			tgbotapi.NewKeyboardButton("Лобио соленое"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Лобио сушеное"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("🔙Назад🔙"),
		),
	)
}

// Функция отправки клавиатуры меню
func sendMainMenu(bot *tgbotapi.BotAPI, update tgbotapi.Update, s string) {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Выберите  продукты :")
	keyboardReply, err := mainMenuKeyboardButtonData(s)
	if err != nil {
		// Обработка ошибки
		return
	}
	// Создаем сообщение с клавиатурой
	msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(keyboardReply...)

	// Отправляем сообщение с клавиатурой
	_, err = bot.Send(msg)
	if err != nil {
		log.Println("Ошибка отправки сообщения с клавиатурой:", err)
		return
	}
}


func mainMenuKeyboardButtonData(categoryName string) ([][]tgbotapi.KeyboardButton, error) {
	// Получаем данные для выбранной категории
	categoryData, found := Categories[categoryName]
	if !found {
		return nil, errors.New("категория не найдена в createCategoryKeyboard")
	}

	// Создаем массив рядов кнопок
	var keyboard [][]tgbotapi.KeyboardButton

	// Создаем первый ряд кнопок с кнопками категории
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

	// Добавляем второй ряд с кнопкой "Назад"
	//backButton := tgbotapi.NewKeyboardButton("🔙Назад🔙")
	//secondRow := []tgbotapi.KeyboardButton{backButton}
	//keyboard = append(keyboard, secondRow)

	return keyboard, nil
}
