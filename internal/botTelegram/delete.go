package botTelegram

import (
	"ProjectTelBotMark/internal/bychat"
	"ProjectTelBotMark/internal/structs"
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
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
func (m *MessageService) photoMessagesDelete(chatID int64) {
	basketMu.Lock()
	msgID := photoMessagesToDelete[chatID]
	basketMu.Unlock()
	if msgID == 0 {
		return
	}
	if _, err := m.bot.Request(tgbotapi.NewDeleteMessage(chatID, msgID)); err != nil {
		m.app.Log.Errorf("photoMessagesDelete: ошибка удаления фото: chatID=%d, messageID=%d, ошибка=%v", chatID, msgID, err)
		return
	}
	basketMu.Lock()
	if photoMessagesToDelete[chatID] == msgID {
		delete(photoMessagesToDelete, chatID)
	}
	basketMu.Unlock()
}

// keyboardMessagesDelete удаляем предыдущее сообщение с клавиатурой, если оно есть.
func (m *MessageService) keyboardMessagesDelete(chatID int64) {
	basketMu.Lock()
	msgID := keyboardMessagesToDelete[chatID]
	basketMu.Unlock()
	if msgID == 0 {
		return
	}
	if _, err := m.bot.Request(tgbotapi.NewDeleteMessage(chatID, msgID)); err != nil {
		m.app.Log.Errorf("keyboardMessagesDelete: ошибка удаления клавиатуры: chatID=%d, messageID=%d, ошибка=%v", chatID, msgID, err)
		return
	}
	basketMu.Lock()
	if keyboardMessagesToDelete[chatID] == msgID {
		delete(keyboardMessagesToDelete, chatID)
	}
	basketMu.Unlock()
}

// clearOrder Функция для очистки данных заказа по chatID.
func (m *MessageService) clearOrder(chatID int64) {
	if chatID == 0 {
		m.app.Log.Errorf("нет данных для обработки chatID = %v", chatID)
		return
	}
	bychat.OrdersMu.Lock()
	delete(bychat.Orders, chatID)
	bychat.OrdersMu.Unlock()
	orderDetailsMu.Lock()
	delete(orderDetails, chatID)
	orderDetailsMu.Unlock()
	commentPendingMu.Lock()
	delete(commentPending, chatID)
	commentPendingMu.Unlock()
}

// deletePreviousMessages Удаляем сообщения.
func (m *MessageService) deletePreviousMessages(chatID int64, messageID int) {
	if chatID == 0 || messageID == 0 {
		m.app.Log.Errorf("нет данных для обработки chatID = %v messageID = %v", chatID, messageID)
		return
	}

	// Проверяем, есть ли сообщения для удаления в этом чате
	basketMu.Lock()
	_, found := messagesToDelete[chatID]
	basketMu.Unlock()
	if found {
		// Проходимся по каждому идентификатору сообщения и удаляем его
		deleteMsg := tgbotapi.NewDeleteMessage(chatID, messageID)
		if _, err := m.bot.Request(deleteMsg); err != nil {
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
func (m *MessageService) deleteMessages(chatID int64) {
	if chatID == 0 {
		m.app.Log.Errorf("нет данных для обработки chatID = %v", chatID)
		return
	}
	// Получаем ID сообщения под общей блокировкой.
	basketMu.Lock()
	messageID := BasketMessages[chatID]
	basketMu.Unlock()
	if messageID == 0 {
		m.app.Log.Debugf("deleteMessages: нет сохранённого ID сообщения: chatID=%d", chatID)
		return
	}
	if _, err := m.bot.Request(tgbotapi.NewDeleteMessage(chatID, messageID)); err != nil {
		m.app.Log.Errorf("deleteMessages: ошибка удаления сообщения: chatID=%d, messageID=%d, ошибка=%v", chatID, messageID, err)
		return
	}
	basketMu.Lock()
	if BasketMessages[chatID] == messageID {
		BasketMessages[chatID] = 0
	}
	basketMu.Unlock()
}

// DeleteBasketContents очищает корзину пользователя.
func (m *MessageService) DeleteBasketContents(chatID int64) {
	if chatID == 0 {
		err := fmt.Errorf("некорректный chatID: %d", chatID)
		m.app.Log.Error("DeleteBasketContents", err.Error())
		return
	}

	basketMu.Lock()
	_, basketExists := structs.Baskets[chatID]
	delete(structs.Baskets, chatID)
	messageID := BasketMessages[chatID]
	BasketMessages[chatID] = 0
	basketMu.Unlock()

	if !basketExists {
		m.app.Log.Warnf("DeleteBasketContents: корзина уже была пустой: chatID=%d", chatID)
	} else {
		m.app.Log.Infof("DeleteBasketContents: корзина очищена: chatID=%d", chatID)
	}

	if messageID != 0 {
		if _, err := m.bot.Request(tgbotapi.NewDeleteMessage(chatID, messageID)); err != nil {
			m.app.Log.Errorf("DeleteBasketContents: ошибка удаления сообщения корзины: chatID=%d, messageID=%d, ошибка=%v", chatID, messageID, err)
		}
	}

	m.sendMessage(chatID, "Корзина успешно очищена.")
}
