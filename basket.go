package main

import (
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Baskets Карта для хранения данных о покупках для каждого пользователя.
var Baskets = make(map[int64][]Purchase)

// BasketMessages Карта для хранения данных о покупках для каждого пользователя.
var BasketMessages = make(map[int64]int)

// messagesToDelete удалкение сообщений.
var messagesToDelete = make(map[int64][]int)

// orderDetails содержание корзины.
var orderDetails string

// AddToBasket Добавление товаров в корзину.
func AddToBasket(bot *tgbotapi.BotAPI, update tgbotapi.Update, item string, quantity int, cost int) {
	chatID := update.CallbackQuery.Message.Chat.ID

	// Добавление покупки в корзину пользователя
	purchase := Purchase{
		Item:      item,
		Quantity:  quantity,
		Cost:      cost,
		Timestamp: time.Now(),
	}
	Baskets[chatID] = append(Baskets[chatID], purchase)

	// Подсчет общей стоимости в корзине
	totalCost := 0
	for _, p := range Baskets[chatID] {
		totalCost += p.Cost * p.Quantity
	}

	// Отправка всплывающего сообщения пользователю
	popupText := fmt.Sprintf("Добавлено в корзину:\n %s, кол-во: %d, цена: %d руб.\n Общая стоимость: %d руб", item, quantity, cost, totalCost)
	SendPopupMessage(bot, update, popupText)
}

// SendPopupMessage Отправка всплывающего сообщения.
func SendPopupMessage(bot *tgbotapi.BotAPI, update tgbotapi.Update, popupText string) {
	callbackConfig := tgbotapi.NewCallback(update.CallbackQuery.ID, popupText)
	callbackConfig.ShowAlert = true // Для отображения в виде всплывающего уведомления
	if _, err := bot.Request(callbackConfig); err != nil {
		logError("не удается создать всплывающее сообщение", err)
		return
	}
}

// ShowBasket Получение содержимого корзины.
func ShowBasket(bot *tgbotapi.BotAPI, chatID int64) {
	purchases := Baskets[chatID]
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

		btnSend := tgbotapi.NewInlineKeyboardButtonData("Оформить заказ", "send_basket_1_1")
		btnDelete := tgbotapi.NewInlineKeyboardButtonData("Очистить корзину", "delete_basket_1_1")
		row := tgbotapi.NewInlineKeyboardRow(btnSend, btnDelete)
		inlineKeyboard := tgbotapi.NewInlineKeyboardMarkup(row)
		msg.ReplyMarkup = inlineKeyboard

	}

	sentMsg, err := bot.Send(msg)
	if err != nil {
		logError("не удается вывести содержимое корзины", err)
		return
	}
	BasketMessages[chatID] = sentMsg.MessageID
}

// DeleteBasketContents Очищаем корзину.
func DeleteBasketContents(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	chatID := update.CallbackQuery.Message.Chat.ID

	// Очистка содержимого корзины пользователя
	Baskets[chatID] = nil // Или 	baskets[chatID] = []Purchase{} //для обнуления без удаления ключа

	// Удаление сообщения с содержимым корзины
	messageID := BasketMessages[chatID]

	if messageID != 0 {
		deleteMsg := tgbotapi.NewDeleteMessage(chatID, messageID)
		if _, err := bot.Request(deleteMsg); err != nil {
			logError("Удаление сообщения с содержимым корзины", err)
		}
		// После успешного удаления сообщения можно обнулить идентификатор сообщения в basketMessages
		BasketMessages[chatID] = 0
	}

	// Отправка подтверждающего сообщения пользователю, что корзина была очищена
	confirmationText := "Корзина успешно очищена."
	confirmMsg := tgbotapi.NewMessage(chatID, confirmationText)
	if _, err := bot.Send(confirmMsg); err != nil {
		logError("не удается Отправить подтверждающего сообщения пользователю, что корзина была очищена", err)
		return
	}
}

func Instructions() string {
	// Получаем текущую дату и добавляем один день
	tomorrow := time.Now().AddDate(0, 0, 1)
	// Форматируем дату в строку
	tomorrowStr := tomorrow.Format("02.01.2006")

	str := fmt.Sprintf("Дата доставки: %v\n", tomorrowStr)
	// Формирование текста с просьбой ввести данные для доставки
	instructions :=
		"Номер телефона: +79345678901\n" +
			"Адрес доставки: Город, Улица, Дом\n" +
			str +
			"ФИО: Иванов Иван"
	return instructions
}

// RequestDeliveryDetails Пример ввода данных заказа.
func RequestDeliveryDetails(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	chatID := update.CallbackQuery.Message.Chat.ID

	// Удаление сообщения
	deleteMessages(bot, chatID)

	// Формирование текста с просьбой ввести данные для доставки
	instructions := "Пожалуйста, введите данные в следующем формате:\n" +
		"Номер телефона: [ваш номер]\n" +
		"Адрес доставки: [ваш адрес]\n" +
		"Дата доставки: [желаемая дата]\n" +
		"ФИО: [ваше имя и фамилия]\n\n" +
		"Пример:\n"

	// Отправка сообщения с инструкциями и инлайн-клавиатурой
	msg := tgbotapi.NewMessage(chatID, instructions)

	sentMsg, err := bot.Send(msg)
	if err != nil {
		logError("Ошибка при отправке сообщения:", err)
		return
	} else {
		// Сохраняем идентификатор отправленного сообщения для последующего удаления
		BasketMessages[chatID] = sentMsg.MessageID
	}

	// Отправка сообщения с инструкциями и инлайн-клавиатурой
	msg = tgbotapi.NewMessage(chatID, Instructions())
	//msg.ReplyMarkup = inlineKeyboard
	sentMsg, err = bot.Send(msg)
	if err != nil {
		logError("Ошибка при отправке сообщения:", err)
		return
	} else {
		// Сохраняем идентификатор отправленного сообщения для последующего удаления
		BasketMessages[chatID] = sentMsg.MessageID
	}
}

// ValidateOrderString Функция для проверки строки заказа.
func ValidateOrderString(orderString string) string {
	lines := strings.Split(orderString, "\n")

	// Проверка строки на наличие всех полей
	if len(lines) != 4 {
		err := fmt.Errorf("Неверный формат строки заказа все 4 строки обязательные")
		logError("", err)
		return "Неверный формат строки заказа все 4 строки обязательные"
	}
	// Разбиваем строку на части по указателю ":"
	phoneParts := strings.Split(lines[0], ":")
	if len(phoneParts) != 2 {
		err := fmt.Errorf("в строке Номер телефона нет : " + phoneParts[0])
		logError("", err)
		return "в строке Номер телефона нет :"
	}
	// Проверка номера телефона
	phonePart := strings.ReplaceAll(phoneParts[1], " ", "") // Удаление всех пробелов из строки

	phoneRegex := regexp.MustCompile(`^(\+7\d{10}|8\d{10})$`)
	if !phoneRegex.MatchString(strings.TrimSpace(phonePart)) {
		err := fmt.Errorf("неверный формат номера телефона " + phonePart)
		logError("", err)
		return "неверный формат номера телефона " + phonePart
	}

	if phonePart == "+79345678901" {
		err := fmt.Errorf("это номер телефона для примера введите свой номер телефона ")
		logError("", err)
		return "это номер телефона для примера введите свой номер телефона"
	}

	// Проверка адреса доставки
	// Разбиваем строку на части по указателю ":"
	adresParts := strings.Split(lines[1], ":")
	if len(adresParts) != 2 {
		err := fmt.Errorf("в строке Адрес доставки нет : " + adresParts[0])
		logError("", err)
		return "в строке Адрес доставки нет : " + adresParts[0]
	}
	if strings.TrimSpace(adresParts[1]) == "" {
		err := fmt.Errorf("адрес доставки не может быть пустым")
		logError("", err)
		return "адрес доставки не может быть пустым"
	}
	if adresParts[1] == " Город, Улица, Дом" || adresParts[1] == "Город, Улица, Дом" {
		err := fmt.Errorf("ведите свой адрес")
		logError("", err)
		return "ведите свой адрес"
	}

	// Проверка даты доставки
	// Разбиваем строку на части по указателю ":"
	dateParts := strings.Split(lines[2], ":")
	if len(dateParts) != 2 {
		err := fmt.Errorf("в строке Дата доставки нет : " + dateParts[0])
		logError("", err)
		return "в строке Дата доставки нет :"
	}
	dateLayout := "02.01.2006"
	_, err := time.Parse(dateLayout, strings.TrimSpace(dateParts[1]))
	if err != nil {
		err = fmt.Errorf("неверный формат даты доставки провельный формат даты 02.01.2006 - " + dateParts[1])
		logError("", err)
		return "неверный формат даты доставки провельный формат даты 02.01.2006"
	}
	if !IsFutureDate(dateParts[1]) {
		err = fmt.Errorf("неверный формат даты доставки провельный формат даты 02.01.2006 - " + dateParts[1])
		logError("", err)
		return "выбранная дата доставки не может быть прошедшей или текущей датой"
	}

	// Проверка ФИО
	// Разбиваем строку на части по указателю ":"
	fioParts := strings.Split(lines[3], ":")
	if len(fioParts) != 2 {
		err = fmt.Errorf("в строке ФИО нет :" + fioParts[0])
		logError("", err)
		return "в строке ФИО нет :"
	}
	if strings.TrimSpace(fioParts[1]) == "" {
		err = fmt.Errorf("ФИО не может быть пустым")
		logError("", err)
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
	if str := ValidateOrderString(MsgText); str != "" {

		// Если данные введены некорректно, просим ввести их заново
		str = str + "\nВведите все данные повторно"
		if _, err := bot.Send(tgbotapi.NewMessage(chatID, str)); err != nil {
			logError("Не удаеься отправить сообщение", err)
			return
		}

		if _, err := bot.Send(tgbotapi.NewMessage(chatID, MsgText)); err != nil {
			logError("Не удаеься отправить сообщение", err)
			return
		}

	} else {

		// Объединяем данные заказа с содержимым корзины
		orderDetails = "Детали заказа:\n" + MsgText // Сообщение с деталями заказа
		// Здесь можно добавить логику для добавления информации о содержимом корзины к orderDetails
		// Получение содержимого корзины
		purchases := Baskets[chatID]

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
			logError("", err)
		}

		// Отправляем пользователю подтверждение с деталями заказа и кнопкой "Заказать"
		btnOrder := tgbotapi.NewInlineKeyboardButtonData("Заказать", "confirm_order")
		inlineKeyboard := tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(btnOrder))

		confirmMsg := tgbotapi.NewMessage(chatID, orderDetails)
		confirmMsg.ReplyMarkup = inlineKeyboard

		// Удаление сообщения
		deleteMessages(bot, chatID)

		if sentMsg, err := bot.Send(confirmMsg); err != nil {
			logError("", err)
			return
		} else {
			// Сохраняем идентификатор отправленного сообщения для последующего удаления
			BasketMessages[chatID] = sentMsg.MessageID
		}
	}
}

// deletePreviousMessages Удаляем сообщения.
func deletePreviousMessages(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	messageID := update.Message.MessageID
	chatID := update.Message.Chat.ID

	// Проверяем, есть ли сообщения для удаления в этом чате
	if _, found := messagesToDelete[chatID]; found {
		// Проходимся по каждому идентификатору сообщения и удаляем его
		deleteMsg := tgbotapi.NewDeleteMessage(chatID, messageID)
		if _, err := bot.Request(deleteMsg); err != nil {
			//log.Printf("Ошибка при удалении сообщения: %v", err)
			// Не используйте log.Panic, чтобы избежать прерывания работы бота из-за одной ошибки
		}
		/*
			for chatIDs, messageIDs := range messagesToDelete {
				fmt.Printf("ChatID: %d-----------------------------------\n", chatIDs)

				for _, messageIDa := range messageIDs {
					fmt.Println("len(messageIDa)---------------------------------------------------------------------------", len(messageIDs))
					fmt.Printf(" MessageID: %d---------------------------------------------------------------------------------------\n", messageIDa)
				}

			}

		*/

	}
}

// deleteMessages Удаляем сообщения.
func deleteMessages(bot *tgbotapi.BotAPI, chatID int64) {
	// Удаление сообщения
	messageIDs := BasketMessages[chatID]
	deleteMsg := tgbotapi.NewDeleteMessage(chatID, messageIDs)
	if _, err := bot.Request(deleteMsg); err != nil {
		//log.Panic(err) // Обработка ошибки по вашему усмотрению
	}
}

// GroupMessagesChatID отправляем заказ.
func GroupMessagesChatID(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	chatID := update.CallbackQuery.Message.Chat.ID
	msg := tgbotapi.NewMessage(chatID, "Нет данных для отправки")
	if orderDetails != "" {
		// Отправляем сообщение в групповой чат
		// Получите ID группового чата из переменной окружения
		groupChatIDStr := os.Getenv("TELEGRAM_GROUPCHATID")
		groupChatID, err := strconv.ParseInt(groupChatIDStr, 10, 64)
		if err != nil {
			logError("Ошибка преобразования ID группового чата: ", err)
			return
		}
		message := tgbotapi.NewMessage(groupChatID, orderDetails)
		_, err = bot.Send(message)
		if err != nil {
			logError("groupMessagesChatID err = ", err)
			return
		} else {
			orderDetails = ""
			// Удаление сообщения
			deleteMessages(bot, chatID)
			msg = tgbotapi.NewMessage(chatID, "Ваш заказ успешно отправлен при сборе заказа мы Вас уведомим")
			if _, err = bot.Send(msg); err != nil {
				logError("Ошибка отправки сообщения:", err)
				return
			}
		}
	} else {
		if _, err := bot.Send(msg); err != nil {
			logError("Ошибка отправки сообщения:", err)
			return
		}
	}
}

// IsFutureDate проверяем валидность даты введенной пользователем при заказе.
func IsFutureDate(selectedDate string) bool {
	lines := strings.Split(selectedDate, " ")
	layout := "02.01.2006" // Формат даты
	var linest string
	if len(lines) == 2 {
		linest = lines[1]
	} else {
		linest = lines[0]
	}
	t, err := time.Parse(layout, linest)
	if err != nil {
		logError("Ошибка при разборе даты:", err)
		return false
	}
	// Получаем текущую дату
	currentTime := time.Now()
	// Сравниваем выбранную дату с текущей
	return t.After(currentTime) || t.Equal(currentTime)
}

// MainOrdersByUserID Выводим заказы пользователей.
func MainOrdersByUserID(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	userID := update.Message.Chat.ID // Ваш user_id
	orders, err := GetOrdersByUserID(db, userID, 10)
	if err != nil {
		logError("Error getting orders:", err)
		return
	}
	//сортируем слайс от наибольшей даты к наименьшей
	sort.Sort(ByDeliveryDate(orders))

	if len(orders) > 0 {
		var msgText string
		var currentDeliveryDate string // Объявляем переменную для отслеживания текущей даты доставки
		var currentDateTime string

		for i, order := range orders {
			// Форматирование даты заказа
			// Парсинг строки времени в тип time.Time
			t, err := time.Parse("15:04 02.01.2006", order.DateTime)
			if err != nil {
				logError("Ошибка парсинга строки времени:", err)
				return
			}

			// Форматирование времени в нужный формат
			formattedDate := t.Format("02.01.2006")

			// Группировка по дате доставки
			if order.DeliveryDate != currentDeliveryDate || order.DateTime != currentDateTime {
				msgText += fmt.Sprintf("Дата заказа: %v\n", formattedDate)
				msgText += fmt.Sprintf("Дата доставки: %v\n", order.DeliveryDate)
				msgText += fmt.Sprintf("Заказчик: %v\n", order.FullName)
				msgText += fmt.Sprintf("Телефон для связи: %v\n", order.PhoneNumber)
				msgText += fmt.Sprintf("Адрес доставки: %v\n", order.DeliveryAddress)
				msgText += "Список заказанных товаров :\n"
				currentDeliveryDate = order.DeliveryDate
				currentDateTime = order.DateTime
			}

			msgText += fmt.Sprintf("%v кол-во: %v цена: %v руб.\n", order.ItemName, order.Quantity, order.Cost)

			if i == len(orders)-1 || (i+1 < len(orders) && orders[i+1].DateTime != currentDateTime) {
				msgText += fmt.Sprintf("Общая стоимость:  %v руб.\n", order.TotalCost)
				msgText += "------------------------------------------------------------\n"

			}
		}
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, msgText)
		if _, err = bot.Send(msg); err != nil {
			logError("Ошибка отправки сообщения:", err)
			return
		}

	} else {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Вы ничего у нас не заказывали")
		if _, err = bot.Send(msg); err != nil {
			logError("Ошибка отправки сообщения:", err)
			return
		}
	}

}
