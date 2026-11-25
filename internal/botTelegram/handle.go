package botTelegram

import (
	"ProjectTelBotMark/config"
	"ProjectTelBotMark/internal/bychat"
	"ProjectTelBotMark/internal/dataSources"
	"ProjectTelBotMark/internal/structs"
	"fmt"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var DeliveryAddress = "Адрес доставки:"
var PhoneNumber = "Номер телефона:"
var FullName = "ФИО:"

// UpdateMessage Основной обработчик всех данных бота docs.
func (m *MessageService) UpdateMessage(cfg config.Config) {
	go func() {
		u := tgbotapi.NewUpdate(0)
		u.Timeout = 10

		updates := m.bot.GetUpdatesChan(u)

		for update := range updates {
			if update.CallbackQuery != nil {
				if update.CallbackQuery.Message == nil || update.CallbackQuery.Message.Chat == nil {
					m.app.Log.Error("UpdateMessage: callback не содержит Message или Chat")
					if update.CallbackQuery.ID != "" {
						if _, err := m.bot.Request(tgbotapi.NewCallback(update.CallbackQuery.ID, "Некорректный запрос")); err != nil {
							m.app.Log.Errorf("UpdateMessage: ошибка ответа на некорректный callback: %v", err)
						}
					}
					continue
				}
				// Проверяем статус user_black
				userChatID := update.CallbackQuery.Message.Chat.ID
				id := dataSources.GetUserBlack(userChatID)
				if id != 0 {
					m.deletePreviousMessages(userChatID, update.CallbackQuery.Message.MessageID)
					//отправляем сообщение
					m.sendMessage(userChatID, "Вам мы ничего продать не можем.")
					userName := ""
					if update.CallbackQuery.Message.From != nil {
						userName = update.CallbackQuery.Message.From.UserName
					}
					m.app.Log.Infof("Юзер из черного списка %v имя %v", userChatID, userName)
					continue // Прерываем выполнение, если user_black != 0
				}

				// Очищаем старый экран только при реальном переходе на другой экран.
				// Кнопки добавления товара, +/- корзины и перелистывания календаря остаются на месте.
				if m.callbackOpensNewScreen(update.CallbackQuery) {
					m.deletePreviousMessages(userChatID, update.CallbackQuery.Message.MessageID)
				}

				// Обработка CallbackQuery
				m.handleCallbackQueryData(cfg, update)
				continue // Переход к следующему обновлению после обработки CallbackQuery
			}

			if update.Message != nil {
				// Проверяем статус user_black
				id := dataSources.GetUserBlack(update.Message.Chat.ID)
				if id != 0 {
					m.deletePreviousMessages(update.Message.Chat.ID, update.Message.MessageID)
					//отправляем сообщение
					m.sendMessage(update.Message.Chat.ID, "Вам мы ничего продать не можем.")
					m.app.Log.Infof("Юзер из черного списка %v имя %v", update.Message.Chat.ID, update.Message.From.UserName)
					continue // Прерываем выполнение, если user_black != 0
				}

				// Удаляем предыдущий экран и входящее сообщение до формирования нового ответа.
				// Команда /start отдельно создаёт приветствие, которое сохраняется между переходами.
				m.deletePreviousMessages(update.Message.Chat.ID, update.Message.MessageID)

				// В этом месте обрабатываем все сообщения, включая команды
				if update.Message.IsCommand() {
					// Обработка команд пользователя
					m.handleCommands(update)
				} else {
					// Обработка текстовых сообщений (не команд)
					m.handleMessageText(update)
				}
				continue
			}

			// Проверяем, что сообщение содержит изменение таймера авто удаление
			if update.Message.MessageAutoDeleteTimerChanged != nil {
				timerChange := update.Message.MessageAutoDeleteTimerChanged
				chatID := update.Message.Chat.ID
				newTimerDuration := timerChange.MessageAutoDeleteTime
				// Логируем изменение таймера авто удаления
				m.app.Log.Infof("Auto-delete timer changed in chat %d: new duration is %d seconds", chatID, newTimerDuration)
			}
		}
	}()
}

// handleCommands Обработка команды /start и вывод основного меню docs.
func (m *MessageService) handleCommands(update tgbotapi.Update) {
	//var err error
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Нет такой команды")
	if update.Message.IsCommand() {
		// Извлекаем команду из сообщения
		command := update.Message.Command()
		// Extract the command from the Message.

		switch command {
		case "start":
			// Отправляем главное меню при старте бота или команде /start
			m.handleStartCommand(update)
			// Другие команды...
		case "cabbage":

		case "status":
		//msg.Text = "I'm ok."
		default:
			msg.ReplyToMessageID = update.Message.MessageID // Устанавливаем ID сообщения, на которое отвечаем
			sent, err := m.bot.Send(msg)
			if err != nil {
				m.app.Log.Errorf("Ошибка отправки сообщения: %v", err)
				return
			}
			m.trackMessage(update.Message.Chat.ID, sent.MessageID)
		}

	}
}

// handleStartCommand добро пожаловать! Нажмите 'Старт' для начала.
func (m *MessageService) handleStartCommand(update tgbotapi.Update) {
	// Входящее сообщение /start уже удалено централизованной очисткой в UpdateMessage.
	// Если приветствие уже есть, не создаём его повторно.
	basketMu.Lock()
	greetingID := greetingMessagesToKeep[update.Message.Chat.ID]
	basketMu.Unlock()
	if greetingID != 0 {
		m.app.Log.Debugf("handleStartCommand: приветствие уже существует: chatID=%d, messageID=%d", update.Message.Chat.ID, greetingID)
		return
	}

	// Отправляем приветственное сообщение и сохраняем его ID отдельно от остальных сообщений.
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("Добро пожаловать %s! Нажмите 'Старт' для начала.", update.Message.Chat.UserName))
	msg.ReplyMarkup = createStartKeyboard()
	sent, err := m.bot.Send(msg)
	if err != nil {
		m.app.Log.Errorf("Ошибка при отправке сообщения: %v", err)
		return
	}
	basketMu.Lock()
	greetingMessagesToKeep[update.Message.Chat.ID] = sent.MessageID
	basketMu.Unlock()
	m.app.Log.Infof("handleStartCommand: приветствие отправлено: chatID=%d, messageID=%d", update.Message.Chat.ID, sent.MessageID)
}

// handleMessageText Разбор MessageText для определения действия текстовых сообщений.
func (m *MessageService) handleMessageText(update tgbotapi.Update) {
	if update.Message.Chat.ID == 0 {
		m.app.Log.Errorf("нет данных для обработки update.Message.Chat.ID = %v ", update.Message.Chat.ID)
		return
	}
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Нет такой категории продуктов")

	if update.Message.Text != "" {
		chatID := update.Message.Chat.ID
		commentPendingMu.Lock()
		waitingForComment := commentPending[chatID]
		if waitingForComment {
			delete(commentPending, chatID)
		}
		commentPendingMu.Unlock()

		if waitingForComment {
			comment := strings.TrimSpace(update.Message.Text)
			if strings.EqualFold(comment, "нет") || comment == "—" || comment == "-" {
				comment = ""
			}
			if err := m.addOrUpdateOrderField(chatID, "Comment", comment); err != nil {
				m.app.Log.Errorf("handleMessageText: ошибка сохранения комментария: chatID=%d, ошибка=%v", chatID, err)
				commentPendingMu.Lock()
				commentPending[chatID] = true
				commentPendingMu.Unlock()
				m.sendMessage(chatID, "Не удалось сохранить комментарий. Попробуйте ещё раз.")
				return
			}
			m.handleDeliveryDetailsMessage(chatID)
			return
		}

		// Обработчик нажатий на кнопки меню
		categoryName := update.Message.Text
		_, pureText := SeparateEmojiAndText(categoryName)
		// Проверяем есть у нас такая категория если нет вызываем другое меню
		err := m.createCategoryMessage(update, pureText)

		if err != nil {
			deliveryDateMu.RLock()
			deliveryDateActive := DeliveryDateChatID[update.Message.Chat.ID] != 0
			deliveryDateMu.RUnlock()
			if deliveryDateActive {
				pureText = m.handleFieldByChatID(update, pureText)
			}
			switch pureText {
			// Проверяем, соответствует ли сообщение ожидаемому формату
			case "ok":
				break

			case PhoneNumber:
				m.requestOrderComment(update.Message.Chat.ID)

			case "Мои заказы":
				m.photoMessagesDelete(update.Message.Chat.ID)
				m.keyboardMessagesDelete(update.Message.Chat.ID)
				//Выводим заказы пользователей
				m.MainOrdersByUserID(update.Message.Chat.ID)

			case "Назад":
				// Возвращаемся в основное меню
				err = m.createCategoryMessage(update, "Старт")
				if err != nil {
					m.app.Log.Errorf("Ошибка создания сообщения %v ", err)
					return
				}

			case "Стр 1":
				// Возвращаемся в основное меню
				err = m.createCategoryMessage(update, "Старт")
				if err != nil {
					m.app.Log.Errorf("Ошибка создания сообщения %v ", err)
					return
				}

			case "Корзина":
				m.photoMessagesDelete(update.Message.Chat.ID)
				m.keyboardMessagesDelete(update.Message.Chat.ID)
				m.ShowBasket(update.Message.Chat.ID)

			default:
				msg.ReplyToMessageID = update.Message.MessageID // Устанавливаем ID сообщения, на которое отвечаем
				sent, sendErr := m.bot.Send(msg)
				if sendErr != nil {
					m.app.Log.Errorf("Ошибка отправки сообщения: %v", sendErr)
				} else {
					m.trackMessage(update.Message.Chat.ID, sent.MessageID)
				}
			}
		}
	}
}

// handleCallbackQueryData разбирает callbackData и определяет действие.
func (m *MessageService) handleCallbackQueryData(cfg config.Config, update tgbotapi.Update) {
	if update.CallbackQuery == nil {
		m.app.Log.Error("handleCallbackQueryData: отсутствует CallbackQuery")
		return
	}

	callbackQuery := update.CallbackQuery

	if callbackQuery.ID == "" {
		m.app.Log.Error("handleCallbackQueryData: пустой CallbackQuery.ID")
		return
	}

	if callbackQuery.Data == "" {
		m.app.Log.Error("handleCallbackQueryData: пустой CallbackQuery.Data")
		return
	}

	if callbackQuery.Message == nil || callbackQuery.Message.Chat == nil {
		m.app.Log.Errorf(
			"handleCallbackQueryData: отсутствует сообщение или Chat, callback=%q",
			callbackQuery.Data,
		)
		return
	}

	chatID := callbackQuery.Message.Chat.ID
	callbackData := callbackQuery.Data
	parts := strings.Split(callbackData, "_")
	action := parts[0]

	// Подтверждение callback без всплывающего сообщения.
	answerCallback := func(text string) {
		_, err := m.bot.Request(
			tgbotapi.NewCallback(callbackQuery.ID, text),
		)
		if err != nil {
			m.app.Log.Errorf(
				"handleCallbackQueryData: ошибка ответа на callback: chatID=%d, callback=%q, ошибка=%v",
				chatID,
				callbackData,
				err,
			)
		}
	}

	switch action {

	// ---------------------------------------------------------
	// КАТАЛОГ
	// ---------------------------------------------------------

	case "cat":
		if !strings.HasPrefix(callbackData, "cat_") {
			err := fmt.Errorf("неверный формат callback категории: chatID=%d, callback=%q", chatID, callbackData)
			m.app.Log.Error("handleCallbackQueryData", err.Error())
			m.SendPopupMessage(update, "Некорректная команда")
			return
		}

		categoryName := strings.TrimPrefix(callbackData, "cat_")
		categoryName = strings.Trim(categoryName, `"`)

		if strings.TrimSpace(categoryName) == "" {
			err := fmt.Errorf("пустое название категории: chatID=%d, callback=%q", chatID, callbackData)
			m.app.Log.Error("handleCallbackQueryData", err.Error())
			m.SendPopupMessage(update, "Не выбрана категория")
			return
		}

		if categoryName == "Корзина" {
			basketMu.Lock()
			purchases := len(structs.Baskets[chatID])
			basketMu.Unlock()
			if purchases == 0 {
				m.SendPopupMessage(update, "🛒 Корзина пуста")
				return
			}
		}

		answerCallback("")

		switch categoryName {
		case "Корзина":
			m.photoMessagesDelete(chatID)
			m.keyboardMessagesDelete(chatID)
			m.ShowBasket(chatID)

		case "Мои заказы":
			m.photoMessagesDelete(chatID)
			m.keyboardMessagesDelete(chatID)
			m.MainOrdersByUserID(chatID)

		case "Избранное":
			m.photoMessagesDelete(chatID)
			m.keyboardMessagesDelete(chatID)
			m.MainFavoritesByUserID(chatID)

		case "Назад", "Стр 1":
			categoryUpdate := update
			categoryUpdate.Message = &tgbotapi.Message{
				Chat: callbackQuery.Message.Chat,
			}

			if err := m.createCategoryMessage(categoryUpdate, "Старт"); err != nil {
				m.app.Log.Errorf(
					"handleCallbackQueryData: ошибка возврата в главное меню: chatID=%d, ошибка=%v",
					chatID,
					err,
				)
			}

		default:
			categoryUpdate := update
			categoryUpdate.Message = &tgbotapi.Message{
				Chat: callbackQuery.Message.Chat,
			}

			if err := m.createCategoryMessage(categoryUpdate, categoryName); err != nil {
				m.app.Log.Errorf(
					"handleCallbackQueryData: ошибка открытия категории: chatID=%d, категория=%q, ошибка=%v",
					chatID,
					categoryName,
					err,
				)
			}
		}

	// ---------------------------------------------------------
	// ДОБАВЛЕНИЕ ТОВАРА
	// ---------------------------------------------------------

	case "add":
		// Последние два параметра — количество и цена; название может содержать "_".
		if len(parts) < 4 {
			err := fmt.Errorf(
				"неверный формат callback добавления товара: callback=%q, количество частей=%d",
				callbackData,
				len(parts),
			)
			m.app.Log.Error("handleCallbackQueryData", err.Error())
			m.SendPopupMessage(update, "Некорректные данные товара")
			return
		}

		item := strings.Join(parts[1:len(parts)-2], "_")
		if strings.TrimSpace(item) == "" {
			err := fmt.Errorf(
				"пустое название товара: chatID=%d, callback=%q",
				chatID,
				callbackData,
			)
			m.app.Log.Error("handleCallbackQueryData", err.Error())
			m.SendPopupMessage(update, "Не указано название товара")
			return
		}

		// Проверяем количество.
		quantity, err := strconv.Atoi(parts[len(parts)-2])
		if err != nil {
			m.app.Log.Errorf(
				"handleCallbackQueryData: ошибка преобразования количества: chatID=%d, товар=%q, значение=%q, ошибка=%v",
				chatID,
				item,
				parts[len(parts)-2],
				err,
			)
			m.SendPopupMessage(update, "Некорректное количество товара")
			return
		}

		if quantity <= 0 {
			err := fmt.Errorf(
				"количество товара должно быть положительным: chatID=%d, товар=%q, quantity=%d",
				chatID,
				item,
				quantity,
			)
			m.app.Log.Error("handleCallbackQueryData", err.Error())
			m.SendPopupMessage(update, "Количество должно быть больше нуля")
			return
		}

		// Проверяем цену.
		cost, err := strconv.Atoi(parts[len(parts)-1])
		if err != nil {
			m.app.Log.Errorf(
				"handleCallbackQueryData: ошибка преобразования цены: chatID=%d, товар=%q, значение=%q, ошибка=%v",
				chatID,
				item,
				parts[len(parts)-1],
				err,
			)
			m.SendPopupMessage(update, "Некорректная цена товара")
			return
		}

		if cost <= 0 {
			err := fmt.Errorf(
				"цена товара должна быть положительной: chatID=%d, товар=%q, cost=%d",
				chatID,
				item,
				cost,
			)
			m.app.Log.Error("handleCallbackQueryData", err.Error())
			m.SendPopupMessage(update, "Цена товара должна быть больше нуля")
			return
		}

		m.AddToBasket(update, item, quantity, cost)

	// ---------------------------------------------------------
	// ИЗМЕНЕНИЕ КОЛИЧЕСТВА В КОРЗИНЕ
	// ---------------------------------------------------------

	case "basket":
		if len(parts) != 3 {
			m.app.Log.Errorf(
				"handleCallbackQueryData: неверный формат изменения корзины: callback=%q",
				callbackData,
			)
			m.SendPopupMessage(update, "Некорректная кнопка")
			return
		}

		itemID, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil || itemID <= 0 {
			m.app.Log.Errorf(
				"handleCallbackQueryData: некорректный ID позиции: chatID=%d, значение=%q, ошибка=%v",
				chatID,
				parts[2],
				err,
			)
			m.SendPopupMessage(update, "Некорректный номер товара")
			return
		}

		var delta int

		switch parts[1] {
		case "plus":
			delta = 1

		case "minus":
			delta = -1

		default:
			m.app.Log.Errorf(
				"handleCallbackQueryData: неизвестное действие корзины: chatID=%d, действие=%q",
				chatID,
				parts[1],
			)
			m.SendPopupMessage(update, "Неизвестное действие")
			return
		}

		purchases, err := m.changeBasketQuantity(chatID, itemID, delta)
		if err != nil {
			// changeBasketQuantity уже записала подробную ошибку в лог.
			m.SendPopupMessage(update, "Не удалось изменить количество")
			return
		}

		m.updateBasketMessage(update, purchases)

	// ---------------------------------------------------------
	// КНОПКА БЕЗ ДЕЙСТВИЯ
	// ---------------------------------------------------------

	case "ignore":
		answerCallback("")

	// ---------------------------------------------------------
	// ПОДТВЕРЖДЕНИЕ ЗАКАЗА
	// ---------------------------------------------------------

	case "confirm":
		if len(parts) != 2 || parts[1] != "order" {
			err := fmt.Errorf("неверный формат callback подтверждения заказа: chatID=%d, callback=%q", chatID, callbackData)
			m.app.Log.Error("handleCallbackQueryData", err.Error())
			m.SendPopupMessage(update, "Некорректная команда подтверждения")
			return
		}
		if !reserveOrderProcessing(chatID) {
			m.SendPopupMessage(update, "Заказ уже обрабатывается")
			return
		}
		defer releaseOrderProcessing(chatID)

		orderDetailsMu.RLock()
		hasOrderDetails := orderDetails[chatID] != ""
		orderDetailsMu.RUnlock()
		if !hasOrderDetails {
			m.app.Log.Warnf(
				"handleCallbackQueryData: время оформления заказа истекло: chatID=%d",
				chatID,
			)

			m.sendMessage(
				chatID,
				"Время заказа истекло, пожалуйста добавьте в корзину продукты и оформите заказ повторно.",
			)

			answerCallback("Время оформления заказа истекло")
			return
		}

		// Получаем копию корзины под блокировкой.
		basketMu.Lock()

		purchases := append(
			[]structs.Purchase(nil),
			structs.Baskets[chatID]...,
		)

		basketMu.Unlock()

		if len(purchases) == 0 {
			err := fmt.Errorf(
				"попытка подтвердить заказ с пустой корзиной: chatID=%d",
				chatID,
			)

			m.app.Log.Error("handleCallbackQueryData", err.Error())

			m.sendMessage(
				chatID,
				"Корзина пуста. Добавьте товары и оформите заказ повторно.",
			)

			m.clearOrder(chatID)
			answerCallback("Корзина пуста")
			return
		}

		// Сохраняем заказ в базе данных.
		if err := m.repo.ProcessOrder(update, purchases); err != nil {
			m.app.Log.Errorf(
				"handleCallbackQueryData: ошибка обработки заказа: chatID=%d, ошибка=%v",
				chatID,
				err,
			)

			m.sendMessage(
				chatID,
				"Не удалось сохранить заказ. Ваши товары остались в корзине. Попробуйте оформить заказ ещё раз.",
			)

			// Корзину не удаляем — пользователь сможет повторить попытку.
			m.clearOrder(chatID)

			answerCallback("Не удалось сохранить заказ")
			return
		}

		// Заказ успешно сохранён.
		m.GroupMessagesChatID(cfg, chatID)

		// Удаляем корзину только после успешного сохранения.
		basketMu.Lock()
		delete(structs.Baskets, chatID)
		basketMu.Unlock()

		m.clearOrder(chatID)

		answerCallback("Заказ успешно оформлен")

	// ---------------------------------------------------------
	// НАЧАЛО ОФОРМЛЕНИЯ ЗАКАЗА
	// ---------------------------------------------------------

	case "send":
		basketMu.Lock()
		purchases := append(
			[]structs.Purchase(nil),
			structs.Baskets[chatID]...,
		)
		basketMu.Unlock()

		if len(purchases) > 0 {
			answerCallback("")
			m.RequestDeliveryDetails(chatID)
		} else {
			m.app.Log.Warnf(
				"handleCallbackQueryData: попытка оформить пустую корзину: chatID=%d",
				chatID,
			)

			bychat.OrdersMu.Lock()
			delete(bychat.Orders, chatID)
			bychat.OrdersMu.Unlock()

			m.sendMessage(
				chatID,
				"Время заказа истекло, пожалуйста добавьте в корзину продукты и оформите заказ повторно.",
			)

			answerCallback("Корзина пуста")
		}

	// ---------------------------------------------------------
	// ПОВТОРНОЕ ИСПОЛЬЗОВАНИЕ ДАННЫХ ЗАКАЗА
	// ---------------------------------------------------------

	case "sends":
		if len(parts) != 2 || parts[1] != "msgText" {
			err := fmt.Errorf(
				"неверный формат callback повторного использования данных: chatID=%d, callback=%q",
				chatID,
				callbackData,
			)
			m.app.Log.Error("handleCallbackQueryData", err.Error())
			m.SendPopupMessage(update, "Некорректные данные")
			return
		}

		deliveryDateMu.Lock()
		DeliveryDateChatID[chatID] = 0
		deliveryDateMu.Unlock()

		orders := m.repo.GetByUserID(chatID)

		if orders.PhoneNumber == "" ||
			orders.FullName == "" ||
			orders.DeliveryAddress == "" {

			err := fmt.Errorf(
				"отсутствуют данные заказа: chatID=%d, телефон=%q, ФИО=%q, адрес=%q",
				chatID,
				orders.PhoneNumber,
				orders.FullName,
				orders.DeliveryAddress,
			)

			m.app.Log.Error("handleCallbackQueryData", err.Error())
			m.SendPopupMessage(update, "Не удалось использовать предыдущие данные. Проверьте телефон, ФИО и адрес.")
			return
		}

		err := m.addOrUpdateOrderField(chatID, "FullName", orders.FullName)
		if err != nil {
			m.app.Log.Errorf(
				"handleCallbackQueryData: ошибка сохранения ФИО: chatID=%d, ошибка=%v",
				chatID,
				err,
			)
			return
		}

		err = m.addOrUpdateOrderField(chatID, "DeliveryAddress", orders.DeliveryAddress)
		if err != nil {
			m.app.Log.Errorf(
				"handleCallbackQueryData: ошибка сохранения адреса: chatID=%d, ошибка=%v",
				chatID,
				err,
			)
			return
		}

		formatted, _ := bychat.FormatPhoneNumber(m.app, orders.PhoneNumber)
		orders.PhoneNumber = formatted

		err = m.addOrUpdateOrderField(chatID, "PhoneNumber", formatted)
		if err != nil {
			m.app.Log.Errorf(
				"handleCallbackQueryData: ошибка сохранения телефона: chatID=%d, ошибка=%v",
				chatID,
				err,
			)
			return
		}


		answerCallback("")

		m.requestOrderComment(chatID)

	// ---------------------------------------------------------
	// ВВОД НОМЕРА ТЕЛЕФОНА
	// ---------------------------------------------------------

	case "phonenumber":
		if len(parts) != 2 || parts[1] != "PhoneNumber" {
			err := fmt.Errorf(
				"неверный формат callback ввода телефона: chatID=%d, callback=%q",
				chatID,
				callbackData,
			)
			m.app.Log.Error("handleCallbackQueryData", err.Error())
			m.SendPopupMessage(update, "Некорректные данные")
			return
		}


		instructions := "\nПожалуйста, введите " + PhoneNumber + "\n" +
			"Пример:\n" +
			"+79345678901\n"

		err := m.addOrUpdateOrderField(chatID, "PhoneNumber", PhoneNumber)
		if err != nil {
			m.app.Log.Errorf(
				"handleCallbackQueryData: ошибка сохранения поля PhoneNumber: chatID=%d, ошибка=%v",
				chatID,
				err,
			)
			return
		}

		answerCallback("")

		m.sendMessage(chatID, instructions)

	// ---------------------------------------------------------
	// ВЫБОР ДАТЫ ДОСТАВКИ
	// ---------------------------------------------------------

	case "deliverydate":
		if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
			err := fmt.Errorf(
				"неверный формат callback даты доставки: chatID=%d, callback=%q",
				chatID,
				callbackData,
			)
			m.app.Log.Error("handleCallbackQueryData", err.Error())
			m.SendPopupMessage(update, "Некорректная дата")
			return
		}


		answerCallback("")

		field, err := bychat.GetFieldByChatID(m.app, chatID, "DeliveryDate")
		if err != nil {
			m.app.Log.Errorf(
				"handleCallbackQueryData: ошибка получения DeliveryDate: chatID=%d, ошибка=%v",
				chatID,
				err,
			)
			return
		}

		if field != "Дату доставки:" {
			m.app.Log.Warnf(
				"handleCallbackQueryData: неожиданный этап выбора даты: chatID=%d, значение=%q",
				chatID,
				field,
			)
			return
		}

		formatted, dateError := bychat.FormatDateString(m.app, parts[1])

		if dateError != "" {
			msg := tgbotapi.NewMessage(
				chatID,
				dateError+"\nВыберите дату доставки:",
			)

			msg.ReplyMarkup = m.createCalendarKeyboard(time.Now())

			sent, err := m.bot.Send(msg)
			if err != nil {
				m.app.Log.Errorf(
					"handleCallbackQueryData: ошибка отправки календаря: chatID=%d, ошибка=%v",
					chatID,
					err,
				)
			} else {
				m.trackMessage(chatID, sent.MessageID)
			}

			return
		}

		err = m.addOrUpdateOrderField(chatID, "DeliveryDate", formatted)
		if err != nil {
			m.app.Log.Errorf(
				"handleCallbackQueryData: ошибка сохранения даты доставки: chatID=%d, дата=%q, ошибка=%v",
				chatID,
				formatted,
				err,
			)
			return
		}

		orders := m.repo.GetByUserID(chatID)

		if orders.PhoneNumber == "" ||
			orders.FullName == "" ||
			orders.DeliveryAddress == "" {

			instructions := "Пожалуйста, введите " + PhoneNumber + "\n" +
				"Пример:\n" +
				"+79345678901\n"

			err = m.addOrUpdateOrderField(chatID, "PhoneNumber", PhoneNumber)
			if err != nil {
				m.app.Log.Errorf(
					"handleCallbackQueryData: ошибка сохранения PhoneNumber: chatID=%d, ошибка=%v",
					chatID,
					err,
				)
				return
			}

			m.sendMessage(chatID, instructions)
			return
		}

		msgText := "Использовать следующие данные для заказа\n"
		msgText += fmt.Sprintf("Номер телефона: %v\n", orders.PhoneNumber)
		msgText += fmt.Sprintf("ФИО: %v\n", orders.FullName)
		msgText += fmt.Sprintf("%s %v\n", DeliveryAddress, orders.DeliveryAddress)

		msg := tgbotapi.NewMessage(chatID, msgText)

		btnSend := tgbotapi.NewInlineKeyboardButtonData("Да", "sends_msgText")
		btnDelete := tgbotapi.NewInlineKeyboardButtonData("Нет", "phonenumber_PhoneNumber")

		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(btnSend, btnDelete),
		)

		sent, err := m.bot.Send(msg)
		if err != nil {
			m.app.Log.Errorf(
				"handleCallbackQueryData: ошибка отправки данных заказа: chatID=%d, ошибка=%v",
				chatID,
				err,
			)
		} else {
			m.trackMessage(chatID, sent.MessageID)
		}

	// ---------------------------------------------------------
	// ПЕРЕКЛЮЧЕНИЕ МЕСЯЦА КАЛЕНДАРЯ
	// ---------------------------------------------------------

	case "fav":
		if len(parts) != 3 || parts[1] != "toggle" {
			err := fmt.Errorf("неверный callback избранного: chatID=%d, callback=%q", chatID, callbackData)
			m.app.Log.Error("handleCallbackQueryData", err.Error())
			answerCallback("Некорректная команда")
			return
		}
		productID, err := strconv.Atoi(parts[2])
		if err != nil || productID <= 0 {
			m.app.Log.Errorf("handleCallbackQueryData: неверный ID товара в избранном: chatID=%d, callback=%q", chatID, callbackData)
			answerCallback("Некорректный товар")
			return
		}
		added, err := m.repo.ToggleFavorite(chatID, productID)
		if err != nil {
			m.app.Log.Errorf("handleCallbackQueryData: ошибка изменения избранного: chatID=%d, productID=%d, ошибка=%v", chatID, productID, err)
			answerCallback("Не удалось изменить избранное")
			return
		}
		if added {
			answerCallback("Добавлено в избранное ⭐")
		} else {
			answerCallback("Удалено из избранного")
		}
		if callbackQuery.Message != nil && strings.HasPrefix(callbackQuery.Message.Text, "⭐ Избранные товары") {
			m.MainFavoritesByUserID(chatID)
		}

	case "repeat":
		if len(parts) != 2 || parts[1] != "last" {
			m.app.Log.Errorf("handleCallbackQueryData: неверный callback повтора заказа: chatID=%d, callback=%q", chatID, callbackData)
			answerCallback("Некорректная команда")
			return
		}
		basketMu.Lock()
		basketNotEmpty := len(structs.Baskets[chatID]) > 0
		basketMu.Unlock()
		if basketNotEmpty {
			answerCallback("Сначала очистите текущую корзину")
			return
		}
		purchases, err := m.repo.GetLastOrderPurchases(chatID)
		if err != nil {
			m.app.Log.Errorf("handleCallbackQueryData: ошибка получения последнего заказа: chatID=%d, ошибка=%v", chatID, err)
			answerCallback("Не удалось повторить заказ")
			return
		}
		if len(purchases) == 0 {
			answerCallback("Предыдущий заказ не найден")
			return
		}
		for i := range purchases {
			if purchases[i].Quantity <= 0 || purchases[i].Cost <= 0 || strings.TrimSpace(purchases[i].Item) == "" {
				m.app.Log.Errorf("handleCallbackQueryData: в предыдущем заказе некорректная позиция: chatID=%d, товар=%q", chatID, purchases[i].Item)
				answerCallback("В предыдущем заказе есть некорректный товар")
				return
			}
			purchases[i].ID = generateBasketItemID()
		}
		basketMu.Lock()
		structs.Baskets[chatID] = purchases
		basketMu.Unlock()
		answerCallback("Последний заказ добавлен в корзину")
		m.deleteMessages(chatID)
		m.ShowBasket(chatID)

	case "nextmonth":
		if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
			m.app.Log.Errorf(
				"handleCallbackQueryData: недостаточно параметров nextmonth: chatID=%d",
				chatID,
			)
			m.SendPopupMessage(update, "Некорректный месяц")
			return
		}

		month, err := time.ParseInLocation("01-2006", parts[1], time.Local)
		if err != nil {
			m.app.Log.Errorf(
				"handleCallbackQueryData: ошибка разбора месяца: chatID=%d, значение=%q, ошибка=%v",
				chatID,
				parts[1],
				err,
			)
			m.SendPopupMessage(update, "Некорректный месяц")
			return
		}

		edit := tgbotapi.NewEditMessageReplyMarkup(
			chatID,
			callbackQuery.Message.MessageID,
			m.createCalendarKeyboard(month),
		)

		if _, err := m.bot.Send(edit); err != nil {
			m.app.Log.Errorf(
				"handleCallbackQueryData: ошибка обновления календаря: chatID=%d, месяц=%s, ошибка=%v",
				chatID,
				month.Format("01-2006"),
				err,
			)
			return
		}

		answerCallback("")

	// ---------------------------------------------------------
	// ОЧИСТКА КОРЗИНЫ
	// ---------------------------------------------------------

	case "delete":
		answerCallback("")
		m.DeleteBasketContents(chatID)

	// ---------------------------------------------------------
	// НЕИЗВЕСТНОЕ ДЕЙСТВИЕ
	// ---------------------------------------------------------

	default:
		m.app.Log.Errorf(
			"handleCallbackQueryData: неизвестное действие: chatID=%d, callback=%q",
			chatID,
			callbackData,
		)

		answerCallback("Неизвестная команда")
	}

	return
}

func (m *MessageService) handleFieldByChatID(update tgbotapi.Update, pureText string) string {
	if update.Message == nil || update.Message.Chat == nil {
		m.app.Log.Error(
			"handleFieldByChatID: отсутствует Message или Chat",
		)
		return "ok"
	}

	chatID := update.Message.Chat.ID

	if chatID == 0 {
		m.app.Log.Error("handleFieldByChatID: chatID равен нулю")
		return "ok"
	}

	// Получаем и выводим поле DeliveryAddress по chatID Номер телефона
	if field, err := bychat.GetFieldByChatID(m.app, chatID, "PhoneNumber"); err == nil {

		if field == PhoneNumber {
			formatted, str := bychat.FormatPhoneNumber(m.app, pureText)

			if str != "" {
				// Формирование текста с просьбой ввести номер телефона
				instructions := "\nПожалуйста, введите " + PhoneNumber + "\n" +
					"Пример:\n" +
					"+79345678901\n"
				str += instructions

				//отправляем сообщение
				m.sendMessage(chatID, str)
				return "ok"
			}

			err = m.addOrUpdateOrderField(chatID, "PhoneNumber", formatted)
			if err != nil {
				m.app.Log.Errorf("Ошибка при добавлении или обновлении поля PhoneNumber %v", err)
				return "ok"
			}

			instructions := "Пожалуйста, введите " + DeliveryAddress + "\n" +
				"Пример:\n" +
				"г. Будущего, ул. Пушкина, д. 12 кв 5\n"

			err = m.addOrUpdateOrderField(chatID, "DeliveryAddress", DeliveryAddress)
			if err != nil {
				m.app.Log.Errorf("Ошибка при добавлении или обновлении поля DeliveryAddress: %v", err)
				return "ok"
			}

			//отправляем сообщение
			m.sendMessage(chatID, instructions)
			pureText = "ok"
			return "ok"
		}
	}

	// Получаем и выводим поле DeliveryAddress по chatID Адрес доставки
	if field, err := bychat.GetFieldByChatID(m.app, chatID, "DeliveryAddress"); err == nil {

		if field == DeliveryAddress {
			if len(pureText) < 10 {
				str := "Адрес доставки не может быть пустым\n"
				instructions := "Пожалуйста, введите " + DeliveryAddress + "\nПример:\nг. Будущего, ул. Пушкина, д. 12 кв 5\n"
				str += instructions
				//отправляем сообщение
				m.sendMessage(chatID, str)
				return "ok"
			}

			err = m.addOrUpdateOrderField(chatID, "DeliveryAddress", pureText)
			if err != nil {
				m.app.Log.Errorf("Ошибка при добавлении или обновлении поля DeliveryAddress: %v", err)
				return "ok"
			}

			instructions := "Пожалуйста, введите " + FullName + "\nПример:\nИванов Иван Иванович\n"

			err = m.addOrUpdateOrderField(chatID, "FullName", FullName)
			if err != nil {
				m.app.Log.Errorf("Ошибка при добавлении или обновлении поля FullName: %v", err)
				return "ok"
			}

			//отправляем сообщение.
			m.sendMessage(chatID, instructions)

			pureText = "ok"
			return "ok"
		}
	}

	// Получаем и выводим поле DeliveryDate по chatID ФИО
	if field, err := bychat.GetFieldByChatID(m.app, chatID, "FullName"); err == nil {

		if field == FullName {
			if pureText == "" || len(pureText) < 4 {

				str := "ФИО: не может быть пустым и имя не может быть короче 3 символов\n"
				instructions := "Пожалуйста, введите " + FullName + "\nПример:\nИванов Иван Иванович\n"
				str += instructions

				//отправляем сообщение.
				m.sendMessage(chatID, str)
				return "ok"
			}

			err = m.addOrUpdateOrderField(chatID, "FullName", pureText)
			if err != nil {
				m.app.Log.Errorf("ошибка при добавлении или обновлении поля FullName: %v", err)
				return "ok"
			}

			pureText = PhoneNumber
			deliveryDateMu.Lock()
			DeliveryDateChatID[chatID] = 0
			deliveryDateMu.Unlock()

			return pureText
		}
	}

	return pureText
}

// handleDeliveryDetailsMessage анализируем и извлекаем данные из сообщения.
// Объединяем данные заказа с содержимым корзины.
func (m *MessageService) handleDeliveryDetailsMessage(chatID int64) {
	if chatID == 0 {
		m.app.Log.Errorf("нет данных для обработки chatID = %v", chatID)
		return
	}

	var MsgText string

	//"Номер телефона: [ваш номер]\n" +
	if field, err := bychat.GetFieldByChatID(m.app, chatID, "PhoneNumber"); err == nil {
		str, _ := field.(string)
		MsgText += "Номер телефона: " + str + "\n"
	}

	//"Адрес доставки: [ваш адрес]\n" +
	if field, err := bychat.GetFieldByChatID(m.app, chatID, "DeliveryAddress"); err == nil {
		str, _ := field.(string)
		MsgText += DeliveryAddress + " " + str + "\n"
	}

	//"Дата доставки: [желаемая дата]\n" +
	if field, err := bychat.GetFieldByChatID(m.app, chatID, "DeliveryDate"); err == nil {
		str, _ := field.(string)
		MsgText += "Дата доставки: " + str + "\n"
	}

	//"ФИО: [ваше имя и фамилия]\n\n" +
	if field, err := bychat.GetFieldByChatID(m.app, chatID, "FullName"); err == nil {
		str, _ := field.(string)
		MsgText += "ФИО: " + str + "\n"
	}

	if comment, err := bychat.GetFieldByChatID(m.app, chatID, "Comment"); err == nil {
		if commentText, ok := comment.(string); ok && strings.TrimSpace(commentText) != "" {
			MsgText += "Комментарий: " + strings.TrimSpace(commentText) + "\n"
		}
	}

	// Получаем согласованный снимок корзины.
	basketMu.Lock()
	purchases := append([]structs.Purchase(nil), structs.Baskets[chatID]...)
	basketMu.Unlock()

	orderText := "Детали заказа:\n" + MsgText + BuildPurchaseReport(purchases)
	orderDetailsMu.Lock()
	orderDetails[chatID] = orderText
	orderDetailsMu.Unlock()

	// Отправляем пользователю подтверждение с деталями заказа и кнопкой "Заказать"
	btnOrder := tgbotapi.NewInlineKeyboardButtonData("Заказать", "confirm_order")
	inlineKeyboard := tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(btnOrder))

	confirmMsg := tgbotapi.NewMessage(chatID, orderText)
	confirmMsg.ReplyMarkup = inlineKeyboard

	if sentMsg, err := m.bot.Send(confirmMsg); err != nil {
		m.app.Log.Errorf("%v", err)
		return
	} else {
		// Сохраняем идентификатор отправленного сообщения для последующего удаления
		basketMu.Lock()
		BasketMessages[chatID] = sentMsg.MessageID
		messagesToDelete[chatID] = append(messagesToDelete[chatID], sentMsg.MessageID)
		basketMu.Unlock()
	}
}
