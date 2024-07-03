package main

import (
	`ProjectTelBotMark/bychat`
	"ProjectTelBotMark/dataSources"
	"ProjectTelBotMark/structs"
	"errors"
	"fmt"
	"os"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Baskets карта для хранения данных о покупках для каждого пользователя.
//var Baskets = make(map[int64][]structs.Purchase)

var DeliveryDateChatID = make(map[int64]int)

// orderDetails содержание корзины.
var orderDetails = make(map[int64]string)

// orders Map для хранения данных по chatID.
//var orders = make(map[int64]structs.OrderDb)

// AddToBasket добавление товаров в корзину.
func AddToBasket(update tgbotapi.Update, item string, quantity int, cost int) {
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
	structs.Baskets[chatID] = append(structs.Baskets[chatID], purchase)

	// Подсчет общей стоимости в корзине
	totalCost := 0
	for _, p := range structs.Baskets[chatID] {
		totalCost += p.Cost * p.Quantity
	}

	// Отправка всплывающего сообщения пользователю
	popupText := fmt.Sprintf("Добавлено в корзину:\n %s, кол-во: %d, цена: %d руб.\n Общая стоимость: %d руб", item, quantity, cost, totalCost)

	SendPopupMessage(update, popupText)
}

// SendPopupMessage Отправка всплывающего сообщения.
func SendPopupMessage(update tgbotapi.Update, popupText string) {
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
func ShowBasket(chatID int64) {
	purchases := structs.Baskets[chatID]
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

// RequestDeliveryDetails Пример ввода данных заказа дата доставки.
func RequestDeliveryDetails(chatID int64) {
	if chatID == 0 {
		logger.Errorf("нет данных для обработки chatID = %v", chatID)
		return
	}

	// Удаление сообщения
	deleteMessages(chatID)

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

// GroupMessagesChatID отправляем заказ.
func GroupMessagesChatID(chatID int64) {
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
		sendMessage(groupChatID, orderDetails[chatID])
		delete(orderDetails, groupChatID)

		// Удаление сообщения
		deleteMessages(chatID)

		//отправляем сообщение.
		sendMessage(chatID, "Ваш заказ успешно отправлен при сборе заказа мы Вас уведомим")
		ScheduleOnce(chatID)

	} else {

		//отправляем сообщение.
		sendMessage(chatID, "Нет данных для отправки")

	}
}

// MainOrdersByUserID Выводим заказы пользователя в телеграмм.
func MainOrdersByUserID(userID int64) {
	if userID == 0 {
		logger.Errorf("нет данных для обработки userID = %v", userID)
		return
	}

	orders, err := dataSources.GetOrdersByUserID(userID, 6)

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
		sendMessage(userID, msgText)

	} else {
		//отправляем сообщение
		sendMessage(userID, "Вы ничего у нас не заказывали")
	}
}

// addOrUpdateOrderField для добавления или обновления конкретного поля заказа.
func addOrUpdateOrderField(chatID int64, fieldName string, value interface{}) error {
	if chatID == 0 {
		logger.Errorf("нет данных для обработки chatID = %v", chatID)
		return errors.New("нет chatID")
	}

	order, exists := bychat.Orders[chatID]

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

	bychat.Orders[chatID] = order
	return nil
}

// sendMessage Функция для отправки сообщения и добавления его в BasketMessages
func sendMessage(chatID int64, text string) {
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

func ScheduleOnce(chatID int64) {
	go func() {
		time.Sleep(1 * time.Minute)
		// Удаление сообщения
		deleteMessages(chatID)

		for chatIDs, messageIDs := range messagesToDelete {
			for _, messageIDa := range messageIDs {
				deleteMsg := tgbotapi.NewDeleteMessage(chatIDs, messageIDa)
				if _, err := bot.Request(deleteMsg); err != nil {
					//log.Printf("Ошибка при удалении сообщения: %v", err)
					// Не используйте log.Panic, чтобы избежать прерывания работы бота из-за одной ошибки
				}

			}
		}

		delete(orderDetails, chatID)

		delete(bychat.Orders, chatID)

		delete(BasketMessages, chatID)

		// Возвращаемся в основное меню
		msg := tgbotapi.NewMessage(chatID, "Добро пожаловать! Нажмите 'Старт' для начала.")
		msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton("Старт"),
			),
		)
		_, err := bot.Send(msg)
		if err != nil {
			logger.Errorf("Error sending message: = %v", err)
		}

	}()
}
