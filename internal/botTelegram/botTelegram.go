package botTelegram

import (
	"ProjectTelBotMark/config"
	"ProjectTelBotMark/internal/bychat"
	"ProjectTelBotMark/internal/dataSources"
	"ProjectTelBotMark/internal/structs"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
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

var DeliveryDateChatID = make(map[int64]int)

// orderDetails содержание корзины.
var orderDetails = make(map[int64]string)

// BuildPurchaseReport функция, которая объединяет покупки с одинаковым товаром и выводит итоговую информацию.
func BuildPurchaseReport(purchases []structs.Purchase) string {
	if len(purchases) == 0 {
		return "Покупок нет."
	}

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
		msgText += fmt.Sprintf(
			"Товар: %s кол-во: %d цена: %d руб.\n",
			p.Item,
			p.Quantity,
			p.Cost,
		)

		totalCost += p.Cost
	}

	msgText += fmt.Sprintf("\nОбщая стоимость: %d руб.", totalCost)

	return msgText
}

func NewUpdateMessage(repo *dataSources.Repository, app *slogavp.Application, bot *tgbotapi.BotAPI) *MessageService {
	return &MessageService{
		repo:           repo,
		app:            app,
		bot:            bot,
		basketMessages: make(map[int64]int),
	}
}

// AddToBasket добавление товаров в корзину.
func (m *MessageService) AddToBasket(update tgbotapi.Update, item string, quantity int, cost int) {
	if update.CallbackQuery.Message.Chat.ID == 0 {
		m.app.Log.Errorf(`нет данных для обработки update.CallbackQuery.Message.Chat.ID = %v `, update.CallbackQuery.Message.Chat.ID)
		return
	}

	if item == `` || quantity == 0 || cost == 0 {
		m.app.Log.Errorf(`нет данных для обработки item = %v quantity = %v cost = %v`, item, quantity, cost)
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
	popupText := fmt.Sprintf(`Добавлено в корзину:
 %s, кол-во: %d, цена: %d руб.
 Общая стоимость: %d руб`, item, quantity, cost, totalCost)

	m.SendPopupMessage(update, popupText)
}

// SendPopupMessage Отправка всплывающего сообщения.
func (m *MessageService) SendPopupMessage(update tgbotapi.Update, popupText string) {
	if update.CallbackQuery.ID == `` || update.CallbackQuery.Message == nil || popupText == `` {
		m.app.Log.Errorf(`нет данных для обработки update.CallbackQuery.Message = %v popupText = %v`, update.CallbackQuery.Message, popupText)
		return
	}
	callbackConfig := tgbotapi.NewCallback(update.CallbackQuery.ID, popupText)
	callbackConfig.ShowAlert = true // Для отображения в виде всплывающего уведомления

	if _, err := m.bot.Request(callbackConfig); err != nil {
		m.app.Log.Errorf(`не удается создать всплывающее сообщение %v`, err)
		return
	}
}

// ShowBasket Получение содержимого корзины.
func (m *MessageService) ShowBasket(chatID int64) {
	purchases := structs.Baskets[chatID]
	msg := tgbotapi.NewMessage(chatID, `Вы пока ничего не добавили в корзину`)

	msgText := BuildPurchaseReport(purchases)
	msg = tgbotapi.NewMessage(chatID, msgText)

	btnSend := tgbotapi.NewInlineKeyboardButtonData(`Оформить заказ`, `send_basket_1_1`)
	btnDelete := tgbotapi.NewInlineKeyboardButtonData(`Очистить корзину`, `delete_basket_1_1`)

	row := tgbotapi.NewInlineKeyboardRow(btnSend, btnDelete)
	inlineKeyboard := tgbotapi.NewInlineKeyboardMarkup(row)
	msg.ReplyMarkup = inlineKeyboard

	sentMsg, err := m.bot.Send(msg)
	if err != nil {
		m.app.Log.Errorf("не удается вывести содержимое корзины %v", err)
		return
	}

	BasketMessages[chatID] = sentMsg.MessageID
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

	_, err = m.bot.Send(msg)
	if err != nil {
		m.app.Log.Errorf(`не удается вывести содержимое корзины %v`, err)
		return
	}

	DeliveryDateChatID[chatID] = 1
}

// GroupMessagesChatID отправляем заказ.
func (m *MessageService) GroupMessagesChatID(cfg config.Config, chatID int64) {
	if chatID == 0 {
		m.app.Log.Errorf(`нет данных для обработки chatID = %v`, chatID)
		return
	}

	if orderDetails[chatID] != `` {

		// Получите ID группового чата из переменной окружения
		//groupChatIDStr := os.Getenv("TELEGRAM_GROUPCHATID")
		//groupChatID, err := strconv.ParseInt(groupChatIDStr, 10, 64)
		groupChatID, err := strconv.ParseInt(cfg.TGgrouptoken, 10, 64)

		if err != nil {

			m.app.Log.Errorf(`Ошибка преобразования ID группового чата: %v`, err)
			return

		}

		// Отправляем сообщение в групповой чат.
		m.sendMessage(groupChatID, orderDetails[chatID])
		delete(orderDetails, groupChatID)

		// Удаление сообщения
		m.deleteMessages(chatID)

		//отправляем сообщение.
		m.sendMessage(chatID, `Ваш заказ успешно отправлен при сборе заказа мы Вас уведомим`)
		m.ScheduleOnce(chatID)

	} else {

		//отправляем сообщение.
		m.sendMessage(chatID, `Нет данных для отправки`)

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
		var currentDeliveryDate string // Объявляем переменную для отслеживания текущей даты доставки
		var currentDateTime string

		for i, order := range orders {

			// Форматирование даты заказа
			// Парсинг строки времени в тип time.Time
			t, err := time.Parse(time.RFC3339, order.DeliveryDate)
			if err != nil {
				m.app.Log.Errorf(`Ошибка парсинга строки времени: %v`, err)
				return
			}

			// Форматирование времени в нужный формат
			formattedDeliveryDate := t.Format(`02.01.2006`)

			// Группировка по дате доставки
			if formattedDeliveryDate != currentDeliveryDate || order.DateTime != currentDateTime {
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
				msgText += `Список заказанных товаров :
`
				currentDeliveryDate = formattedDeliveryDate
				currentDateTime = order.DateTime
			}

			msgText += fmt.Sprintf(`%v кол-во: %v цена: %v руб.
`, order.Purchase.Item, order.Purchase.Quantity, order.Purchase.Cost)

			if i == len(orders)-1 || (i+1 < len(orders) && orders[i+1].DateTime != currentDateTime) {
				msgText += fmt.Sprintf(`Общая стоимость:  %v руб.
`, order.TotalCost)
				msgText += `------------------------------------------------------------
`

			}
		}
		//отправляем сообщение
		m.sendMessage(userID, msgText)

	} else {
		//отправляем сообщение
		m.sendMessage(userID, `Вы ничего у нас не заказывали`)
	}
}

// addOrUpdateOrderField для добавления или обновления конкретного поля заказа.
func (m *MessageService) addOrUpdateOrderField(chatID int64, fieldName string, value interface{}) error {
	if chatID == 0 {
		m.app.Log.Errorf(`нет данных для обработки chatID = %v`, chatID)
		return errors.New(`нет chatID`)
	}

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

// sendMessage Функция для отправки сообщения и добавления его в BasketMessages
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
	// Сохраняем идентификатор отправленного сообщения для последующего удаления.
	// Добавляем новое сообщение в мапу.
	BasketMessages[chatID] = sentMsg.MessageID

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

		for chatIDs, messageIDs := range messagesToDelete {
			for _, messageIDa := range messageIDs {
				deleteMsg := tgbotapi.NewDeleteMessage(chatIDs, messageIDa)
				if _, err := m.bot.Request(deleteMsg); err != nil {
					//log.Printf("Ошибка при удалении сообщения: %v", err)
					// Не используйте log.Panic, чтобы избежать прерывания работы бота из-за одной ошибки
				}
			}
		}

		delete(orderDetails, chatID)

		delete(bychat.Orders, chatID)

		delete(BasketMessages, chatID)

		// Возвращаемся в основное меню
		msg := tgbotapi.NewMessage(chatID, `Добро пожаловать! Нажмите 'Старт' для начала.`)
		msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(
			tgbotapi.NewKeyboardButtonRow(
				tgbotapi.NewKeyboardButton(`Старт`),
			),
		)
		_, err := m.bot.Send(msg)
		if err != nil {
			m.app.Log.Errorf(`Error sending message: = %v`, err)
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

	m.photoMessagesDelete(chatID)
	m.keyboardMessagesDelete(chatID)

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
		msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(keyboardReply...)

		// Отправляем сообщение с клавиатурой
		sentMsg, err := m.bot.Send(msg)
		if err != nil {
			m.app.Log.Errorf(`Ошибка при отправке сообщения: %v`, err)
			return err
		}
		messagesToDelete[chatID] = append(messagesToDelete[chatID], sentMsg.MessageID)

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
			photoMessagesToDelete[chatID] = sentMsgs[0].MessageID
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
		messagesToDelete[chatID] = append(messagesToDelete[chatID], sentMsg.MessageID)
		keyboardMessagesToDelete[chatID] = sentMsg.MessageID // Сохраняем идентификатор сообщения с клавиатурой

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
