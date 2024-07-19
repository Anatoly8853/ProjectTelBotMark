package main

import (
	`ProjectTelBotMark/bychat`
	`ProjectTelBotMark/structs`

	tgbotapi `github.com/go-telegram-bot-api/telegram-bot-api/v5`
)

// photoMessagesToDelete структура для хранения идентификаторов сообщений с фото.
var photoMessagesToDelete = make(map[int64]int)

// keyboardMessagesToDelete структура для хранения идентификаторов сообщений с клавиатурой.
var keyboardMessagesToDelete = make(map[int64]int)

// messagesToDelete удаление сообщений.
var messagesToDelete = make(map[int64][]int)

// BasketMessages Карта для хранения данных о покупках для каждого пользователя.
var BasketMessages = make(map[int64]int)

// photoMessagesDelete удаляем предыдущее сообщение с фото, если оно есть.
func photoMessagesDelete(chatID int64) {

	// Удаляем предыдущее сообщение с фото, если оно есть
	if msgID, ok := photoMessagesToDelete[chatID]; ok {
		deleteMessage := tgbotapi.NewDeleteMessage(chatID, msgID)
		if _, err := bot.Send(deleteMessage); err != nil {
			//logger.Errorf("Ошибка при удалении предыдущего сообщения с фото: %v", err)
		} else {
			delete(photoMessagesToDelete, chatID) // Удаляем запись из карты после успешного удаления
		}
	}

}

// keyboardMessagesDelete удаляем предыдущее сообщение с клавиатурой, если оно есть.
func keyboardMessagesDelete(chatID int64) {

	// Удаляем предыдущее сообщение с клавиатурой, если оно есть
	if msgID, ok := keyboardMessagesToDelete[chatID]; ok {
		deleteMessage := tgbotapi.NewDeleteMessage(chatID, msgID)
		if _, err := bot.Send(deleteMessage); err != nil {
			//logger.Errorf("Ошибка при удалении предыдущего сообщения с клавиатурой: %v", err)
		} else {
			delete(keyboardMessagesToDelete, chatID) // Удаляем запись из карты после успешного удаления
		}
	}

}

// clearOrder Функция для очистки данных заказа по chatID.
func clearOrder(chatID int64) {
	if chatID == 0 {
		app.Log.Errorf("нет данных для обработки chatID = %v", chatID)
		return
	}
	delete(bychat.Orders, chatID)
}

// deletePreviousMessages Удаляем сообщения.
func deletePreviousMessages(chatID int64, messageID int) {
	if chatID == 0 || messageID == 0 {
		app.Log.Errorf("нет данных для обработки chatID = %v messageID = %v", chatID, messageID)
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
					deleteMsg = tgbotapi.NewDeleteMessage(chatIDs, messageIDa)
					if _, err := bot.Request(deleteMsg); err != nil {
						//log.Printf("Ошибка при удалении сообщения: %v", err)
						// Не используйте log.Panic, чтобы избежать прерывания работы бота из-за одной ошибки
					}

				}
			}

		*/

	}
}

// deleteMessages Удаляем сообщения.
func deleteMessages(chatID int64) {
	if chatID == 0 {
		app.Log.Errorf("нет данных для обработки chatID = %v", chatID)
		return
	}
	// Удаление сообщения
	messageIDs := BasketMessages[chatID]
	deleteMsg := tgbotapi.NewDeleteMessage(chatID, messageIDs)
	if _, err := bot.Request(deleteMsg); err != nil {
		//log.Panic(err) // Обработка ошибки по вашему усмотрению
	}
}

// DeleteBasketContents Очищаем корзину.
func DeleteBasketContents(chatID int64) {
	if chatID == 0 {
		app.Log.Errorf("нет данных для обработки chatID = %v", chatID)
		return
	}

	// Очистка содержимого корзины пользователя
	structs.Baskets[chatID] = nil // Или 	baskets[chatID] = []Purchase{} //для обнуления без удаления ключа

	// Удаление сообщения с содержимым корзины
	if BasketMessages[chatID] == 0 {
		app.Log.Errorf("нет данных для обработки BasketMessages[chatID] = %v", BasketMessages[chatID])
		return
	}

	messageID := BasketMessages[chatID]

	if messageID != 0 {
		deleteMsg := tgbotapi.NewDeleteMessage(chatID, messageID)
		if _, err := bot.Request(deleteMsg); err != nil {
			app.Log.Errorf("Удаление сообщения с содержимым корзины %v", err)
		}
		// После успешного удаления сообщения можно обнулить идентификатор сообщения в basketMessages
		BasketMessages[chatID] = 0
	}

	// Отправка подтверждающего сообщения пользователю, что корзина была очищена
	sendMessage(chatID, "Корзина успешно очищена.")

}
