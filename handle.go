package main

import (
	"ProjectTelBotMark/dataSources"
	"fmt"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// UpdateMessage Основной обработчик всех данных бота docs.
func UpdateMessage(bot *tgbotapi.BotAPI) {
	go func() {
		u := tgbotapi.NewUpdate(0)
		u.Timeout = 10

		updates := bot.GetUpdatesChan(u)

		for update := range updates {

			if update.CallbackQuery != nil {
				/*
					// Проверяем статус user_black
					if dataSources.UserBlack(dataSources.DB, update.CallbackQuery.Message.Chat.ID) {
						deletePreviousMessages(bot, update.CallbackQuery.Message.Chat.ID, update.CallbackQuery.Message.MessageID)
						//отправляем сообщение
						sendMessage(bot, update.CallbackQuery.Message.Chat.ID, "Вам мы ничего продать не можем.")
						logger.Infof("Юзер из черного списка %v имя %v", update.CallbackQuery.Message.Chat.ID, update.CallbackQuery.Message.From.UserName)
						continue // Прерываем выполнение, если user_black != 0
					}

				*/

				// Обработка CallbackQuery
				handleCallbackQueryData(bot, update)
				continue // Переход к следующему обновлению после обработки CallbackQuery
			}

			if update.Message != nil {
				/*
					// Проверяем статус user_black
					if dataSources.UserBlack(dataSources.DB, update.Message.Chat.ID) {
						deletePreviousMessages(bot, update.Message.Chat.ID, update.Message.MessageID)
						//отправляем сообщение
						sendMessage(bot, update.Message.Chat.ID, "Вам мы ничего продать не можем.")
						logger.Infof("Юзер из черного списка %v имя %v", update.Message.Chat.ID, update.Message.From.UserName)
						continue // Прерываем выполнение, если user_black != 0
					}

				*/

				// В этом месте обрабатываем все сообщения, включая команды
				if update.Message.IsCommand() {
					// Обработка команд пользователя
					handleCommands(bot, update)
				} else {
					// Обработка текстовых сообщений (не команд)
					handleMessageText(bot, update)
				}
				continue
			}

			// Проверяем, что сообщение содержит изменение таймера авто удаление
			if update.Message.MessageAutoDeleteTimerChanged != nil {
				timerChange := update.Message.MessageAutoDeleteTimerChanged
				chatID := update.Message.Chat.ID
				newTimerDuration := timerChange.MessageAutoDeleteTime

				// Логируем изменение таймера авто удаления
				logger.Infof("Auto-delete timer changed in chat %d: new duration is %d seconds", chatID, newTimerDuration)
			}
		}
	}()
}

// handleCommands Обработка команды /start и вывод основного меню docs.
func handleCommands(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	//var err error
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Нет такой команды")
	if update.Message.IsCommand() {
		// Извлекаем команду из сообщения
		command := update.Message.Command()
		// Extract the command from the Message.

		switch command {
		case "start":
			// Отправляем главное меню при старте бота или команде /start
			handleStartCommand(bot, update)
			// Другие команды...
		case "cabbage":

		case "status":
		//msg.Text = "I'm ok."
		default:
			msg.ReplyToMessageID = update.Message.MessageID // Устанавливаем ID сообщения, на которое отвечаем
			if _, err := bot.Send(msg); err != nil {
				logger.Errorf("Ошибка отправки сообщения: %v", err)
				return
			}
		}

	}
}

// handleMessageText Разбор callbackData для определения действия текстовых сообщений.
func handleMessageText(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	if update.Message.Chat.ID == 0 {
		logger.Errorf("нет данных для обработки update.Message.Chat.ID = %v ", update.Message.Chat.ID)
		return
	}
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Нет такой категории продуктов")

	if update.Message.Text != "" {

		// Обработчик нажатий на кнопки меню
		categoryName := update.Message.Text
		// Создаем сообщение с фото категории и встроенной клавиатурой
		_, pureText := SeparateEmojiAndText(categoryName)

		// Проверяем есть у нас такая категория если нет вызываем другое меню
		err := createCategoryMessage(bot, update, pureText)

		if err != nil {

			if DeliveryDateChatID[update.Message.Chat.ID] != 0 {

				// Получаем и выводим поле DeliveryDate по chatID ФИО
				if field, err := getFieldByChatID(update.Message.Chat.ID, "FullName"); err == nil {

					if field == "ФИО:" {
						deleteMessages(bot, update.Message.Chat.ID)
						if pureText == "" || len(pureText) < 3 {

							//Удаляем предыдущие сообщения
							deletePreviousMessages(bot, update.Message.Chat.ID, update.Message.MessageID)

							str := "ФИО: не может быть пустым и имя не может быть короче 3 символов\n"
							FullName := "ФИО:"
							instructions := "Пожалуйста, введите " + FullName + "\n" +
								"Пример:\n" +
								"Иванов Иван Иванович\n"
							str += instructions

							//отправляем сообщение.
							sendMessage(bot, update.Message.Chat.ID, str)
							return
						}
						err = addOrUpdateOrderField(update.Message.Chat.ID, "FullName", pureText)
						if err != nil {
							logger.Errorf("ошибка при добавлении или обновлении поля FullName: %v", err)
							return
						}
						pureText = "Номер телефона:"
						DeliveryDateChatID[update.Message.Chat.ID] = 0
					}
				}

				// Получаем и выводим поле DeliveryAddress по chatID Адрес доставки
				if field, err := getFieldByChatID(update.Message.Chat.ID, "DeliveryAddress"); err == nil {

					if field == "Адрес доставки:" {

						deleteMessages(bot, update.Message.Chat.ID)
						if len(pureText) < 10 {
							deletePreviousMessages(bot, update.Message.Chat.ID, update.Message.MessageID)
							str := "Адрес доставки не может быть пустым\n"
							DeliveryAddress := "Адрес доставки:"
							instructions := "Пожалуйста, введите " + DeliveryAddress + "\n" +
								"Пример:\n" +
								"г. Будущего, ул. Пушкина, д. 12 кв 5\n"
							str += instructions
							//отправляем сообщение
							sendMessage(bot, update.Message.Chat.ID, str)
							return
						}
						err = addOrUpdateOrderField(update.Message.Chat.ID, "DeliveryAddress", pureText)
						if err != nil {
							logger.Errorf("Ошибка при добавлении или обновлении поля DeliveryAddress: %v", err)
							return
						}

						FullName := "ФИО:"
						instructions := "Пожалуйста, введите " + FullName + "\n" +
							"Пример:\n" +
							"Иванов Иван Иванович\n"

						err = addOrUpdateOrderField(update.Message.Chat.ID, "FullName", FullName)
						if err != nil {
							logger.Errorf("Ошибка при добавлении или обновлении поля FullName: %v", err)
							return
						}

						//отправляем сообщение.
						sendMessage(bot, update.Message.Chat.ID, instructions)

						pureText = "ok"
					}
				}

				// Получаем и выводим поле DeliveryAddress по chatID Номер телефона
				if field, err := getFieldByChatID(update.Message.Chat.ID, "PhoneNumber"); err == nil {

					if field == "Номер телефона:" {

						formatted, str := formatPhoneNumber(pureText)
						deleteMessages(bot, update.Message.Chat.ID)

						if str != "" {
							deletePreviousMessages(bot, update.Message.Chat.ID, update.Message.MessageID)
							// Формирование текста с просьбой ввести номер телефона
							PhoneNumber := "Номер телефона:"

							instructions := "\nПожалуйста, введите " + PhoneNumber + "\n" +
								"Пример:\n" +
								"+79345678901\n"
							str += instructions

							//отправляем сообщение
							sendMessage(bot, update.Message.Chat.ID, str)
							return
						}

						err = addOrUpdateOrderField(update.Message.Chat.ID, "PhoneNumber", formatted)
						if err != nil {
							logger.Errorf("Ошибка при добавлении или обновлении поля PhoneNumber %v", err)
							return
						}

						DeliveryAddress := "Адрес доставки:"
						instructions := "Пожалуйста, введите " + DeliveryAddress + "\n" +
							"Пример:\n" +
							"г. Будущего, ул. Пушкина, д. 12 кв 5\n"

						err = addOrUpdateOrderField(update.Message.Chat.ID, "DeliveryAddress", DeliveryAddress)
						if err != nil {
							logger.Errorf("Ошибка при добавлении или обновлении поля DeliveryAddress: %v", err)
							return
						}

						//отправляем сообщение
						sendMessage(bot, update.Message.Chat.ID, instructions)
						pureText = "ok"
					}
				}
			}

			switch pureText {
			// Проверяем, соответствует ли сообщение ожидаемому формату

			case "ok":
				break
			case "Номер телефона:":

				handleDeliveryDetailsMessage(bot, update.Message.Chat.ID)

			case "Мои заказы":

				//Выводим заказы пользователей
				MainOrdersByUserID(bot, update.Message.Chat.ID)

			case "Назад":

				// Возвращаемся в основное меню
				err = createCategoryMessage(bot, update, "Меню")
				if err != nil {
					logger.Errorf("Ошибка создания сообщения %v ", err)
					return
				}

			case "Корзина":

				ShowBasket(bot, update.Message.Chat.ID)

			default:
				msg.ReplyToMessageID = update.Message.MessageID // Устанавливаем ID сообщения, на которое отвечаем
				if _, err = bot.Send(msg); err != nil {
					logger.Errorf("Ошибка отправки сообщения: %v", err)
				}
			}
		}
		deletePreviousMessages(bot, update.Message.Chat.ID, update.Message.MessageID)
	}
}

// handleCallbackQueryData Разбор callbackData для определения действия.
func handleCallbackQueryData(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	if update.CallbackQuery != nil {

		if update.CallbackQuery.Data == "" {
			logger.Errorf("нет данных для обработки update.CallbackQuery.Data = %v ", update.CallbackQuery.Data)
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
				logger.Errorf("Ошибка преобразования числа 1: %v", err)
				return
			}

			num2, er := strconv.Atoi(parts[3])
			if er != nil {
				logger.Errorf("Ошибка преобразования числа 2: %v", er)
				return
			}

			AddToBasket(bot, update, parts[1], num1, num2) // Добавление товара с указанием названия, количества и стоимости
		case "ignore":

			break
		case "confirm":
			if orderDetails[update.CallbackQuery.Message.Chat.ID] != "" {

				err := dataSources.ProcessOrder(orderDetails[update.CallbackQuery.Message.Chat.ID], dataSources.DB, update)
				if err != nil {

					//отправляем сообщение
					sendMessage(bot, update.CallbackQuery.Message.Chat.ID, "Время заказа истекло, пожалуйста добавьте в корзину продукты и оформите заказ повторно.")
					Baskets[update.CallbackQuery.Message.Chat.ID] = nil
					logger.Errorf("Ошибка обработки заказа: %v", err)

				} else {

					//Заказ успешно обработан и добавлен в базу данных.
					GroupMessagesChatID(bot, update.CallbackQuery.Message.Chat.ID)
					Baskets[update.CallbackQuery.Message.Chat.ID] = nil
				}

			} else {

				//отправляем сообщение
				sendMessage(bot, update.CallbackQuery.Message.Chat.ID, "Время заказа истекло, пожалуйста добавьте в корзину продукты и оформите заказ повторно.")
			}
		case "send": // Получение содержимого корзины
			if Baskets[update.CallbackQuery.Message.Chat.ID] != nil {
				// Пример ввода данных заказа
				RequestDeliveryDetails(bot, update.CallbackQuery.Message.Chat.ID)

			} else {

				delete(orders, update.CallbackQuery.Message.Chat.ID)

				//отправляем сообщение
				sendMessage(bot, update.CallbackQuery.Message.Chat.ID, "Время заказа истекло, пожалуйста добавьте в корзину продукты и оформите заказ повторно.")
			}
		case "sends":

			if len(parts) < 2 {
				logger.Errorf("len(parts) = %v", len(parts))
				return
			}

			DeliveryDateChatID[update.CallbackQuery.Message.Chat.ID] = 0

			orders := dataSources.GetByUserID(dataSources.DB, update.CallbackQuery.Message.Chat.ID)

			if orders.PhoneNumber == "" || orders.FullName == "" || orders.DeliveryAddress == "" {
				logger.Errorf("нет данных orders.PhoneNumber = %v, orders.FullName = %v, orders.DeliveryAddress = %v", orders.PhoneNumber, orders.FullName, orders.DeliveryAddress)
			}

			err := addOrUpdateOrderField(update.CallbackQuery.Message.Chat.ID, "FullName", orders.FullName)
			if err != nil {
				logger.Errorf("ошибка при добавлении или обновлении поля FullName: %v", err)
				return
			}

			err = addOrUpdateOrderField(update.CallbackQuery.Message.Chat.ID, "DeliveryAddress", orders.DeliveryAddress)
			if err != nil {
				logger.Errorf("Ошибка при добавлении или обновлении поля DeliveryAddress: %v", err)
				return
			}

			formatted, _ := formatPhoneNumber(orders.PhoneNumber)
			err = addOrUpdateOrderField(update.CallbackQuery.Message.Chat.ID, "PhoneNumber", formatted)
			if err != nil {
				logger.Errorf("Ошибка при добавлении или обновлении поля PhoneNumber %v", err)
				return
			}

			//Удаляем предыдущие сообщения
			deletePreviousMessages(bot, update.CallbackQuery.Message.Chat.ID, update.CallbackQuery.Message.MessageID)

			callback := tgbotapi.NewCallback(update.CallbackQuery.ID, "")
			if _, err := bot.Request(callback); err != nil {
				logger.Error(err)
			}

			handleDeliveryDetailsMessage(bot, update.CallbackQuery.Message.Chat.ID)
		case "phonenumber":

			if len(parts) < 2 {
				logger.Errorf("len(parts) = %v", len(parts))
				return
			}
			//Удаляем предыдущие сообщения
			deletePreviousMessages(bot, update.CallbackQuery.Message.Chat.ID, update.CallbackQuery.Message.MessageID)

			// Формирование текста с просьбой ввести номер телефона
			PhoneNumber := "Номер телефона:"

			instructions := "\nПожалуйста, введите " + PhoneNumber + "\n" +
				"Пример:\n" +
				"+79345678901\n"

			err := addOrUpdateOrderField(update.CallbackQuery.Message.Chat.ID, "PhoneNumber", PhoneNumber)
			if err != nil {
				logger.Errorf("Ошибка при добавлении или обновлении поля DeliveryAddress: %v", err)
				return
			}

			//отправляем сообщение
			sendMessage(bot, update.CallbackQuery.Message.Chat.ID, instructions)
		case "deliverydate":

			if len(parts) < 2 {
				logger.Errorf("len(parts) = %v", len(parts))
				return
			}

			//Удаляем предыдущие сообщения
			deletePreviousMessages(bot, update.CallbackQuery.Message.Chat.ID, update.CallbackQuery.Message.MessageID)

			callback := tgbotapi.NewCallback(update.CallbackQuery.ID, "")
			if _, err := bot.Request(callback); err != nil {
				logger.Error(err)
			}

			if field, err := getFieldByChatID(update.CallbackQuery.Message.Chat.ID, "DeliveryDate"); err == nil {

				if field == "Дату доставки:" {
					//проверяем дату доставки.
					formatted, str := formatDateString(parts[1])
					// если str вернул ошибку отправляем повторно
					if str != "" {

						msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, str+"\nВыберите дату доставки:")
						now := time.Now()
						msg.ReplyMarkup = createCalendarKeyboard(now)

						_, err = bot.Send(msg)
						if err != nil {
							logger.Errorf("не удается вывести содержимое корзины %v", err)
							return
						}
						return
					}

					err = addOrUpdateOrderField(update.CallbackQuery.Message.Chat.ID, "DeliveryDate", formatted)
					if err != nil {
						logger.Errorf("Ошибка при добавлении или обновлении поля DeliveryDate: %v", err)
						return
					}

					PhoneNumber := "Номер телефона:"
					var msgText string

					orders := dataSources.GetByUserID(dataSources.DB, update.CallbackQuery.Message.Chat.ID)

					if orders.PhoneNumber == "" || orders.FullName == "" || orders.DeliveryAddress == "" {

						// Формирование текста с просьбой ввести данные для доставки
						instructions := "Пожалуйста, введите " + PhoneNumber + "\n" +
							"Пример:\n" +
							"+79345678901\n"

						// Добавляем или обновляем поле PhoneNumber
						err = addOrUpdateOrderField(update.CallbackQuery.Message.Chat.ID, "PhoneNumber", PhoneNumber)
						if err != nil {
							logger.Errorf("Ошибка при добавлении или обновлении поля: %v", err)
						}

						// Отправка сообщение
						sendMessage(bot, update.CallbackQuery.Message.Chat.ID, instructions)
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
						logger.Errorf("не удается вывести содержимое корзины %v", err)
						return
					}
				}
			}
		case "nextmonth":
			if len(parts) < 2 {
				logger.Errorf("len(parts) = %v", len(parts))
				return
			}

			month, err := time.Parse("01-2006", parts[1])
			if err != nil {
				logger.Error(err)

			}

			// Заменить старый календарь новым
			edit := tgbotapi.NewEditMessageReplyMarkup(update.CallbackQuery.Message.Chat.ID, update.CallbackQuery.Message.MessageID, createCalendarKeyboard(month))
			_, err = bot.Send(edit)
			if err != nil {
				logger.Error(err)
			}

			callback := tgbotapi.NewCallback(update.CallbackQuery.ID, "")
			if _, err := bot.Request(callback); err != nil {
				logger.Error(err)
			}
		case "delete":
			DeleteBasketContents(bot, update.CallbackQuery.Message.Chat.ID)
		}
	}
}

func handleStartCommand(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	//deletePreviousMessages(bot, update.Message.Chat.ID, update.Message.MessageID)
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Добро пожаловать! Нажмите 'Старт' для начала.")
	msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Старт"),
		),
	)
	_, err := bot.Send(msg)
	if err != nil {
		logger.Errorf("Error sending message:", err)
	}
}
