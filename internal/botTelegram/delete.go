package botTelegram

import (
	"ProjectTelBotMark/internal/bychat"
	"ProjectTelBotMark/internal/structs"
	"fmt"
	"strings"

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

// greetingMessagesToKeep хранит ID приветственного сообщения, которое остаётся в чате.
var greetingMessagesToKeep = make(map[int64]int)

 // trackMessage сохраняет ID нового сообщения бота для последующей общей очистки.
func (m *MessageService) trackMessage(chatID int64, messageID int) {
	if chatID == 0 || messageID == 0 {
		m.app.Log.Errorf("trackMessage: некорректные идентификаторы: chatID=%d, messageID=%d", chatID, messageID)
		return
	}
	basketMu.Lock()
	for _, trackedID := range messagesToDelete[chatID] {
		if trackedID == messageID {
			basketMu.Unlock()
			return
		}
	}
	messagesToDelete[chatID] = append(messagesToDelete[chatID], messageID)
	basketMu.Unlock()
}

 // forgetTrackedMessage убирает уже удалённое сообщение из всех списков учёта.
func forgetTrackedMessage(chatID int64, messageID int) {
	if chatID == 0 || messageID == 0 {
		return
	}
	basketMu.Lock()
	ids := messagesToDelete[chatID]
	filtered := ids[:0]
	for _, id := range ids {
		if id != messageID {
			filtered = append(filtered, id)
		}
	}
	if len(filtered) == 0 {
		delete(messagesToDelete, chatID)
	} else {
		messagesToDelete[chatID] = filtered
	}
	if photoMessagesToDelete[chatID] == messageID {
		delete(photoMessagesToDelete, chatID)
	}
	if keyboardMessagesToDelete[chatID] == messageID {
		delete(keyboardMessagesToDelete, chatID)
	}
	if BasketMessages[chatID] == messageID {
		delete(BasketMessages, chatID)
	}
	basketMu.Unlock()
}

 // callbackOpensNewScreen определяет, должно ли нажатие убрать текущий экран.
func (m *MessageService) callbackOpensNewScreen(callback *tgbotapi.CallbackQuery) bool {
	if callback == nil || callback.Message == nil || callback.Message.Chat == nil {
		return false
	}

	data := callback.Data

	// Эти действия открывают другой экран или полностью заменяют текущий.
	switch data {
	case "send_basket_1_1", "confirm_order", "sends_msgText",
		"phonenumber_PhoneNumber", "repeat_last", "delete_basket_1_1",
		"deliveryconfirm_yes", "deliveryconfirm_edit":
		return true
	}

	switch {
	case strings.HasPrefix(data, "cat_"):
		category := strings.TrimPrefix(data, "cat_")
		if category == "Корзина" {
			basketMu.Lock()
			empty := len(structs.Baskets[callback.Message.Chat.ID]) == 0
			basketMu.Unlock()
			return !empty
		}
		// Категория, избранное, заказы, старт и навигация — новые экраны.
		return true

	case strings.HasPrefix(data, "fav_toggle_"):
		// В избранном после переключения кнопки список перерисовывается.
		return strings.HasPrefix(callback.Message.Text, "⭐ Избранные товары")

	case strings.HasPrefix(data, "deliverydate_"):
		// Выбор даты завершает экран календаря.
		return true

	default:
		// add_*, basket_*, nextmonth_* и ignore не удаляют текущий экран.
		return false
	}
}
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
	forgetTrackedMessage(chatID, msgID)
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
	forgetTrackedMessage(chatID, msgID)
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

// deletePreviousMessages удаляет все предыдущие сообщения бота и указанное сообщение пользователя.
 // Приветственное сообщение сохраняется; при нажатии его кнопки удаляется только клавиатура.
func (m *MessageService) deletePreviousMessages(chatID int64, messageID int) {
	if chatID == 0 {
		m.app.Log.Errorf("deletePreviousMessages: некорректный chatID=%d", chatID)
		return
	}

	basketMu.Lock()
	messageIDs := make(map[int]struct{})
	for _, id := range messagesToDelete[chatID] {
		if id != 0 {
			messageIDs[id] = struct{}{}
		}
	}
	if id := photoMessagesToDelete[chatID]; id != 0 {
		messageIDs[id] = struct{}{}
	}
	if id := keyboardMessagesToDelete[chatID]; id != 0 {
		messageIDs[id] = struct{}{}
	}
	if id := BasketMessages[chatID]; id != 0 {
		messageIDs[id] = struct{}{}
	}

	greetingID := greetingMessagesToKeep[chatID]
	keepGreeting := greetingID != 0 && greetingID == messageID
	if messageID != 0 && messageID != greetingID {
		messageIDs[messageID] = struct{}{}
	}

	delete(messagesToDelete, chatID)
	delete(photoMessagesToDelete, chatID)
	delete(keyboardMessagesToDelete, chatID)
	delete(BasketMessages, chatID)
	basketMu.Unlock()

	if keepGreeting {
		edit := tgbotapi.NewEditMessageReplyMarkup(chatID, greetingID, tgbotapi.InlineKeyboardMarkup{InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{}})
		if _, err := m.bot.Request(edit); err != nil {
			m.app.Log.Errorf(
				"deletePreviousMessages: не удалось убрать клавиатуру приветствия: chatID=%d, messageID=%d, ошибка=%v",
				chatID, greetingID, err,
			)
		} else {
			m.app.Log.Debugf("deletePreviousMessages: клавиатура приветствия убрана: chatID=%d, messageID=%d", chatID, greetingID)
		}
	}

	delete(messageIDs, greetingID)
	for id := range messageIDs {
		if _, err := m.bot.Request(tgbotapi.NewDeleteMessage(chatID, id)); err != nil {
			m.app.Log.Errorf(
				"deletePreviousMessages: ошибка удаления сообщения: chatID=%d, messageID=%d, ошибка=%v",
				chatID, id, err,
			)
			continue
		}
		m.app.Log.Debugf("deletePreviousMessages: сообщение удалено: chatID=%d, messageID=%d", chatID, id)
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
	forgetTrackedMessage(chatID, messageID)
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
		} else {
			forgetTrackedMessage(chatID, messageID)
		}
	}

	m.sendMessage(chatID, "Корзина успешно очищена.")
}
