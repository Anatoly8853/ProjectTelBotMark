package main

import (
	"ProjectTelBotMark/dataSources"
	"ProjectTelBotMark/structs"
	"errors"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Baskets Карта для хранения данных о покупках для каждого пользователя.
var Baskets = make(map[int64][]structs.Purchase)

// BasketMessages Карта для хранения данных о покупках для каждого пользователя.
var BasketMessages = make(map[int64]int)

var DeliveryDateChatID = make(map[int64]int)

// messagesToDelete удаление сообщений.
var messagesToDelete = make(map[int64][]int)

// orderDetails содержание корзины.
var orderDetails = make(map[int64]string)

// orders Map для хранения данных по chatID.
var orders = make(map[int64]structs.OrderDb)

// AddToBasket Добавление товаров в корзину.
func AddToBasket(bot *tgbotapi.BotAPI, update tgbotapi.Update, item string, quantity int, cost int) {
	if update.CallbackQuery.Message.Chat.ID == 0 {
		logger.Errorf("нет данных для обработки update.CallbackQuery.Message.Chat.ID = %v ", update.CallbackQuery.Message.Chat.ID)
		return
	}

	if item == "" || quantity == 0 || cost == 0 {
		logger.Errorf("нет данных для обработки item = %v quantity = %v cost = %v", item, quantity, cost)
		return
	}
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
	if update.CallbackQuery.ID == "" || update.CallbackQuery.Message == nil || popupText == "" {
		logger.Errorf("нет данных для обработки update.CallbackQuery.Message = %v popupText = %v", update.CallbackQuery.Message, popupText)
		return
	}
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
func DeleteBasketContents(bot *tgbotapi.BotAPI, chatID int64) {
	if chatID == 0 {
		logger.Errorf("нет данных для обработки chatID = %v", chatID)
		return
	}

	// Очистка содержимого корзины пользователя
	Baskets[chatID] = nil // Или 	baskets[chatID] = []Purchase{} //для обнуления без удаления ключа

	// Удаление сообщения с содержимым корзины
	if BasketMessages[chatID] == 0 {
		logger.Errorf("нет данных для обработки BasketMessages[chatID] = %v", BasketMessages[chatID])
		return
	}

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
	sendMessage(bot, chatID, "Корзина успешно очищена.")

}

// RequestDeliveryDetails Пример ввода данных заказа дата доставки.
func RequestDeliveryDetails(bot *tgbotapi.BotAPI, chatID int64) {
	if chatID == 0 {
		logger.Errorf("нет данных для обработки chatID = %v", chatID)
		return
	}

	// Удаление сообщения
	deleteMessages(bot, chatID)

	DeliveryDate := "Дату доставки:"

	err := addOrUpdateOrderField(chatID, "DeliveryDate", DeliveryDate)
	if err != nil {
		logger.Errorf("Ошибка при добавлении или обновлении поля DeliveryDate: %v", err)
		return
	}

	msg := tgbotapi.NewMessage(chatID, "Выберите дату доставки:")
	now := time.Now()
	msg.ReplyMarkup = createCalendarKeyboard(now)

	_, err = bot.Send(msg)
	if err != nil {
		logger.Errorf("не удается вывести содержимое корзины %v", err)
		return
	}

	DeliveryDateChatID[chatID] = 1
}

// handleDeliveryDetailsMessage Анализируем и извлекаем данные из сообщения.
// Объединяем данные заказа с содержимым корзины.
func handleDeliveryDetailsMessage(bot *tgbotapi.BotAPI, chatID int64) {
	if chatID == 0 {
		logger.Errorf("нет данных для обработки chatID = %v", chatID)
		return
	}

	var MsgText string

	if field, err := getFieldByChatID(chatID, "PhoneNumber"); err == nil {
		str, _ := field.(string)
		MsgText += "Номер телефона: " + str + "\n"
	}

	//"Адрес доставки: [ваш адрес]\n" +
	if field, err := getFieldByChatID(chatID, "DeliveryAddress"); err == nil {
		str, _ := field.(string)
		MsgText += "Адрес доставки: " + str + "\n"
	}

	//"Дата доставки: [желаемая дата]\n" +
	if field, err := getFieldByChatID(chatID, "DeliveryDate"); err == nil {
		str, _ := field.(string)
		MsgText += "Дата доставки: " + str + "\n"
	}

	//"ФИО: [ваше имя и фамилия]\n\n" +
	if field, err := getFieldByChatID(chatID, "FullName"); err == nil {
		str, _ := field.(string)
		MsgText += "ФИО: " + str + "\n"
	}

	// Объединяем данные заказа с содержимым корзины
	orderDetails[chatID] = "Детали заказа:\n" + MsgText // Сообщение с деталями заказа

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

	orderDetails[chatID] += MsgTexts

	// Отправляем пользователю подтверждение с деталями заказа и кнопкой "Заказать"
	btnOrder := tgbotapi.NewInlineKeyboardButtonData("Заказать", "confirm_order")
	inlineKeyboard := tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(btnOrder))

	confirmMsg := tgbotapi.NewMessage(chatID, orderDetails[chatID])
	confirmMsg.ReplyMarkup = inlineKeyboard

	if sentMsg, err := bot.Send(confirmMsg); err != nil {
		logger.Errorf("%v", err)
		return
	} else {
		// Сохраняем идентификатор отправленного сообщения для последующего удаления
		BasketMessages[chatID] = sentMsg.MessageID
	}
}

// deletePreviousMessages Удаляем сообщения.
func deletePreviousMessages(bot *tgbotapi.BotAPI, chatID int64, messageID int) {
	if chatID == 0 || messageID == 0 {
		logger.Errorf("нет данных для обработки chatID = %v messageID = %v", chatID, messageID)
		return
	}

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
	if chatID == 0 {
		logger.Errorf("нет данных для обработки chatID = %v", chatID)
		return
	}
	// Удаление сообщения
	messageIDs := BasketMessages[chatID]
	deleteMsg := tgbotapi.NewDeleteMessage(chatID, messageIDs)
	if _, err := bot.Request(deleteMsg); err != nil {
		//log.Panic(err) // Обработка ошибки по вашему усмотрению
	}
}

// GroupMessagesChatID отправляем заказ.
func GroupMessagesChatID(bot *tgbotapi.BotAPI, chatID int64) {
	if chatID == 0 {
		logger.Errorf("нет данных для обработки chatID = %v", chatID)
		return
	}

	if orderDetails[chatID] != "" {

		// Получите ID группового чата из переменной окружения
		groupChatIDStr := os.Getenv("TELEGRAM_GROUPCHATID")
		groupChatID, err := strconv.ParseInt(groupChatIDStr, 10, 64)

		if err != nil {

			logger.Errorf("Ошибка преобразования ID группового чата: %v", err)
			return

		}

		// Отправляем сообщение в групповой чат.
		sendMessage(bot, groupChatID, orderDetails[chatID])
		delete(orderDetails, groupChatID)

		// Удаление сообщения
		deleteMessages(bot, chatID)

		//отправляем сообщение.
		sendMessage(bot, chatID, "Ваш заказ успешно отправлен при сборе заказа мы Вас уведомим")
		delete(orderDetails, chatID)

		delete(orders, chatID)

		delete(BasketMessages, chatID)

	} else {

		//отправляем сообщение.
		sendMessage(bot, chatID, "Нет данных для отправки")

	}
}

// MainOrdersByUserID Выводим заказы пользователя в телеграмм.
func MainOrdersByUserID(bot *tgbotapi.BotAPI, userID int64) {
	if userID == 0 {
		logger.Errorf("нет данных для обработки userID = %v", userID)
		return
	}

	orders, err := dataSources.GetOrdersByUserID(dataSources.DB, userID, 6)

	if err != nil {
		logger.Errorf("Error getting orders: %v", err)
		return
	}

	if len(orders) > 0 {

		var msgText string
		var currentDeliveryDate string // Объявляем переменную для отслеживания текущей даты доставки
		var currentDateTime string

		for i, order := range orders {

			// Форматирование даты заказа
			// Парсинг строки времени в тип time.Time
			t, err := time.Parse(time.RFC3339, order.DeliveryDate)
			if err != nil {
				logger.Errorf("Ошибка парсинга строки времени: %v", err)
				return
			}

			// Форматирование времени в нужный формат
			formattedDeliveryDate := t.Format("02.01.2006")

			// Группировка по дате доставки
			if formattedDeliveryDate != currentDeliveryDate || order.DateTime != currentDateTime {
				msgText += fmt.Sprintf("Дата заказа: %v\n", order.DateTime)
				msgText += fmt.Sprintf("Дата доставки: %v\n", formattedDeliveryDate)
				msgText += fmt.Sprintf("Заказчик: %v\n", order.FullName)
				msgText += fmt.Sprintf("Телефон для связи: %v\n", order.PhoneNumber)
				msgText += fmt.Sprintf("Адрес доставки: %v\n", order.DeliveryAddress)
				msgText += "Список заказанных товаров :\n"
				currentDeliveryDate = formattedDeliveryDate
				currentDateTime = order.DateTime
			}

			msgText += fmt.Sprintf("%v кол-во: %v цена: %v руб.\n", order.Purchase.Item, order.Purchase.Quantity, order.Purchase.Cost)

			if i == len(orders)-1 || (i+1 < len(orders) && orders[i+1].DateTime != currentDateTime) {
				msgText += fmt.Sprintf("Общая стоимость:  %v руб.\n", order.TotalCost)
				msgText += "------------------------------------------------------------\n"

			}
		}
		//отправляем сообщение
		sendMessage(bot, userID, msgText)

	} else {
		//отправляем сообщение
		sendMessage(bot, userID, "Вы ничего у нас не заказывали")
	}
}

// addOrUpdateOrderField Функция для добавления или обновления конкретного поля заказа.
func addOrUpdateOrderField(chatID int64, fieldName string, value interface{}) error {
	if chatID == 0 {
		logger.Errorf("нет данных для обработки chatID = %v", chatID)
		return errors.New("нет chatID")
	}

	order, exists := orders[chatID]

	if !exists {

		order = structs.OrderDb{}

	}

	val := reflect.ValueOf(&order).Elem()
	fieldVal := val.FieldByName(fieldName)

	if !fieldVal.IsValid() {

		logger.Error("такого поля не существует")
		return errors.New("такого поля не существует")

	}

	if fieldVal.CanSet() {

		fieldVal.Set(reflect.ValueOf(value))

	} else {

		logger.Error("не удалось установить значение поля")
		return errors.New("не удалось установить значение поля")

	}

	orders[chatID] = order
	return nil
}

// clearOrder Функция для очистки данных заказа по chatID.
func clearOrder(chatID int64) {
	if chatID == 0 {
		logger.Errorf("нет данных для обработки chatID = %v", chatID)
		return
	}
	delete(orders, chatID)
}

// getFieldByChatID Функция для получения значения поля по chatID и имени поля.
func getFieldByChatID(chatID int64, fieldName string) (interface{}, error) {
	if chatID == 0 {
		logger.Errorf("нет данных для обработки chatID = %v", chatID)
		return nil, errors.New("нет chatID")
	}
	order, exists := orders[chatID]

	if !exists {
		logger.Errorf("заказ с таким chatID = %v не найден", chatID)
		return nil, errors.New("заказ с таким chatID не найден")
	}

	val := reflect.ValueOf(order)
	fieldVal := val.FieldByName(fieldName)

	if !fieldVal.IsValid() {
		logger.Error("такого поля не существует")
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
	if chatID == 0 || text == "" {
		logger.Errorf("нет данных для обработки chatID = %v text = %v", chatID, text)
		return
	}

	msg := tgbotapi.NewMessage(chatID, text)
	sentMsg, err := bot.Send(msg)

	if err != nil {

		// Получаем имя вызывающей функции и номер строки
		pc, file, line, ok := runtime.Caller(1)

		if ok {
			funcName := runtime.FuncForPC(pc).Name()
			funcName = trimFuncName(funcName) // Укорачиваем имя функции
			file = trimFilePath(file)         // Укорачиваем путь к файлу
			logger.Errorf("не удается отправить сообщение chatID = %v в функции %s (строка %d, файл %s): %v", chatID, funcName, line, file, err)
		} else {
			logger.Errorf("не удается отправить сообщение: %v", err)
		}
		return

	} else {
		// Сохраняем идентификатор отправленного сообщения для последующего удаления
		// Добавляем новое сообщение в мапу
		BasketMessages[chatID] = sentMsg.MessageID
	}
	// Удаляем предыдущие сообщения, если их больше 3
	//deleteMessages(bot, chatID)
}

// trimFuncName Функция для укорачивания полного имени функции.
func trimFuncName(fullFuncName string) string {
	parts := strings.Split(fullFuncName, "/")
	shortName := parts[len(parts)-1]
	return shortName
}

// trimFilePath Функция для укорачивания пути к файлу.
func trimFilePath(fullFilePath string) string {
	parts := strings.Split(fullFilePath, "/")
	if len(parts) > 2 {
		return strings.Join(parts[len(parts)-2:], "/")
	}
	return fullFilePath
}
