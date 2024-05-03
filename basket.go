package main

import (
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Карта для хранения данных о покупках для каждого пользователя
var baskets = make(map[int64][]Purchase)
var basketMessages = make(map[int64]int)
var messagesToDelete = make(map[int64][]int)
var orderDetails string

// Добавление товаров в корзину
func addToBasket(bot *tgbotapi.BotAPI, update tgbotapi.Update, item string, quantity int, cost int) {
	chatID := update.CallbackQuery.Message.Chat.ID

	// Добавление покупки в корзину пользователя
	purchase := Purchase{
		Item:      item,
		Quantity:  quantity,
		Cost:      cost,
		Timestamp: time.Now(),
	}
	baskets[chatID] = append(baskets[chatID], purchase)

	// Подсчет общей стоимости в корзине
	totalCost := 0
	for _, p := range baskets[chatID] {
		totalCost += p.Cost * p.Quantity
	}

	// Отправка всплывающего сообщения пользователю
	popupText := fmt.Sprintf("Добавлено в корзину:\n %s, кол-во: %d, цена: %d руб.\n Общая стоимость: %d руб", item, quantity, cost, totalCost)
	sendPopupMessage(bot, update, popupText)
}

// Отправка всплывающего сообщения.
func sendPopupMessage(bot *tgbotapi.BotAPI, update tgbotapi.Update, popupText string) {
	callbackConfig := tgbotapi.NewCallback(update.CallbackQuery.ID, popupText)
	callbackConfig.ShowAlert = true // Для отображения в виде всплывающего уведомления
	if _, err := bot.Request(callbackConfig); err != nil {
		log.Panic(err)
	}
}

// Получение содержимого корзины.
func showBasket(bot *tgbotapi.BotAPI, chatID int64, a int) {
	purchases := baskets[chatID]
	msg := tgbotapi.NewMessage(chatID, "Вы пока ничего не добавили в корзину")

	if len(purchases) > 0 {
		groupedPurchases := make(map[string]Purchase)
		for _, p := range purchases {
			key := p.Item
			if existing, found := groupedPurchases[key]; found {
				existing.Quantity += p.Quantity
				existing.Cost += p.Cost
				groupedPurchases[key] = existing
			} else {
				groupedPurchases[key] = p
			}
		}

		var msgText string
		totalCost := 0
		for _, p := range groupedPurchases {
			itemTotalCost := p.Cost // Общая стоимость для этой группы товаров
			msgText += fmt.Sprintf("Товар: %s кол-во: %d цена: %d руб.\n", p.Item, p.Quantity, itemTotalCost)
			totalCost += itemTotalCost
		}

		msgText += fmt.Sprintf("Общая стоимость: %d руб.\n", totalCost)
		msg = tgbotapi.NewMessage(chatID, msgText)

		if a != 1 {
			btnSend := tgbotapi.NewInlineKeyboardButtonData("Оформить заказ", "send_basket_1_1")
			btnDelete := tgbotapi.NewInlineKeyboardButtonData("Очистить корзину", "delete_basket_1_1")
			row := tgbotapi.NewInlineKeyboardRow(btnSend, btnDelete)
			inlineKeyboard := tgbotapi.NewInlineKeyboardMarkup(row)
			msg.ReplyMarkup = inlineKeyboard
		}
	}

	sentMsg, err := bot.Send(msg)
	if err != nil {
		log.Panic(err)
	}
	basketMessages[chatID] = sentMsg.MessageID
}

// Очищаем корзину.
func deleteBasketContents(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	chatID := update.CallbackQuery.Message.Chat.ID

	// Очистка содержимого корзины пользователя
	baskets[chatID] = nil // Или 	baskets[chatID] = []Purchase{} //для обнуления без удаления ключа

	// Удаление сообщения с содержимым корзины
	messageID := basketMessages[chatID]
	//fmt.Println("deleteBasketContents  messageID = ", messageID, "------------------------------------------------------------------")
	if messageID != 0 {
		deleteMsg := tgbotapi.NewDeleteMessage(chatID, messageID)
		if _, err := bot.Request(deleteMsg); err != nil {
			log.Panic(err) // Обработка ошибки по вашему усмотрению
		}
		// После успешного удаления сообщения можно обнулить идентификатор сообщения в basketMessages
		basketMessages[chatID] = 0
	}

	// Отправка подтверждающего сообщения пользователю, что корзина была очищена
	confirmationText := "Корзина успешно очищена."
	confirmMsg := tgbotapi.NewMessage(chatID, confirmationText)
	if _, err := bot.Send(confirmMsg); err != nil {
		log.Panic(err) // Обработка ошибки по вашему усмотрению
	}
}

// Пример ввода данных заказа.
func requestDeliveryDetails(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	chatID := update.CallbackQuery.Message.Chat.ID

	// Удаление сообщения
	deleteMessages(bot, chatID)

	// Получаем текущую дату и добавляем один день
	tomorrow := time.Now().AddDate(0, 0, 1)
	// Форматируем дату в строку
	tomorrowStr := tomorrow.Format("02.01.2006")

	// Формирование текста с просьбой ввести данные для доставки
	instructions := "Пожалуйста, введите данные в следующем формате:\n" +
		"Номер телефона: [ваш номер]\n" +
		"Адрес доставки: [ваш адрес]\n" +
		"Дата доставки: [желаемая дата]\n" +
		"ФИО: [ваше имя и фамилия]\n\n" +
		"Пример:\n"

	// Отправка сообщения с инструкциями и инлайн-клавиатурой
	msg := tgbotapi.NewMessage(chatID, instructions)
	//msg.ReplyMarkup = inlineKeyboard
	sentMsg, err := bot.Send(msg)
	if err != nil {
		log.Printf("Ошибка при отправке сообщения: %v", err)
	} else {
		// Сохраняем идентификатор отправленного сообщения для последующего удаления
		basketMessages[chatID] = sentMsg.MessageID
	}
	str := fmt.Sprintf("Дата доставки: %v\n", tomorrowStr)
	// Формирование текста с просьбой ввести данные для доставки
	instructions =
		"Номер телефона: +79345678901\n" +
			"Адрес доставки: Город, Улица, Дом\n" +
			str +
			"ФИО: Иванов Иван"

	// Отправка сообщения с инструкциями и инлайн-клавиатурой
	msg = tgbotapi.NewMessage(chatID, instructions)
	//msg.ReplyMarkup = inlineKeyboard
	sentMsg, err = bot.Send(msg)
	if err != nil {
		log.Printf("Ошибка при отправке сообщения: %v", err)
	} else {
		// Сохраняем идентификатор отправленного сообщения для последующего удаления
		basketMessages[chatID] = sentMsg.MessageID
	}
}

// Функция для проверки строки заказа.
func validateOrderString(orderString string) string {
	lines := strings.Split(orderString, "\n")

	// Проверка строки на наличие всех полей
	if len(lines) != 4 {
		return "неверный формат строки заказа все 4 строки обязательные"
	}
	// Разбиваем строку на части по указателю ":"
	phoneParts := strings.Split(lines[0], ":")
	// Проверка номера телефона
	phonePart := strings.Split(phoneParts[1], " ")
	phoneRegex := regexp.MustCompile(`^(\+7\d{10}|8\d{10})$`)
	if !phoneRegex.MatchString(strings.TrimSpace(phonePart[1])) {
		return "неверный формат номера телефона"
	}
	if phoneParts[1] == " +79345678901" {
		return "это номер телефона для примера введите свой номер телефона"
	}

	// Проверка адреса доставки
	// Разбиваем строку на части по указателю ":"
	adresParts := strings.Split(lines[1], ":")
	if strings.TrimSpace(adresParts[1]) == "" {
		return "адрес доставки не может быть пустым"
	}
	if adresParts[1] == " Город, Улица, Дом" {
		return "ведите свой адрес"
	}

	// Проверка даты доставки
	// Разбиваем строку на части по указателю ":"
	dateParts := strings.Split(lines[2], ":")
	dateLayout := "02.01.2006"
	_, err := time.Parse(dateLayout, strings.TrimSpace(dateParts[1]))
	if err != nil {
		return "неверный формат даты доставки провельный формат даты 02.01.2006"
	}
	if !isFutureDate(dateParts[1]) {
		return "выбранная дата доставки не может быть прошедшей или текущей датой"
	}

	// Проверка ФИО
	// Разбиваем строку на части по указателю ":"
	fioParts := strings.Split(lines[3], ":")
	if strings.TrimSpace(fioParts[1]) == "" {
		return "ФИО не может быть пустым"
	}

	return ""
}

// Анализируем и извлекаем данные из сообщения.
// Объединяем данные заказа с содержимым корзины.
func handleDeliveryDetailsMessage(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	chatID := update.Message.Chat.ID
	messageID := update.Message.MessageID
	MsgText := update.Message.Text

	// Проверяем, соответствует ли сообщение ожидаемому формату
	if str := validateOrderString(MsgText); str != "" {

		// Если данные введены некорректно, просим ввести их заново
		if _, err := bot.Send(tgbotapi.NewMessage(chatID, str)); err != nil {
			fmt.Println(err)
		}

	} else {

		// Объединяем данные заказа с содержимым корзины
		orderDetails = "Детали заказа:\n" + MsgText // Сообщение с деталями заказа
		// Здесь можно добавить логику для добавления информации о содержимом корзины к orderDetails
		// Получение содержимого корзины
		purchases := baskets[chatID]

		//if len(purchases) > 0 {
		groupedPurchases := make(map[string]Purchase)
		for _, p := range purchases {
			key := p.Item
			if existing, found := groupedPurchases[key]; found {
				existing.Quantity += p.Quantity
				existing.Cost += p.Cost
				groupedPurchases[key] = existing
			} else {
				groupedPurchases[key] = p
			}
		}
		var MsgTexts string
		totalCost := 0
		for _, p := range groupedPurchases {
			itemTotalCost := p.Cost // Общая стоимость для этой группы товаров
			MsgTexts += fmt.Sprintf("\nТовар: %s кол-во: %d цена: %d руб.", p.Item, p.Quantity, itemTotalCost)
			totalCost += itemTotalCost
		}

		MsgTexts += fmt.Sprintf("\nОбщая стоимость: %d руб.", totalCost)

		orderDetails += MsgTexts

		// Удаление сообщения с введенными данными
		deleteMsg := tgbotapi.NewDeleteMessage(chatID, messageID)
		if _, err := bot.Request(deleteMsg); err != nil {
			fmt.Println(err) // Обработка ошибки по вашему усмотрению
		}

		// Отправляем пользователю подтверждение с деталями заказа и кнопкой "Заказать"
		btnOrder := tgbotapi.NewInlineKeyboardButtonData("Заказать", "confirm_order")
		inlineKeyboard := tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(btnOrder))

		confirmMsg := tgbotapi.NewMessage(chatID, orderDetails)
		confirmMsg.ReplyMarkup = inlineKeyboard
		//}
		// Удаление сообщения
		deleteMessages(bot, chatID)

		if sentMsg, err := bot.Send(confirmMsg); err != nil {
			fmt.Println(err) // Обработка ошибки по вашему усмотрению
		} else {
			// Сохраняем идентификатор отправленного сообщения для последующего удаления
			basketMessages[chatID] = sentMsg.MessageID
		}
	}
}

// Удаляем сообщения
func deletePreviousMessages(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	messageID := update.Message.MessageID
	chatID := update.Message.Chat.ID

	// Проверяем, есть ли сообщения для удаления в этом чате
	if _, found := messagesToDelete[chatID]; found {
		// Проходимся по каждому идентификатору сообщения и удаляем его
		if messageID > 0 {
			deleteMsg := tgbotapi.NewDeleteMessage(chatID, messageID)
			if _, err := bot.Request(deleteMsg); err != nil {
				//log.Printf("Ошибка при удалении сообщения: %v", err)
				// Не используйте log.Panic, чтобы избежать прерывания работы бота из-за одной ошибки
			}
		}
		/*
			for chatIDs, messageIDs := range messagesToDelete {
				fmt.Printf("ChatID: %d-----------------------------------", chatIDs)
				for _, messageIDa := range messageIDs {
					fmt.Printf(" MessageID: %d---------------------------------------------------------------------------------------\n", messageIDa)
				}
			}

		*/
	}
}

// Удаляем сообщения
func deleteMessages(bot *tgbotapi.BotAPI, chatID int64) {
	// Удаление сообщения
	messageIDs := basketMessages[chatID]
	if messageIDs != 0 {
		deleteMsg := tgbotapi.NewDeleteMessage(chatID, messageIDs)
		if _, err := bot.Request(deleteMsg); err != nil {
			//log.Panic(err) // Обработка ошибки по вашему усмотрению
		}

		//fmt.Println("deleteMessages ------------------------------------------------------------------------------", messageIDs)
	}
}

// отправляем заказ
func groupMessagesChatID(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	chatID := update.CallbackQuery.Message.Chat.ID
	msg := tgbotapi.NewMessage(chatID, "Нет данных для отправки")
	if orderDetails != "" {
		// Отправляем сообщение в групповой чат
		// Получите ID группового чата из переменной окружения
		groupChatIDStr := os.Getenv("TELEGRAM_GROUPCHATID")
		groupChatID, err := strconv.ParseInt(groupChatIDStr, 10, 64)
		if err != nil {
			log.Printf("Ошибка преобразования ID группового чата: %v", err)
			return
		}
		message := tgbotapi.NewMessage(groupChatID, orderDetails)
		_, err = bot.Send(message)
		if err != nil {
			//log.Panic(err)
			fmt.Println("groupMessagesChatID err = ", err)
			return
		} else {
			orderDetails = ""
			// Удаление сообщения
			deleteMessages(bot, chatID)
			msg = tgbotapi.NewMessage(chatID, "Ваш заказ успешно отправлен при сборе заказа мы Вас уведомим")
			if _, err = bot.Send(msg); err != nil {
				log.Println("Ошибка отправки сообщения:", err)
			}
		}
	} else {
		if _, err := bot.Send(msg); err != nil {
			log.Println("Ошибка отправки сообщения:", err)
		}
	}
}

func isFutureDate(selectedDate string) bool {
	lines := strings.Split(selectedDate, " ")
	//fmt.Println("lines[0]---------------------------------------------", lines[0])
	//fmt.Println("lines[1]---------------------------------------------", lines[1])
	layout := "02.01.2006" // Формат даты
	t, err := time.Parse(layout, lines[1])
	if err != nil {
		fmt.Println("Ошибка при разборе даты:", err)
		return false
	}
	// Получаем текущую дату
	currentTime := time.Now()
	// Сравниваем выбранную дату с текущей
	return t.After(currentTime) || t.Equal(currentTime)
}
