package main

import (
	"ProjectTelBotMark/structs"
	"errors"
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Baskets Карта для хранения данных о покупках для каждого пользователя.
var Baskets = make(map[int64][]structs.Purchase)

// BasketMessages Карта для хранения данных о покупках для каждого пользователя.
var BasketMessages = make(map[int64]int)

// messagesToDelete удаление сообщений.
var messagesToDelete = make(map[int64][]int)

// orderDetails содержание корзины.
var orderDetails string

// orders Map для хранения данных по chatID.
var orders = make(map[int64]structs.OrderInfo)

// AddToBasket Добавление товаров в корзину.
func AddToBasket(bot *tgbotapi.BotAPI, update tgbotapi.Update, item string, quantity int, cost int) {
	chatID := update.CallbackQuery.Message.Chat.ID

	// Добавление покупки в корзину пользователя
	purchase := structs.Purchase{
		Item:     item,
		Quantity: quantity,
		Cost:     cost,
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
		logger.Errorf("не удается создать всплывающее сообщение %v", err)
		return
	}
}

// ShowBasket Получение содержимого корзины.
func ShowBasket(bot *tgbotapi.BotAPI, chatID int64) {
	purchases := Baskets[chatID]
	msg := tgbotapi.NewMessage(chatID, "Вы пока ничего не добавили в корзину")

	if len(purchases) > 0 {
		groupedPurchases := make(map[string]structs.Purchase)
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
		logger.Errorf("не удается вывести содержимое корзины %v", err)
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
			logger.Errorf("Удаление сообщения с содержимым корзины %v", err)
		}
		// После успешного удаления сообщения можно обнулить идентификатор сообщения в basketMessages
		BasketMessages[chatID] = 0
	}

	// Отправка подтверждающего сообщения пользователю, что корзина была очищена
	confirmationText := "Корзина успешно очищена."
	confirmMsg := tgbotapi.NewMessage(chatID, confirmationText)
	if _, err := bot.Send(confirmMsg); err != nil {
		logger.Errorf("не удается Отправить подтверждающего сообщения пользователю, что корзина была очищена %v", err)
		return
	}
}

// RequestDeliveryDetails Пример ввода данных заказа.
func RequestDeliveryDetails(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	chatID := update.CallbackQuery.Message.Chat.ID

	// Удаление сообщения
	deleteMessages(bot, chatID)

	// Формирование текста с просьбой ввести данные для доставки
	PhoneNumber := "Номер телефона:"
	instructions := "Пожалуйста, введите " + PhoneNumber + "\n" +
		"Пример:\n" +
		"+79345678901\n"

	// Добавляем или обновляем поле PhoneNumber
	err := addOrUpdateOrderField(chatID, "PhoneNumber", PhoneNumber)
	if err != nil {
		logger.Errorf("Ошибка при добавлении или обновлении поля: %v", err)
	}

	// Отправка сообщения с инструкциями и инлайн-клавиатурой
	msg := tgbotapi.NewMessage(chatID, instructions)

	sentMsg, err := bot.Send(msg)
	if err != nil {
		logger.Errorf("Ошибка при отправке сообщения: %v", err)
		return
	} else {
		// Сохраняем идентификатор отправленного сообщения для последующего удаления
		BasketMessages[chatID] = sentMsg.MessageID
	}
}

// handleDeliveryDetailsMessage Анализируем и извлекаем данные из сообщения.
// Объединяем данные заказа с содержимым корзины.
func handleDeliveryDetailsMessage(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	chatID := update.Message.Chat.ID
	messageID := update.Message.MessageID
	//MsgText := update.Message.Text
	var MsgText string
	if field, err := getFieldByChatID(update.Message.Chat.ID, "PhoneNumber"); err == nil {
		str, _ := field.(string)
		MsgText += "Номер телефона: " + str + "\n"
	}
	//"Адрес доставки: [ваш адрес]\n" +
	if field, err := getFieldByChatID(update.Message.Chat.ID, "DeliveryAddress"); err == nil {
		str, _ := field.(string)
		MsgText += "Адрес доставки: " + str + "\n"
	}
	//"Дата доставки: [желаемая дата]\n" +
	if field, err := getFieldByChatID(update.Message.Chat.ID, "DeliveryDate"); err == nil {
		str, _ := field.(string)
		MsgText += "Дата доставки: " + str + "\n"
	}
	//"ФИО: [ваше имя и фамилия]\n\n" +
	if field, err := getFieldByChatID(update.Message.Chat.ID, "FullName"); err == nil {
		str, _ := field.(string)
		MsgText += "ФИО: " + str + "\n"
	}
	// Объединяем данные заказа с содержимым корзины
	orderDetails = "Детали заказа:\n" + MsgText // Сообщение с деталями заказа
	// Здесь можно добавить логику для добавления информации о содержимом корзины к orderDetails
	// Получение содержимого корзины
	purchases := Baskets[chatID]

	//if len(purchases) > 0 {
	groupedPurchases := make(map[string]structs.Purchase)
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
		MsgTexts += fmt.Sprintf("Товар: %s кол-во: %d цена: %d руб.\n", p.Item, p.Quantity, itemTotalCost)
		totalCost += itemTotalCost
	}

	MsgTexts += fmt.Sprintf("Общая стоимость: %d руб.", totalCost)

	// Очищаем данные заказа
	clearOrder(chatID)

	orderDetails += MsgTexts

	// Удаление сообщения с введенными данными
	deleteMsg := tgbotapi.NewDeleteMessage(chatID, messageID)
	if _, err := bot.Request(deleteMsg); err != nil {
		logger.Errorf("%v", err)
	}

	// Отправляем пользователю подтверждение с деталями заказа и кнопкой "Заказать"
	btnOrder := tgbotapi.NewInlineKeyboardButtonData("Заказать", "confirm_order")
	inlineKeyboard := tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(btnOrder))

	confirmMsg := tgbotapi.NewMessage(chatID, orderDetails)
	confirmMsg.ReplyMarkup = inlineKeyboard

	// Удаление сообщения
	deleteMessages(bot, chatID)

	if sentMsg, err := bot.Send(confirmMsg); err != nil {
		logger.Errorf("%v", err)
		return
	} else {
		// Сохраняем идентификатор отправленного сообщения для последующего удаления
		BasketMessages[chatID] = sentMsg.MessageID
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
			logger.Errorf("Ошибка преобразования ID группового чата: %v", err)
			return
		}
		message := tgbotapi.NewMessage(groupChatID, orderDetails)
		_, err = bot.Send(message)
		if err != nil {
			logger.Errorf("%v", err)
			return
		} else {
			orderDetails = ""
			// Удаление сообщения
			deleteMessages(bot, chatID)
			msg = tgbotapi.NewMessage(chatID, "Ваш заказ успешно отправлен при сборе заказа мы Вас уведомим")
			if _, err = bot.Send(msg); err != nil {
				logger.Errorf("Ошибка отправки сообщения: %v", err)
				return
			}
		}
	} else {
		if _, err := bot.Send(msg); err != nil {
			logger.Errorf("Ошибка отправки сообщения: %v", err)
			return
		}
	}
}

// MainOrdersByUserID Выводим заказы пользователей.
func MainOrdersByUserID(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	userID := update.Message.Chat.ID // Ваш user_id
	orders, err := GetOrdersByUserID(db, userID, 10)
	if err != nil {
		logger.Errorf("Error getting orders: %v", err)
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
				logger.Errorf("Ошибка парсинга строки времени: %v", err)
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

			msgText += fmt.Sprintf("%v кол-во: %v цена: %v руб.\n", order.Purchase.Item, order.Purchase.Quantity, order.Purchase.Cost)

			if i == len(orders)-1 || (i+1 < len(orders) && orders[i+1].DateTime != currentDateTime) {
				msgText += fmt.Sprintf("Общая стоимость:  %v руб.\n", order.TotalCost)
				msgText += "------------------------------------------------------------\n"

			}
		}
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, msgText)
		if _, err = bot.Send(msg); err != nil {
			logger.Errorf("Ошибка отправки сообщения: %v", err)
			return
		}

	} else {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Вы ничего у нас не заказывали")
		if _, err = bot.Send(msg); err != nil {
			logger.Errorf("Ошибка отправки сообщения: %v", err)
			return
		}
	}

}

// addOrUpdateOrderField Функция для добавления или обновления конкретного поля заказа.
func addOrUpdateOrderField(chatID int64, fieldName string, value interface{}) error {
	order, exists := orders[chatID]
	if !exists {
		order = structs.OrderInfo{}
	}

	val := reflect.ValueOf(&order).Elem()
	fieldVal := val.FieldByName(fieldName)
	if !fieldVal.IsValid() {
		return errors.New("такого поля не существует")
	}

	if fieldVal.CanSet() {
		fieldVal.Set(reflect.ValueOf(value))
	} else {
		return errors.New("не удалось установить значение поля")
	}

	orders[chatID] = order
	return nil
}

// clearOrder Функция для очистки данных заказа по chatID.
func clearOrder(chatID int64) {
	delete(orders, chatID)
}

// getFieldByChatID Функция для получения значения поля по chatID и имени поля.
func getFieldByChatID(chatID int64, fieldName string) (interface{}, error) {
	order, exists := orders[chatID]
	if !exists {
		return nil, errors.New("заказ с таким chatID не найден")
	}

	val := reflect.ValueOf(order)
	fieldVal := val.FieldByName(fieldName)
	if !fieldVal.IsValid() {
		return nil, errors.New("такого поля не существует")
	}

	return fieldVal.Interface(), nil
}

// formatDateString Функция для очистки строки от пробелов и знаков препинания, а также форматирования даты.
func formatDateString(dateStr string) (string, string) {
	// Удаляем все пробелы и знаки препинания, кроме точки
	re := regexp.MustCompile(`[^\d.]`)
	cleaned := re.ReplaceAllString(dateStr, "")

	// Разделяем строку на день, месяц и год
	parts := strings.Split(cleaned, ".")
	if len(parts) != 3 {
		err := fmt.Errorf("")
		logger.Infof("неправильный формат даты %v ошибка %v", parts, err)
		return "", "неправильный формат даты"
	}

	// Приводим день и месяц к формату с ведущим нулем
	day, err := strconv.Atoi(parts[0])
	if err != nil {
		logger.Infof("неправильный формат дня %v ошибка %v", day, err)
		return "", "неправильный формат дня"
	}
	month, err := strconv.Atoi(parts[1])
	if err != nil {
		logger.Infof("неправильный формат месяца %v ошибка %v", month, err)
		return "", "неправильный формат месяца"
	}
	year, err := strconv.Atoi(parts[2])
	if err != nil {
		logger.Infof("неправильный формат года %v ошибка %v", year, err)
		return "", "неправильный формат года"
	}

	// Форматируем день и месяц с ведущими нулями
	formattedDate := fmt.Sprintf("%02d.%02d.%04d", day, month, year)

	// Проверяем правильность формата даты
	parsedDate, err := time.Parse("02.01.2006", formattedDate)
	if err != nil {
		logger.Infof("неправильный формат даты %v ошибка %v", formattedDate, err)
		return "", "неправильный формат даты"
	}

	// Получаем текущую дату
	currentTime := time.Now()
	currentYear := currentTime.Year()
	currentMonth := int(currentTime.Month())

	// Проверка, что год совпадает с текущим годом
	if year != currentYear {
		err = fmt.Errorf("год должен быть текущим")
		logger.Infof("год %v ошибка %v", year, err)
		return "", "год должен быть текущим"
	}

	// Проверка, что месяц текущий или следующий
	if month != currentMonth && month != currentMonth+1 {
		err = fmt.Errorf("месяц должен быть текущим или следующим")
		logger.Infof("месяц %v ошибка %v", month, err)
		return "", "месяц должен быть текущим или следующим"
	}

	// Проверка, что дата не прошедшая и не сегодняшняя
	if parsedDate.Before(currentTime) || parsedDate.Equal(currentTime) {
		err = fmt.Errorf("дата не должна быть прошедшей или сегодняшней")
		logger.Infof("дата %v ошибка %v", currentTime, err)
		return "", "дата не должна быть прошедшей или сегодняшней"
	}

	return formattedDate, ""
}

// formatPhoneNumber Функция для проверки и форматирования номера телефона
func formatPhoneNumber(phone string) (string, string) {
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
			logger.Infof("номер телефона %v ошибка %v", cleaned, err)
			return "", "номер должен содержать 10 цифр после +7"
		}
	} else {
		err := fmt.Errorf("номер должен начинаться с +7 ")
		logger.Infof("номер телефона %v ошибка %v", cleaned, err)
		return "", "номер должен начинаться с +7"
	}

	if cleaned == "+79345678901" {
		err := fmt.Errorf("это номер телефона для примера введите свой номер телефона ")
		logger.Infof("%v", err)

		return "", "это номер телефона для примера введите свой номер телефона"
	}

	return cleaned, ""
}

// Функция для отправки сообщения и добавления его в BasketMessages
func sendMessage(bot *tgbotapi.BotAPI, chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	sentMsg, err := bot.Send(msg)
	if err != nil {
		logger.Errorf("не удается отправить сообщение %v", err)
		return
	} else {
		// Сохраняем идентификатор отправленного сообщения для последующего удаления
		// Добавляем новое сообщение в мапу
		BasketMessages[chatID] = sentMsg.MessageID
	}
	// Удаляем предыдущие сообщения, если их больше 3
	//deleteMessages(bot, chatID)
}
