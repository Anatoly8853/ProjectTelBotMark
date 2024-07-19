package main

import (
	`ProjectTelBotMark/bychat`
	"ProjectTelBotMark/dataSources"
	`ProjectTelBotMark/structs`
	"fmt"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// UpdateMessage Основной обработчик всех данных бота docs.
func UpdateMessage() {
	go func() {
		u := tgbotapi.NewUpdate(0)
		u.Timeout = 10

		updates := bot.GetUpdatesChan(u)

		for update := range updates {

			if update.CallbackQuery != nil {

				// Проверяем статус user_black
				id := dataSources.UserBlack[update.CallbackQuery.Message.Chat.ID]
				if id != 0 {
					deletePreviousMessages(update.CallbackQuery.Message.Chat.ID, update.CallbackQuery.Message.MessageID)
					//отправляем сообщение
					sendMessage(update.CallbackQuery.Message.Chat.ID, "Вам мы ничего продать не можем.")
					app.Log.Infof("Юзер из черного списка %v имя %v", update.CallbackQuery.Message.Chat.ID, update.CallbackQuery.Message.From.UserName)
					continue // Прерываем выполнение, если user_black != 0
				}

				// Обработка CallbackQuery
				handleCallbackQueryData(update)
				continue // Переход к следующему обновлению после обработки CallbackQuery
			}

			if update.Message != nil {

				// Проверяем статус user_black
				id := dataSources.UserBlack[update.Message.Chat.ID]
				if id != 0 {
					deletePreviousMessages(update.Message.Chat.ID, update.Message.MessageID)
					//отправляем сообщение
					sendMessage(update.Message.Chat.ID, "Вам мы ничего продать не можем.")
					app.Log.Infof("Юзер из черного списка %v имя %v", update.Message.Chat.ID, update.Message.From.UserName)
					continue // Прерываем выполнение, если user_black != 0
				}

				// В этом месте обрабатываем все сообщения, включая команды
				if update.Message.IsCommand() {
					// Обработка команд пользователя
					handleCommands(update)
				} else {
					// Обработка текстовых сообщений (не команд)
					handleMessageText(update)
				}
				continue
			}

			// Проверяем, что сообщение содержит изменение таймера авто удаление
			if update.Message.MessageAutoDeleteTimerChanged != nil {
				timerChange := update.Message.MessageAutoDeleteTimerChanged
				chatID := update.Message.Chat.ID
				newTimerDuration := timerChange.MessageAutoDeleteTime

				// Логируем изменение таймера авто удаления
				app.Log.Infof("Auto-delete timer changed in chat %d: new duration is %d seconds", chatID, newTimerDuration)
			}
		}
	}()
}

// handleCommands Обработка команды /start и вывод основного меню docs.
func handleCommands(update tgbotapi.Update) {
	//var err error
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Нет такой команды")
	if update.Message.IsCommand() {
		// Извлекаем команду из сообщения
		command := update.Message.Command()
		// Extract the command from the Message.

		switch command {
		case "start":
			// Отправляем главное меню при старте бота или команде /start
			handleStartCommand(update)
			// Другие команды...
		case "cabbage":

		case "status":
		//msg.Text = "I'm ok."
		default:
			msg.ReplyToMessageID = update.Message.MessageID // Устанавливаем ID сообщения, на которое отвечаем
			if _, err := bot.Send(msg); err != nil {
				app.Log.Errorf("Ошибка отправки сообщения: %v", err)
				return
			}
		}

	}
}

// handleMessageText Разбор MessageText для определения действия текстовых сообщений.
func handleMessageText(update tgbotapi.Update) {
	if update.Message.Chat.ID == 0 {
		app.Log.Errorf("нет данных для обработки update.Message.Chat.ID = %v ", update.Message.Chat.ID)
		return
	}
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Нет такой категории продуктов")

	if update.Message.Text != "" {

		// Обработчик нажатий на кнопки меню
		categoryName := update.Message.Text
		// Создаем сообщение с фото категории и встроенной клавиатурой
		_, pureText := SeparateEmojiAndText(categoryName)

		// Проверяем есть у нас такая категория если нет вызываем другое меню
		err := createCategoryMessage(update, pureText)

		if err != nil {

			if DeliveryDateChatID[update.Message.Chat.ID] != 0 {

				pureText = handleFieldByChatID(update, pureText)

			}

			switch pureText {
			// Проверяем, соответствует ли сообщение ожидаемому формату

			case "ok":
				break
			case "Номер телефона:":

				handleDeliveryDetailsMessage(update.Message.Chat.ID)

			case "Мои заказы":

				photoMessagesDelete(update.Message.Chat.ID)

				keyboardMessagesDelete(update.Message.Chat.ID)

				//Выводим заказы пользователей
				MainOrdersByUserID(update.Message.Chat.ID)

			case "Назад":

				// Возвращаемся в основное меню
				err = createCategoryMessage(update, "Меню")
				if err != nil {
					app.Log.Errorf("Ошибка создания сообщения %v ", err)
					return
				}

			case "Корзина":

				photoMessagesDelete(update.Message.Chat.ID)

				keyboardMessagesDelete(update.Message.Chat.ID)

				ShowBasket(update.Message.Chat.ID)

			default:
				msg.ReplyToMessageID = update.Message.MessageID // Устанавливаем ID сообщения, на которое отвечаем
				if _, err = bot.Send(msg); err != nil {
					app.Log.Errorf("Ошибка отправки сообщения: %v", err)
				}
			}
		}
		deletePreviousMessages(update.Message.Chat.ID, update.Message.MessageID)
	}
}

// handleCallbackQueryData Разбор callbackData для определения действия.
func handleCallbackQueryData(update tgbotapi.Update) {
	if update.CallbackQuery != nil {

		if update.CallbackQuery.Data == "" {
			app.Log.Errorf("нет данных для обработки update.CallbackQuery.Data = %v ", update.CallbackQuery.Data)
			return
		}

		// Разбор callbackData для определения действия
		callbackData := update.CallbackQuery.Data

		// Примеры callbackData: "add_1", "remove", "change_1_3" (изменить количество первого товара на 3)
		parts := strings.Split(callbackData, "_")

		action := parts[0]

		switch action {
		case "add":
			//deletePreviousMessages(bot, update.CallbackQuery.Message.Chat.ID)
			num1, err := strconv.Atoi(parts[2])
			if err != nil {
				app.Log.Errorf("Ошибка преобразования числа 1: %v", err)
				return
			}

			num2, er := strconv.Atoi(parts[3])
			if er != nil {
				app.Log.Errorf("Ошибка преобразования числа 2: %v", er)
				return
			}

			AddToBasket(update, parts[1], num1, num2) // Добавление товара с указанием названия, количества и стоимости
		case "ignore":
			break
		case "confirm": // Оформляем заказ
			if orderDetails[update.CallbackQuery.Message.Chat.ID] != "" {

				err := dataSources.ProcessOrder(app, update)
				if err != nil {

					//отправляем сообщение
					sendMessage(update.CallbackQuery.Message.Chat.ID, "Время заказа истекло, пожалуйста добавьте в корзину продукты и оформите заказ повторно.")
					structs.Baskets[update.CallbackQuery.Message.Chat.ID] = nil
					app.Log.Errorf("Ошибка обработки заказа: %v", err)

				} else {

					//Заказ успешно обработан и добавлен в базу данных.
					GroupMessagesChatID(update.CallbackQuery.Message.Chat.ID)
					structs.Baskets[update.CallbackQuery.Message.Chat.ID] = nil
				}

				clearOrder(update.CallbackQuery.Message.Chat.ID)

			} else {

				//отправляем сообщение
				sendMessage(update.CallbackQuery.Message.Chat.ID, "Время заказа истекло, пожалуйста добавьте в корзину продукты и оформите заказ повторно.")
			}
		case "send": // Получение содержимого корзины
			if structs.Baskets[update.CallbackQuery.Message.Chat.ID] != nil {
				// Пример ввода данных заказа
				RequestDeliveryDetails(update.CallbackQuery.Message.Chat.ID)

			} else {

				delete(bychat.Orders, update.CallbackQuery.Message.Chat.ID)

				//отправляем сообщение
				sendMessage(update.CallbackQuery.Message.Chat.ID, "Время заказа истекло, пожалуйста добавьте в корзину продукты и оформите заказ повторно.")
			}
		case "sends":

			if len(parts) < 2 {
				app.Log.Errorf("len(parts) = %v", len(parts))
				return
			}

			DeliveryDateChatID[update.CallbackQuery.Message.Chat.ID] = 0

			orders := dataSources.GetByUserID(update.CallbackQuery.Message.Chat.ID)

			if orders.PhoneNumber == "" || orders.FullName == "" || orders.DeliveryAddress == "" {
				app.Log.Errorf("нет данных orders.PhoneNumber = %v, orders.FullName = %v, orders.DeliveryAddress = %v", orders.PhoneNumber, orders.FullName, orders.DeliveryAddress)
			}

			err := addOrUpdateOrderField(update.CallbackQuery.Message.Chat.ID, "FullName", orders.FullName)
			if err != nil {
				app.Log.Errorf("ошибка при добавлении или обновлении поля FullName: %v", err)
				return
			}

			err = addOrUpdateOrderField(update.CallbackQuery.Message.Chat.ID, "DeliveryAddress", orders.DeliveryAddress)
			if err != nil {
				app.Log.Errorf("Ошибка при добавлении или обновлении поля DeliveryAddress: %v", err)
				return
			}

			formatted, _ := bychat.FormatPhoneNumber(app, orders.PhoneNumber)
			err = addOrUpdateOrderField(update.CallbackQuery.Message.Chat.ID, "PhoneNumber", formatted)
			if err != nil {
				app.Log.Errorf("Ошибка при добавлении или обновлении поля PhoneNumber %v", err)
				return
			}

			//Удаляем предыдущие сообщения
			deletePreviousMessages(update.CallbackQuery.Message.Chat.ID, update.CallbackQuery.Message.MessageID)

			callback := tgbotapi.NewCallback(update.CallbackQuery.ID, "")
			if _, err = bot.Request(callback); err != nil {
				app.Log.Error(err)
			}

			handleDeliveryDetailsMessage(update.CallbackQuery.Message.Chat.ID)
		case "phonenumber":

			if len(parts) < 2 {
				app.Log.Errorf("len(parts) = %v", len(parts))
				return
			}
			//Удаляем предыдущие сообщения
			deletePreviousMessages(update.CallbackQuery.Message.Chat.ID, update.CallbackQuery.Message.MessageID)

			// Формирование текста с просьбой ввести номер телефона
			PhoneNumber := "Номер телефона:"

			instructions := "\nПожалуйста, введите " + PhoneNumber + "\n" +
				"Пример:\n" +
				"+79345678901\n"

			err := addOrUpdateOrderField(update.CallbackQuery.Message.Chat.ID, "PhoneNumber", PhoneNumber)
			if err != nil {
				app.Log.Errorf("Ошибка при добавлении или обновлении поля DeliveryAddress: %v", err)
				return
			}

			//отправляем сообщение
			sendMessage(update.CallbackQuery.Message.Chat.ID, instructions)
		case "deliverydate":

			if len(parts) < 2 {
				app.Log.Errorf("len(parts) = %v", len(parts))
				return
			}

			//Удаляем предыдущие сообщения
			deletePreviousMessages(update.CallbackQuery.Message.Chat.ID, update.CallbackQuery.Message.MessageID)

			callback := tgbotapi.NewCallback(update.CallbackQuery.ID, "")
			if _, err := bot.Request(callback); err != nil {
				app.Log.Error(err)
			}

			if field, err := bychat.GetFieldByChatID(app, update.CallbackQuery.Message.Chat.ID, "DeliveryDate"); err == nil {

				if field == "Дату доставки:" {
					//проверяем дату доставки.
					formatted, str := bychat.FormatDateString(app, parts[1])
					// если str вернул ошибку отправляем повторно
					if str != "" {

						msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, str+"\nВыберите дату доставки:")
						now := time.Now()
						msg.ReplyMarkup = createCalendarKeyboard(now)

						_, err = bot.Send(msg)
						if err != nil {
							app.Log.Errorf("не удается вывести содержимое корзины %v", err)
							return
						}
						return
					}

					err = addOrUpdateOrderField(update.CallbackQuery.Message.Chat.ID, "DeliveryDate", formatted)
					if err != nil {
						app.Log.Errorf("Ошибка при добавлении или обновлении поля DeliveryDate: %v", err)
						return
					}

					PhoneNumber := "Номер телефона:"
					var msgText string

					orders := dataSources.GetByUserID(update.CallbackQuery.Message.Chat.ID)

					if orders.PhoneNumber == "" || orders.FullName == "" || orders.DeliveryAddress == "" {

						// Формирование текста с просьбой ввести данные для доставки
						instructions := "Пожалуйста, введите " + PhoneNumber + "\n" +
							"Пример:\n" +
							"+79345678901\n"

						// Добавляем или обновляем поле PhoneNumber
						err = addOrUpdateOrderField(update.CallbackQuery.Message.Chat.ID, "PhoneNumber", PhoneNumber)
						if err != nil {
							app.Log.Errorf("Ошибка при добавлении или обновлении поля: %v", err)
						}

						// Отправка сообщение
						sendMessage(update.CallbackQuery.Message.Chat.ID, instructions)
						return

					}

					msgText += "Использовать следующие данные для заказа\n"
					msgText += fmt.Sprintf("Номер телефона: %v\n", orders.PhoneNumber)
					msgText += fmt.Sprintf("ФИО: %v\n", orders.FullName)
					msgText += fmt.Sprintf("Адрес доставки: %v\n", orders.DeliveryAddress)

					msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, msgText)
					btnSend := tgbotapi.NewInlineKeyboardButtonData("Да", "sends_msgText")
					btnDelete := tgbotapi.NewInlineKeyboardButtonData("Нет", "phonenumber_PhoneNumber")

					row := tgbotapi.NewInlineKeyboardRow(btnSend, btnDelete)
					inlineKeyboard := tgbotapi.NewInlineKeyboardMarkup(row)
					msg.ReplyMarkup = inlineKeyboard

					_, err = bot.Send(msg)
					if err != nil {
						app.Log.Errorf("не удается вывести содержимое корзины %v", err)
						return
					}
				}
			}
		case "nextmonth":
			if len(parts) < 2 {
				app.Log.Errorf("len(parts) = %v", len(parts))
				return
			}

			month, err := time.Parse("01-2006", parts[1])
			if err != nil {
				app.Log.Error(err)

			}

			// Заменить старый календарь новым
			edit := tgbotapi.NewEditMessageReplyMarkup(update.CallbackQuery.Message.Chat.ID, update.CallbackQuery.Message.MessageID, createCalendarKeyboard(month))
			_, err = bot.Send(edit)
			if err != nil {
				app.Log.Error(err)
			}

			callback := tgbotapi.NewCallback(update.CallbackQuery.ID, "")
			if _, err = bot.Request(callback); err != nil {
				app.Log.Error(err)
			}
		case "delete": // Очищаем корзину
			DeleteBasketContents(update.CallbackQuery.Message.Chat.ID)
		}
	}
}

//handleStartCommand добро пожаловать! Нажмите 'Старт' для начала.
func handleStartCommand(update tgbotapi.Update) {
	//deletePreviousMessages(bot, update.Message.Chat.ID, update.Message.MessageID)
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Добро пожаловать! Нажмите 'Старт' для начала.")
	msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Старт"),
		),
	)
	_, err := bot.Send(msg)
	if err != nil {
		app.Log.Errorf("Error sending message: = %v", err)
	}
}

func handleFieldByChatID(update tgbotapi.Update, pureText string) string {

	// Получаем и выводим поле DeliveryDate по chatID ФИО
	if field, err := bychat.GetFieldByChatID(app, update.Message.Chat.ID, "FullName"); err == nil {

		if field == "ФИО:" {
			deleteMessages(update.Message.Chat.ID)
			if pureText == "" || len(pureText) < 3 {

				//Удаляем предыдущие сообщения
				deletePreviousMessages(update.Message.Chat.ID, update.Message.MessageID)

				str := "ФИО: не может быть пустым и имя не может быть короче 3 символов\n"
				FullName := "ФИО:"
				instructions := "Пожалуйста, введите " + FullName + "\nПример:\nИванов Иван Иванович\n"
				str += instructions

				//отправляем сообщение.
				sendMessage(update.Message.Chat.ID, str)
				return "ok"
			}

			err = addOrUpdateOrderField(update.Message.Chat.ID, "FullName", pureText)
			if err != nil {
				app.Log.Errorf("ошибка при добавлении или обновлении поля FullName: %v", err)
				return "ok"
			}
			pureText = "Номер телефона:"
			DeliveryDateChatID[update.Message.Chat.ID] = 0

			return pureText
		}
	}

	// Получаем и выводим поле DeliveryAddress по chatID Адрес доставки
	if field, err := bychat.GetFieldByChatID(app, update.Message.Chat.ID, "DeliveryAddress"); err == nil {

		if field == "Адрес доставки:" {

			deleteMessages(update.Message.Chat.ID)
			if len(pureText) < 10 {
				deletePreviousMessages(update.Message.Chat.ID, update.Message.MessageID)
				str := "Адрес доставки не может быть пустым\n"
				DeliveryAddress := "Адрес доставки:"
				instructions := "Пожалуйста, введите " + DeliveryAddress + "\nПример:\nг. Будущего, ул. Пушкина, д. 12 кв 5\n"
				str += instructions
				//отправляем сообщение
				sendMessage(update.Message.Chat.ID, str)
				return "ok"
			}

			err = addOrUpdateOrderField(update.Message.Chat.ID, "DeliveryAddress", pureText)
			if err != nil {
				app.Log.Errorf("Ошибка при добавлении или обновлении поля DeliveryAddress: %v", err)
				return "ok"
			}

			FullName := "ФИО:"
			instructions := "Пожалуйста, введите " + FullName + "\nПример:\nИванов Иван Иванович\n"

			err = addOrUpdateOrderField(update.Message.Chat.ID, "FullName", FullName)
			if err != nil {
				app.Log.Errorf("Ошибка при добавлении или обновлении поля FullName: %v", err)
				return "ok"
			}

			//отправляем сообщение.
			sendMessage(update.Message.Chat.ID, instructions)

			pureText = "ok"
			return "ok"
		}
	}

	// Получаем и выводим поле DeliveryAddress по chatID Номер телефона
	if field, err := bychat.GetFieldByChatID(app, update.Message.Chat.ID, "PhoneNumber"); err == nil {

		if field == "Номер телефона:" {

			formatted, str := bychat.FormatPhoneNumber(app, pureText)
			deleteMessages(update.Message.Chat.ID)

			if str != "" {
				deletePreviousMessages(update.Message.Chat.ID, update.Message.MessageID)
				// Формирование текста с просьбой ввести номер телефона
				PhoneNumber := "Номер телефона:"

				instructions := "\nПожалуйста, введите " + PhoneNumber + "\n" +
					"Пример:\n" +
					"+79345678901\n"
				str += instructions

				//отправляем сообщение
				sendMessage(update.Message.Chat.ID, str)
				return "ok"
			}

			err = addOrUpdateOrderField(update.Message.Chat.ID, "PhoneNumber", formatted)
			if err != nil {
				app.Log.Errorf("Ошибка при добавлении или обновлении поля PhoneNumber %v", err)
				return "ok"
			}

			DeliveryAddress := "Адрес доставки:"
			instructions := "Пожалуйста, введите " + DeliveryAddress + "\n" +
				"Пример:\n" +
				"г. Будущего, ул. Пушкина, д. 12 кв 5\n"

			err = addOrUpdateOrderField(update.Message.Chat.ID, "DeliveryAddress", DeliveryAddress)
			if err != nil {
				app.Log.Errorf("Ошибка при добавлении или обновлении поля DeliveryAddress: %v", err)
				return "ok"
			}

			//отправляем сообщение
			sendMessage(update.Message.Chat.ID, instructions)
			pureText = "ok"
			return "ok"
		}
	}

	return pureText
}

// handleDeliveryDetailsMessage анализируем и извлекаем данные из сообщения.
// Объединяем данные заказа с содержимым корзины.
func handleDeliveryDetailsMessage(chatID int64) {
	if chatID == 0 {
		app.Log.Errorf("нет данных для обработки chatID = %v", chatID)
		return
	}

	var MsgText string

	//"Номер телефона: [ваш номер]\n" +
	if field, err := bychat.GetFieldByChatID(app, chatID, "PhoneNumber"); err == nil {
		str, _ := field.(string)
		MsgText += "Номер телефона: " + str + "\n"
	}

	//"Адрес доставки: [ваш адрес]\n" +
	if field, err := bychat.GetFieldByChatID(app, chatID, "DeliveryAddress"); err == nil {
		str, _ := field.(string)
		MsgText += "Адрес доставки: " + str + "\n"
	}

	//"Дата доставки: [желаемая дата]\n" +
	if field, err := bychat.GetFieldByChatID(app, chatID, "DeliveryDate"); err == nil {
		str, _ := field.(string)
		MsgText += "Дата доставки: " + str + "\n"
	}

	//"ФИО: [ваше имя и фамилия]\n\n" +
	if field, err := bychat.GetFieldByChatID(app, chatID, "FullName"); err == nil {
		str, _ := field.(string)
		MsgText += "ФИО: " + str + "\n"
	}

	// Объединяем данные заказа с содержимым корзины
	orderDetails[chatID] = "Детали заказа:\n" + MsgText // Сообщение с деталями заказа

	// Получение содержимого корзины
	purchases := structs.Baskets[chatID]

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

	//TODO
	// Очищаем данные заказа
	//clearOrder(chatID)

	orderDetails[chatID] += MsgTexts

	// Отправляем пользователю подтверждение с деталями заказа и кнопкой "Заказать"
	btnOrder := tgbotapi.NewInlineKeyboardButtonData("Заказать", "confirm_order")
	inlineKeyboard := tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(btnOrder))

	confirmMsg := tgbotapi.NewMessage(chatID, orderDetails[chatID])
	confirmMsg.ReplyMarkup = inlineKeyboard

	if sentMsg, err := bot.Send(confirmMsg); err != nil {
		app.Log.Errorf("%v", err)
		return
	} else {
		// Сохраняем идентификатор отправленного сообщения для последующего удаления
		BasketMessages[chatID] = sentMsg.MessageID
	}
}
