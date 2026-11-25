package botTelegram

import (
	"ProjectTelBotMark/config"
	"ProjectTelBotMark/internal/bychat"
	"ProjectTelBotMark/internal/dataSources"
	"ProjectTelBotMark/internal/dateutil"
	"ProjectTelBotMark/internal/structs"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	slogavp "github.com/Anatoly8853/slog-avp/v2"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const UploadsDir = `./internal/routers/html`

type MessageService struct {
	repo *dataSources.Repository
	app  *slogavp.Application
	bot  *tgbotapi.BotAPI

	basketMessages map[int64]int
	mu             sync.RWMutex
}

var (
	DeliveryDateChatID = make(map[int64]int)
	deliveryDateMu     sync.RWMutex
	orderDetails       = make(map[int64]string)
	orderDetailsMu     sync.RWMutex
	commentPending     = make(map[int64]bool)
	commentPendingMu   sync.Mutex
	orderProcessing    = make(map[int64]bool)
	orderProcessingMu  sync.Mutex
	basketMu           sync.Mutex
	basketItemID       atomic.Int64
)

// requestOrderComment запрашивает у пользователя комментарий к заказу.
func (m *MessageService) requestOrderComment(chatID int64) {
	if chatID == 0 {
		m.app.Log.Error("requestOrderComment: chatID равен нулю")
		return
	}

	if err := m.addOrUpdateOrderField(chatID, "Comment", ""); err != nil {
		m.app.Log.Errorf("requestOrderComment: не удалось очистить предыдущий комментарий: chatID=%d, ошибка=%v", chatID, err)
		return
	}

	commentPendingMu.Lock()
	commentPending[chatID] = true
	commentPendingMu.Unlock()

	m.sendMessage(chatID, "Напишите комментарий к заказу. Если комментариев нет, напишите «Нет».")
}

// reserveOrderProcessing не позволяет одновременно сохранить один заказ дважды.
func reserveOrderProcessing(chatID int64) bool {
	orderProcessingMu.Lock()
	defer orderProcessingMu.Unlock()
	if orderProcessing[chatID] {
		return false
	}
	orderProcessing[chatID] = true
	return true
}

func releaseOrderProcessing(chatID int64) {
	orderProcessingMu.Lock()
	delete(orderProcessing, chatID)
	orderProcessingMu.Unlock()
}

func generateBasketItemID() int64 {
	return basketItemID.Add(1)
}

// BuildPurchaseReport формирует подробный отчёт по позициям корзины.
// Cost — цена одной единицы товара.
func BuildPurchaseReport(purchases []structs.Purchase) string {
	if len(purchases) == 0 {
		return "🛒 Корзина пуста."
	}
	totalCost, err := structs.CalculatePurchasesTotal(purchases)
	if err != nil {
		return "Не удалось рассчитать стоимость корзины: " + err.Error()
	}
	var builder strings.Builder
	for _, purchase := range purchases {
		lineTotal := purchase.Cost * purchase.Quantity
		fmt.Fprintf(&builder,
			"📦 %s\nКоличество: %d\nЦена за единицу: %d руб.\nСумма: %d руб.\n\n",
			purchase.Item, purchase.Quantity, purchase.Cost, lineTotal,
		)
	}
	fmt.Fprintf(&builder, "💰 Итого: %d руб.", totalCost)
	return builder.String()
}

// formatProductTitle формирует единое отображаемое название товара.
func formatProductTitle(category, subcategory, item string) string {
	category = strings.TrimSpace(category)
	subcategory = strings.TrimSpace(subcategory)
	item = strings.TrimSpace(item)

	if category == "" {
		return item
	}
	if subcategory == "" {
		return strings.TrimSpace(category + " " + item)
	}

	suffix := strings.TrimSpace(strings.TrimPrefix(item, subcategory))
	return strings.TrimSpace(category + " " + strings.ToLower(subcategory) + " " + suffix)
}

// addPurchaseToBasket добавляет позицию к снимку корзины и проверяет общую сумму.
func addPurchaseToBasket(current []structs.Purchase, item string, quantity, cost int) ([]structs.Purchase, int, error) {
	if strings.TrimSpace(item) == "" || quantity <= 0 || cost <= 0 {
		return nil, 0, fmt.Errorf("некорректные данные товара: товар=%q, количество=%d, цена=%d", item, quantity, cost)
	}
	purchases := append([]structs.Purchase(nil), current...)
	found := false
	maxInt := int(^uint(0) >> 1)
	for i := range purchases {
		if purchases[i].Item != item || purchases[i].Cost != cost {
			continue
		}
		if purchases[i].Quantity > maxInt-quantity {
			return nil, 0, fmt.Errorf("переполнение количества товара %q", item)
		}
		purchases[i].Quantity += quantity
		found = true
		break
	}
	if !found {
		purchases = append(purchases, structs.Purchase{ID: generateBasketItemID(), Item: item, Quantity: quantity, Cost: cost})
	}
	total, err := structs.CalculatePurchasesTotal(purchases)
	if err != nil {
		return nil, 0, err
	}
	return purchases, total, nil
}

// changePurchaseQuantity меняет количество позиции в копии корзины; ноль удаляет позицию.
func changePurchaseQuantity(current []structs.Purchase, itemID int64, delta int) ([]structs.Purchase, int, error) {
	if itemID <= 0 || (delta != 1 && delta != -1) {
		return nil, 0, fmt.Errorf("некорректные параметры изменения корзины: itemID=%d, delta=%d", itemID, delta)
	}
	purchases := append([]structs.Purchase(nil), current...)
	for i := range purchases {
		if purchases[i].ID != itemID {
			continue
		}
		if delta > 0 && purchases[i].Quantity == int(^uint(0)>>1) {
			return nil, 0, fmt.Errorf("переполнение количества позиции itemID=%d", itemID)
		}
		purchases[i].Quantity += delta
		if purchases[i].Quantity <= 0 {
			purchases = append(purchases[:i], purchases[i+1:]...)
		}
		if len(purchases) == 0 {
			return purchases, 0, nil
		}
		total, err := structs.CalculatePurchasesTotal(purchases)
		if err != nil {
			return nil, 0, err
		}
		return purchases, total, nil
	}
	return nil, 0, fmt.Errorf("позиция корзины не найдена: itemID=%d", itemID)
}

func NewUpdateMessage(repo *dataSources.Repository, app *slogavp.Application, bot *tgbotapi.BotAPI) *MessageService {
	return &MessageService{
		repo:           repo,
		app:            app,
		bot:            bot,
		basketMessages: make(map[int64]int),
	}
}

// AddToBasket добавляет товар в корзину пользователя.
func (m *MessageService) AddToBasket(update tgbotapi.Update, item string, quantity int, cost int) {
	if update.CallbackQuery == nil || update.CallbackQuery.Message == nil || update.CallbackQuery.Message.Chat == nil {
		m.app.Log.Error("AddToBasket: отсутствуют данные CallbackQuery, Message или Chat")
		return
	}

	chatID := update.CallbackQuery.Message.Chat.ID
	if chatID == 0 || strings.TrimSpace(item) == "" || quantity <= 0 || cost <= 0 {
		m.app.Log.Errorf(
			"AddToBasket: некорректные данные: chatID=%d, товар=%q, количество=%d, цена=%d",
			chatID, item, quantity, cost,
		)
		m.SendPopupMessage(update, "Некорректные данные товара")
		return
	}

	basketMu.Lock()
	purchases, totalCost, err := addPurchaseToBasket(structs.Baskets[chatID], item, quantity, cost)
	if err != nil {
		basketMu.Unlock()
		m.app.Log.Errorf("AddToBasket: не удалось добавить товар: chatID=%d, товар=%q, ошибка=%v", chatID, item, err)
		m.SendPopupMessage(update, "Не удалось добавить товар: "+err.Error())
		return
	}
	structs.Baskets[chatID] = purchases
	basketMu.Unlock()

	popupText := fmt.Sprintf(
		"Добавлено в корзину:\n%s, количество: %d, цена за единицу: %d руб.\nОбщая стоимость: %d руб.",
		item, quantity, cost, totalCost,
	)
	m.SendPopupMessage(update, popupText)
}

// SendPopupMessage Отправка всплывающего сообщения.
func (m *MessageService) SendPopupMessage(update tgbotapi.Update, popupText string) {
	if update.CallbackQuery == nil || update.CallbackQuery.ID == `` || update.CallbackQuery.Message == nil || popupText == `` {
		m.app.Log.Errorf(`SendPopupMessage: отсутствуют данные callback или текст сообщения`)
		return
	}
	callbackConfig := tgbotapi.NewCallback(update.CallbackQuery.ID, popupText)
	callbackConfig.ShowAlert = true // Для отображения в виде всплывающего уведомления

	if _, err := m.bot.Request(callbackConfig); err != nil {
		m.app.Log.Errorf(`не удается создать всплывающее сообщение %v`, err)
		return
	}
}

// ShowBasket показывает содержимое корзины пользователя.
func (m *MessageService) ShowBasket(chatID int64) {
	if chatID == 0 {
		m.app.Log.Error("ShowBasket: chatID равен нулю")
		return
	}

	basketMu.Lock()
	purchases := append([]structs.Purchase(nil), structs.Baskets[chatID]...)
	basketMu.Unlock()

	msg := tgbotapi.NewMessage(chatID, BuildPurchaseReport(purchases))
	keyboard := m.createBasketKeyboard(purchases)
	msg.ReplyMarkup = keyboard

	sentMsg, err := m.bot.Send(msg)
	if err != nil {
		m.app.Log.Errorf("ShowBasket: не удалось показать корзину: chatID=%d, ошибка=%v", chatID, err)
		return
	}

	basketMu.Lock()
	BasketMessages[chatID] = sentMsg.MessageID
	basketMu.Unlock()
	m.trackMessage(chatID, sentMsg.MessageID)
}

// createBasketKeyboard формирует клавиатуру корзины.
func (m *MessageService) createBasketKeyboard(purchases []structs.Purchase) tgbotapi.InlineKeyboardMarkup {
	rows := make([][]tgbotapi.InlineKeyboardButton, 0, len(purchases)+2)

	for _, purchase := range purchases {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("➖", fmt.Sprintf("basket_minus_%d", purchase.ID)),
			tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("%d шт.", purchase.Quantity), "ignore"),
			tgbotapi.NewInlineKeyboardButtonData("➕", fmt.Sprintf("basket_plus_%d", purchase.ID)),
		))
	}

	if len(purchases) > 0 {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Оформить заказ", "send_basket_1_1"),
			tgbotapi.NewInlineKeyboardButtonData("Очистить корзину", "delete_basket_1_1"),
		))
	}

	defaultButtons := m.repo.GetDefaultButtonsM()
	defaultRow := make([]tgbotapi.InlineKeyboardButton, 0, len(defaultButtons))
	for _, button := range defaultButtons {
		defaultRow = append(defaultRow, tgbotapi.NewInlineKeyboardButtonData(button.Text, button.Callback))
	}
	if len(defaultRow) > 0 {
		rows = append(rows, defaultRow)
	}

	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

// changeBasketQuantity изменяет количество товара в корзине.
func (m *MessageService) changeBasketQuantity(chatID, itemID int64, delta int) ([]structs.Purchase, error) {
	if chatID == 0 {
		err := fmt.Errorf("некорректный chatID: %d", chatID)
		m.app.Log.Error("changeBasketQuantity", err.Error())
		return nil, err
	}
	basketMu.Lock()
	defer basketMu.Unlock()
	purchases, exists := structs.Baskets[chatID]
	if !exists {
		err := fmt.Errorf("корзина пользователя %d не найдена", chatID)
		m.app.Log.Error("changeBasketQuantity", err.Error())
		return nil, err
	}
	updated, _, err := changePurchaseQuantity(purchases, itemID, delta)
	if err != nil {
		m.app.Log.Errorf("changeBasketQuantity: chatID=%d, itemID=%d, delta=%d, ошибка=%v", chatID, itemID, delta, err)
		return nil, err
	}
	structs.Baskets[chatID] = updated
	return append([]structs.Purchase(nil), updated...), nil
}

// updateBasketMessage обновляет сообщение с корзиной после изменения количества.
func (m *MessageService) updateBasketMessage(update tgbotapi.Update, purchases []structs.Purchase) {
	if update.CallbackQuery == nil || update.CallbackQuery.Message == nil || update.CallbackQuery.Message.Chat == nil {
		m.app.Log.Error("updateBasketMessage: отсутствуют данные callback-сообщения")
		return
	}
	callback := update.CallbackQuery
	chatID := callback.Message.Chat.ID
	if chatID == 0 || callback.Message.MessageID == 0 {
		m.app.Log.Errorf("updateBasketMessage: некорректные идентификаторы: chatID=%d, messageID=%d", chatID, callback.Message.MessageID)
		return
	}

	edit := tgbotapi.NewEditMessageText(chatID, callback.Message.MessageID, BuildPurchaseReport(purchases))
	keyboard := m.createBasketKeyboard(purchases)
	edit.ReplyMarkup = &keyboard
	if _, err := m.bot.Send(edit); err != nil {
		m.app.Log.Errorf("updateBasketMessage: ошибка обновления корзины: chatID=%d, messageID=%d, ошибка=%v", chatID, callback.Message.MessageID, err)
		m.SendPopupMessage(update, "Не удалось обновить корзину")
		return
	}

	if callback.ID == "" {
		m.app.Log.Errorf("updateBasketMessage: пустой callback ID: chatID=%d", chatID)
		return
	}
	if _, err := m.bot.Request(tgbotapi.NewCallback(callback.ID, "Корзина обновлена")); err != nil {
		m.app.Log.Errorf("updateBasketMessage: ошибка ответа на callback: chatID=%d, ошибка=%v", chatID, err)
	}
}

// RequestDeliveryDetails Пример ввода данных заказа дата доставки.
func (m *MessageService) RequestDeliveryDetails(chatID int64) {
	if chatID == 0 {
		m.app.Log.Errorf(`нет данных для обработки chatID = %v`, chatID)
		return
	}

	// Удаление сообщения
	m.deleteMessages(chatID)

	DeliveryDate := `Дату доставки:`

	err := m.addOrUpdateOrderField(chatID, `DeliveryDate`, DeliveryDate)
	if err != nil {
		m.app.Log.Errorf(`Ошибка при добавлении или обновлении поля DeliveryDate: %v`, err)
		return
	}

	msg := tgbotapi.NewMessage(chatID, `Выберите дату доставки:`)
	now := time.Now()
	msg.ReplyMarkup = m.createCalendarKeyboard(now)

	sent, err := m.bot.Send(msg)
	if err != nil {
		m.app.Log.Errorf("RequestDeliveryDetails: не удалось отправить календарь: chatID=%d, ошибка=%v", chatID, err)
		return
	}
	m.trackMessage(chatID, sent.MessageID)

	deliveryDateMu.Lock()
	DeliveryDateChatID[chatID] = 1
	deliveryDateMu.Unlock()
}

// GroupMessagesChatID отправляет оформленный заказ в групповой чат.
func (m *MessageService) GroupMessagesChatID(cfg config.Config, chatID int64) {
	if chatID == 0 {
		m.app.Log.Errorf("GroupMessagesChatID: некорректный chatID=%d", chatID)
		return
	}

	orderDetailsMu.RLock()
	orderText := orderDetails[chatID]
	orderDetailsMu.RUnlock()

	if orderText == "" {
		m.sendMessage(chatID, "Нет данных для отправки")
		return
	}

	groupChatID, err := strconv.ParseInt(cfg.TGgrouptoken, 10, 64)
	if err != nil || groupChatID == 0 {
		m.app.Log.Errorf("GroupMessagesChatID: ошибка преобразования ID группового чата %q: %v", cfg.TGgrouptoken, err)
		m.sendMessage(chatID, "Заказ сохранён, но не удалось определить групповой чат операторов. Не оформляйте заказ повторно.")
		orderDetailsMu.Lock()
		delete(orderDetails, chatID)
		orderDetailsMu.Unlock()
		return
	}

	groupMessage := tgbotapi.NewMessage(groupChatID, orderText)
	var sendErr error
	for attempt := 1; attempt <= 3; attempt++ {
		if _, sendErr = m.bot.Send(groupMessage); sendErr == nil {
			break
		}
		m.app.Log.Errorf("GroupMessagesChatID: попытка %d/3 отправки заказа в группу не удалась: chatID=%d, groupChatID=%d, ошибка=%v", attempt, chatID, groupChatID, sendErr)
		if attempt < 3 {
			time.Sleep(time.Duration(attempt) * time.Second)
		}
	}
	if sendErr != nil {
		m.sendMessage(chatID, "Заказ сохранён, но уведомление оператору не отправилось. Не оформляйте заказ повторно; пожалуйста, свяжитесь с нами для уточнения.")
		orderDetailsMu.Lock()
		delete(orderDetails, chatID)
		orderDetailsMu.Unlock()
		return
	}
	m.app.Log.Infof("GroupMessagesChatID: заказ пользователя %d отправлен в группу %d", chatID, groupChatID)
	orderDetailsMu.Lock()
	delete(orderDetails, chatID)
	orderDetailsMu.Unlock()

	m.deleteMessages(chatID)
	m.sendMessage(chatID, "Ваш заказ успешно отправлен. При сборе заказа мы Вас уведомим.")
	m.ScheduleOnce(chatID)
}

// sendMessageWithKeyboard отправляет сообщение с inline-клавиатурой и сохраняет его ID для удаления.
func (m *MessageService) sendMessageWithKeyboard(chatID int64, text string, keyboard tgbotapi.InlineKeyboardMarkup) error {
	if chatID == 0 {
		err := fmt.Errorf("некорректный chatID: %d", chatID)
		m.app.Log.Error("sendMessageWithKeyboard", err.Error())
		return err
	}
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = keyboard
	sent, err := m.bot.Send(msg)
	if err != nil {
		m.app.Log.Errorf("sendMessageWithKeyboard: ошибка отправки сообщения: chatID=%d, ошибка=%v", chatID, err)
		return err
	}
	m.trackMessage(chatID, sent.MessageID)
	return nil
}

func ordersNavigationKeyboard(withRepeat bool) tgbotapi.InlineKeyboardMarkup {
	rows := [][]tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙Назад", "cat_Старт"),
			tgbotapi.NewInlineKeyboardButtonData("Стр 1", "cat_Стр 1"),
			tgbotapi.NewInlineKeyboardButtonData("📋 Мои заказы", "cat_Мои заказы"),
			tgbotapi.NewInlineKeyboardButtonData("🛒 Корзина", "cat_Корзина"),
		),
	}
	if withRepeat {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔁 Повторить последний заказ", "repeat_last"),
		))
	}
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

func truncateButtonText(value string, maxRunes int) string {
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value
	}
	if maxRunes <= 1 {
		return "…"
	}
	return string(runes[:maxRunes-1]) + "…"
}

// MainFavoritesByUserID показывает сохранённые пользователем товары.
func (m *MessageService) MainFavoritesByUserID(userID int64) {
	products, err := m.repo.GetFavoriteProducts(userID)
	if err != nil {
		m.app.Log.Errorf("MainFavoritesByUserID: ошибка получения избранного: userID=%d, ошибка=%v", userID, err)
		m.sendMessage(userID, "Не удалось загрузить избранное. Попробуйте позже.")
		return
	}

	var builder strings.Builder
	builder.WriteString("⭐ Избранные товары\n\n")
	rows := make([][]tgbotapi.InlineKeyboardButton, 0, len(products)+1)
	for _, product := range products {
		productTitle := formatProductTitle(product.Category, product.Title, "")
		fmt.Fprintf(&builder, "• %s, %s — %d руб.\n", productTitle, product.Packaging, product.Price)
		buttonText := truncateButtonText("⭐ "+productTitle+" "+product.Packaging, 60)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(buttonText, product.Callback),
			tgbotapi.NewInlineKeyboardButtonData("❌", fmt.Sprintf("fav_toggle_%d", product.ID)),
		))
	}
	if len(products) == 0 {
		builder.WriteString("Пока здесь ничего нет. Нажимайте ⭐ рядом с фасовкой товара, чтобы сохранить её.")
	}
	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🔙Назад", "cat_Старт"),
		tgbotapi.NewInlineKeyboardButtonData("📋 Мои заказы", "cat_Мои заказы"),
		tgbotapi.NewInlineKeyboardButtonData("🛒 Корзина", "cat_Корзина"),
	))
	m.deleteMessages(userID)
	if err := m.sendMessageWithKeyboard(userID, builder.String(), tgbotapi.NewInlineKeyboardMarkup(rows...)); err != nil {
		m.app.Log.Errorf("MainFavoritesByUserID: не удалось отправить избранное: userID=%d, ошибка=%v", userID, err)
	}
}

// MainOrdersByUserID Выводим заказы пользователя в Telegram.
func (m *MessageService) MainOrdersByUserID(userID int64) {
	if userID == 0 {
		m.app.Log.Errorf(`нет данных для обработки userID = %v`, userID)
		return
	}

	orders, err := m.repo.GetOrdersByUserID(userID, 6)

	if err != nil {
		m.app.Log.Errorf(`Error getting orders: %v`, err)
		return
	}

	if len(orders) > 0 {

		var msgText string
		var currentOrderID int64

		for i, order := range orders {
			t, err := dateutil.Parse(order.DeliveryDate)
			if err != nil {
				m.app.Log.Errorf(`Ошибка парсинга строки времени: %v`, err)
				return
			}

			// Форматирование времени в нужный формат
			formattedDeliveryDate := t.Format(`02.01.2006`)

			// Группировка по дате доставки
			if order.OrderID != currentOrderID {
				msgText += fmt.Sprintf(`Дата заказа: %v
`, order.DateTime)
				msgText += fmt.Sprintf(`Дата доставки: %v
`, formattedDeliveryDate)
				msgText += fmt.Sprintf(`Заказчик: %v
`, order.FullName)
				msgText += fmt.Sprintf(`Телефон для связи: %v
`, order.PhoneNumber)
				msgText += fmt.Sprintf(`%s %v
`, DeliveryAddress, order.DeliveryAddress)
				if strings.TrimSpace(order.Comment) != "" {
					msgText += fmt.Sprintf("Комментарий: %s\n", order.Comment)
				}
				msgText += `Список заказанных товаров :
`
				currentOrderID = order.OrderID
			}

			msgText += fmt.Sprintf(`%v кол-во: %v цена: %v руб.
`, order.Purchase.Item, order.Purchase.Quantity, order.Purchase.Cost)

			if i == len(orders)-1 || (i+1 < len(orders) && orders[i+1].OrderID != currentOrderID) {
				msgText += fmt.Sprintf(`Общая стоимость:  %v руб.
`, order.TotalCost)
				msgText += `------------------------------------------------------------
`

			}
		}
		m.deleteMessages(userID)
		if err := m.sendMessageWithKeyboard(userID, msgText, ordersNavigationKeyboard(true)); err != nil {
			m.app.Log.Errorf("MainOrdersByUserID: ошибка отправки заказов: userID=%d, ошибка=%v", userID, err)
		}

	} else {
		m.deleteMessages(userID)
		if err := m.sendMessageWithKeyboard(userID, `Вы ничего у нас не заказывали`, ordersNavigationKeyboard(false)); err != nil {
			m.app.Log.Errorf("MainOrdersByUserID: ошибка отправки пустого списка заказов: userID=%d, ошибка=%v", userID, err)
		}
	}
}

// addOrUpdateOrderField для добавления или обновления конкретного поля заказа.
func (m *MessageService) addOrUpdateOrderField(chatID int64, fieldName string, value interface{}) error {
	if chatID == 0 {
		m.app.Log.Errorf(`нет данных для обработки chatID = %v`, chatID)
		return errors.New(`нет chatID`)
	}

	bychat.OrdersMu.Lock()
	defer bychat.OrdersMu.Unlock()

	order, exists := bychat.Orders[chatID]

	if !exists {

		order = structs.OrderDb{}

	}

	val := reflect.ValueOf(&order).Elem()
	fieldVal := val.FieldByName(fieldName)

	if !fieldVal.IsValid() {

		m.app.Log.Error(`такого поля не существует`)
		return errors.New(`такого поля не существует`)

	}

	if fieldVal.CanSet() {

		fieldVal.Set(reflect.ValueOf(value))

	} else {

		m.app.Log.Error(`не удалось установить значение поля`)
		return errors.New(`не удалось установить значение поля`)

	}

	bychat.Orders[chatID] = order
	return nil
}

// sendMessage отправляет сообщение и регистрирует его для общей очистки.
func (m *MessageService) sendMessage(chatID int64, text string) {
	if chatID == 0 || text == `` {
		m.app.Log.Errorf(`нет данных для обработки chatID = %v text = %v`, chatID, text)
		return
	}

	msg := tgbotapi.NewMessage(chatID, text)
	sentMsg, err := m.bot.Send(msg)

	if err != nil {

		// Получаем имя вызывающей функции и номер строки
		pc, file, line, ok := runtime.Caller(1)

		if ok {
			funcName := runtime.FuncForPC(pc).Name()
			funcName = trimFuncName(funcName) // Укорачиваем имя функции
			file = trimFilePath(file)         // Укорачиваем путь к файлу
			m.app.Log.Errorf(`не удается отправить сообщение chatID = %v в функции %s (строка %d, файл %s): %v`, chatID, funcName, line, file, err)
		} else {
			m.app.Log.Errorf(`не удается отправить сообщение: %v`, err)
		}
		return

	}
	// Сохраняем сообщение в общем списке для последующей очистки.
	m.trackMessage(chatID, sentMsg.MessageID)

	// Удаляем предыдущие сообщения, если их больше 3
	//deleteMessages(bot, chatID)
}

// trimFuncName Функция для укорачивания полного имени функции.
func trimFuncName(fullFuncName string) string {
	parts := strings.Split(fullFuncName, `/`)
	shortName := parts[len(parts)-1]
	return shortName
}

// trimFilePath Функция для укорачивания пути к файлу.
func trimFilePath(fullFilePath string) string {
	parts := strings.Split(fullFilePath, `/`)
	if len(parts) > 2 {
		return strings.Join(parts[len(parts)-2:], `/`)
	}
	return fullFilePath
}

func (m *MessageService) ScheduleOnce(chatID int64) {
	go func() {
		time.Sleep(1 * time.Minute)
		// Удаление сообщения
		m.deleteMessages(chatID)

		basketMu.Lock()
		messageIDs := make(map[int]struct{})
		for _, messageID := range messagesToDelete[chatID] {
			messageIDs[messageID] = struct{}{}
		}
		if messageID := photoMessagesToDelete[chatID]; messageID != 0 {
			messageIDs[messageID] = struct{}{}
		}
		if messageID := keyboardMessagesToDelete[chatID]; messageID != 0 {
			messageIDs[messageID] = struct{}{}
		}
		delete(messagesToDelete, chatID)
		delete(photoMessagesToDelete, chatID)
		delete(keyboardMessagesToDelete, chatID)
		basketMu.Unlock()
		for messageID := range messageIDs {
			if _, err := m.bot.Request(tgbotapi.NewDeleteMessage(chatID, messageID)); err != nil {
				m.app.Log.Errorf("ScheduleOnce: ошибка удаления сообщения: chatID=%d, messageID=%d, ошибка=%v", chatID, messageID, err)
			}
		}

		orderDetailsMu.Lock()
		delete(orderDetails, chatID)
		orderDetailsMu.Unlock()

		commentPendingMu.Lock()
		delete(commentPending, chatID)
		commentPendingMu.Unlock()

		bychat.OrdersMu.Lock()
		delete(bychat.Orders, chatID)
		bychat.OrdersMu.Unlock()

		basketMu.Lock()
		delete(BasketMessages, chatID)
		basketMu.Unlock()

		deliveryDateMu.Lock()
		delete(DeliveryDateChatID, chatID)
		deliveryDateMu.Unlock()

		// Возвращаем кнопку «Старт» на сохранённое приветствие.
		// Если приветствия ещё нет, отправляем новое.
		basketMu.Lock()
		greetingID := greetingMessagesToKeep[chatID]
		basketMu.Unlock()

		if greetingID != 0 {
			edit := tgbotapi.NewEditMessageReplyMarkup(chatID, greetingID, createStartKeyboard())
			if _, editErr := m.bot.Request(edit); editErr != nil {
				m.app.Log.Errorf(
					"ScheduleOnce: не удалось вернуть кнопку «Старт»: chatID=%d, messageID=%d, ошибка=%v",
					chatID, greetingID, editErr,
				)
			} else {
				m.app.Log.Infof(
					"ScheduleOnce: кнопка «Старт» возвращена на приветствие: chatID=%d, messageID=%d",
					chatID, greetingID,
				)
			}
		} else {
			msg := tgbotapi.NewMessage(chatID, `Добро пожаловать! Нажмите 'Старт' для начала.`)
			msg.ReplyMarkup = createStartKeyboard()
			sent, sendErr := m.bot.Send(msg)
			if sendErr != nil {
				m.app.Log.Errorf("ScheduleOnce: ошибка отправки приветствия: chatID=%d, ошибка=%v", chatID, sendErr)
			} else {
				basketMu.Lock()
				greetingMessagesToKeep[chatID] = sent.MessageID
				basketMu.Unlock()
				m.app.Log.Infof("ScheduleOnce: приветствие отправлено: chatID=%d, messageID=%d", chatID, sent.MessageID)
			}
		}
	}()
}

// createCategoryMessage Вывод изображения продуктов и кнопки фасовки продукта.
func (m *MessageService) createCategoryMessage(update tgbotapi.Update, categoryName string) error {

	if update.Message.Chat.ID == 0 {
		m.app.Log.Errorf(`нет данных для обработки update.Message.Chat.ID = %v `, update.Message.Chat.ID)
		er := `нет данных для обработки update.Message.Chat.ID`
		return errors.New(er)
	}

	chatID := update.Message.Chat.ID
	// Поиск категории в данных
	categoryData, found := structs.Categories[categoryName]
	if !found {
		er := `категория не найдена`
		//m.app.Log.Errorf(" %v categoryData : %q", er, categoryName)
		return errors.New(er)
	}

	if categoryData.Photo == `` {
		keyboardReply, err := CreateCategoryKeyboardReply(categoryData.CategoryName)
		if err != nil {
			// Обработка ошибки
			m.app.Log.Errorf(`ошибка %v`, err)
			return err
		}

		// Создаем сообщение с инлайн-клавиатурой
		msg := tgbotapi.NewMessage(chatID, `Выбранная категория: `+categoryData.CategoryName)
		//msg := tgbotapi.NewMessage(chatID, ".")
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(keyboardReply...)

		// Отправляем сообщение с клавиатурой
		sentMsg, err := m.bot.Send(msg)
		if err != nil {
			m.app.Log.Errorf(`Ошибка при отправке сообщения: %v`, err)
			return err
		}
		m.trackMessage(chatID, sentMsg.MessageID)

	} else {
		// Создаем медиа-объект для изображения
		photo := tgbotapi.NewInputMediaPhoto(tgbotapi.FilePath(UploadsDir + `/` + categoryData.Photo))

		// Создаем медиагруппу с изображением и URL
		mediaGroup := tgbotapi.NewMediaGroup(chatID, []interface{}{photo})

		// Отправляем медиагруппу
		sentMsgs, err := m.bot.SendMediaGroup(mediaGroup)
		if err != nil {
			m.app.Log.Errorf(`Ошибка при отправке сообщения медиагруппу: %v`, err)
			return err
		}
		// Сохраняем идентификатор сообщения для последующего удаления
		if len(sentMsgs) > 0 {
			basketMu.Lock()
			photoMessagesToDelete[chatID] = sentMsgs[0].MessageID
			basketMu.Unlock()
			for _, sent := range sentMsgs {
				m.trackMessage(chatID, sent.MessageID)
			}
		}

		// Создаем кнопки клавиатуры
		keyboard, err := CreateCategoryKeyboard(categoryData.CategoryName)
		if err != nil {
			// Обработка ошибки
			m.app.Log.Errorf(`Ошибка не удается создать кнопки клавиатуры %v`, err)
			return err
		}

		// Создаем сообщение с инлайн-клавиатурой
		msg := tgbotapi.NewMessage(chatID, `Выберите нужное количество продукта: `+categoryData.CategoryMsg)
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(keyboard...)

		// Отправляем сообщение с клавиатурой
		sentMsg, err := m.bot.Send(msg)
		if err != nil {
			m.app.Log.Errorf(`Ошибка при отправке сообщения: %v`, err)
			return err
		}
		// Сохраняем идентификатор сообщения для последующего удаления
		m.trackMessage(chatID, sentMsg.MessageID)
		basketMu.Lock()
		keyboardMessagesToDelete[chatID] = sentMsg.MessageID // Сохраняем идентификатор сообщения с клавиатурой
		basketMu.Unlock()

	}
	//deletePreviousMessages(bot, update.Message.Chat.ID, update.Message.MessageID)
	return nil
}

// SeparateEmojiAndText Паттерн для поиска эмотзи update.Message.Text.
func SeparateEmojiAndText(text string) (string, string) {
	// Паттерн для поиска эмотзи
	emojiPattern := regexp.MustCompile("\\p{So}")
	// Ищем все эмотзи в тексте
	emojis := emojiPattern.FindAllString(text, -1)

	// Удаляем эмотзи из исходного текста
	cleanText := emojiPattern.ReplaceAllString(text, ``)
	// Удаляем лишние пробелы
	cleanText = regexp.MustCompile("\\s+").ReplaceAllString(cleanText, ` `)

	return joinStrings(emojis), cleanText
}

// joinStrings Объединяем все строки из среза в одну строку из update.Message.Text.
func joinStrings(str []string) string {
	// Объединяем все строки из среза в одну строку
	result := ``
	for _, s := range str {
		result += s
	}
	return result
}
