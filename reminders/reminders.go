package reminders

import (
	"ProjectTelBotMark/dataSources"
	"ProjectTelBotMark/structs"
	"fmt"
	"os"
	"strconv"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// CheckDatabase функция для проверки базы данных на совпадение по дате.
func CheckDatabase(app *structs.Application, bot *tgbotapi.BotAPI) {

	now := time.Now().Format("02.01.2006") // формат даты в виде DD.MM.YYYY

	params := structs.QueryParams{
		StartDate: time.Now().Format("2006-01-02"),
		EndDate:   time.Now().Format("2006-01-02"),
		OrderBy:   "tg_user_orders_date.delivery_date",
		OrderDir:  "DESC",
		//OrderDir: "ASC",
	}

	orders, err := dataSources.GetOrdersWithFilters(app, params)
	if err != nil {
		app.Log.Errorf("ошибка запрос к базе %v", err)
		return
	}

	if len(orders) > 0 {
		var currentDeliveryDate string // Объявляем переменную для отслеживания текущей даты доставки
		var currentUserID int64
		var currentDateTime string
		var msgText string

		for i, order := range orders {

			// Парсинг строки времени в тип time.Time
			t, err := time.Parse(time.RFC3339, order.DeliveryDate)
			if err != nil {
				app.Log.Errorf("Ошибка парсинга строки времени: %v", err)
				return
			}

			// Форматирование времени в нужный формат
			formattedDeliveryDate := t.Format("02.01.2006")

			if formattedDeliveryDate == now {

				// Группировка по дате доставки
				if order.UserID.UserID != currentUserID || formattedDeliveryDate != currentDeliveryDate || order.DateTime != currentDateTime {
					//msgText += fmt.Sprintf("Дата заказа: %v\n", formattedDate)
					msgText += fmt.Sprintf("Дата доставки: %v\n", formattedDeliveryDate)
					msgText += fmt.Sprintf("Заказчик: %v\n", order.FullName)
					msgText += fmt.Sprintf("Телефон для связи: %v\n", order.PhoneNumber)
					msgText += fmt.Sprintf("Адрес доставки: %v\n", order.DeliveryAddress)
					msgText += "Список заказанных товаров :\n"
					currentDeliveryDate = formattedDeliveryDate
					currentDateTime = order.DateTime
					currentUserID = order.UserID.UserID
				}

				msgText += fmt.Sprintf("%v кол-во: %v цена: %v руб.\n", order.Purchase.Item, order.Purchase.Quantity, order.Purchase.Cost)

				if i == len(orders)-1 || (i+1 < len(orders) && orders[i+1].DateTime != currentDateTime) {
					msgText += fmt.Sprintf("Общая стоимость:  %v руб.\n", order.TotalCost)
					msgText += "------------------------------------------------------------\n"

					// Отправляем сообщение в групповой чат
					// Получите ID группового чата из переменной окружения

				}
			}
		}
		if msgText != "" {
			sendOrderMessage(bot, app, msgText)
		}
	} else {
		msgText := fmt.Sprintf("На %s заказов нет, отдыхаем", now)
		sendOrderMessage(bot, app, msgText)
	}
}

// StartDailyCheck запускает Go-рутину для ежедневной проверки базы данных.
func StartDailyCheck(app *structs.Application, bot *tgbotapi.BotAPI) {
	go func() {
		for {
			now := time.Now()

			// Устанавливаем время следующего запуска на 8:00 утра текущего дня
			next := time.Date(now.Year(), now.Month(), now.Day(), 13, 42, 0, 0, now.Location())

			// Если текущее время уже после 8:00 утра, то устанавливаем следующий запуск на 8:00 утра следующего дня
			if now.After(next) {
				next = next.Add(24 * time.Hour)
			}

			// Создаем таймер до следующего запуска
			duration := time.Until(next)
			timer := time.NewTimer(duration)
			<-timer.C

			// Выполняем проверку базы данных
			CheckDatabase(app, bot)

		}
	}()
}

// sendOrderMessage отправляет сообщение с заказом
func sendOrderMessage(bot *tgbotapi.BotAPI, app *structs.Application, msgText string) {
	groupChatIDStr := os.Getenv("TELEGRAM_GROUPCHATID")
	groupChatID, err := strconv.ParseInt(groupChatIDStr, 10, 64)
	if err != nil {
		app.Log.Errorf("Ошибка преобразования ID группового чата: %v", err)
		return
	}

	message := tgbotapi.NewMessage(groupChatID, msgText)
	_, err = bot.Send(message)
	if err != nil {
		app.Log.Errorf("%v", err)
	}
}
