package reminders

import (
	"ProjectTelBotMark/config"
	"ProjectTelBotMark/internal/botTelegram"
	"ProjectTelBotMark/internal/dataSources"
	"ProjectTelBotMark/internal/dateutil"
	"ProjectTelBotMark/internal/structs"
	"fmt"
	"strconv"
	"time"

	slogavp "github.com/Anatoly8853/slog-avp/v2"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type MessageService struct {
	repo *dataSources.Repository
	app  *slogavp.Application
	bot  *tgbotapi.BotAPI
}

func NewMessageService(repo *dataSources.Repository, app *slogavp.Application, bot *tgbotapi.BotAPI) *MessageService {
	return &MessageService{repo: repo, app: app, bot: bot}
}

// CheckDatabase функция для проверки базы данных на совпадение по дате.
func (m *MessageService) CheckDatabase() {

	now := time.Now().Format("02.01.2006") // формат даты в виде DD.MM.YYYY

	params := structs.QueryParams{
		StartDate: time.Now().Format("2006-01-02"),
		EndDate:   time.Now().Format("2006-01-02"),
		OrderBy:   "tg_user_orders_date.delivery_date",
		OrderDir:  "DESC",
		//OrderDir: "ASC",
	}

	orders, err := m.repo.GetOrdersWithFilters(params)
	if err != nil {
		m.app.Log.Errorf("ошибка запрос к базе %v", err)
		return
	}

	if len(orders) > 0 {
		var currentOrderID int64
		var msgText string

		for i, order := range orders {
			// Парсинг строки времени в тип time.Time
			t, err := dateutil.Parse(order.DeliveryDate)
			if err != nil {
				m.app.Log.Errorf("Ошибка парсинга строки времени: %v", err)
				return
			}

			// Форматирование времени в нужный формат
			formattedDeliveryDate := t.Format("02.01.2006")

			if formattedDeliveryDate == now {
				// Группировка по дате доставки
				if order.OrderID != currentOrderID {
					//msgText += fmt.Sprintf("Дата заказа: %v\n", formattedDate)
					msgText += fmt.Sprintf("Дата доставки: %v\n", formattedDeliveryDate)
					msgText += fmt.Sprintf("Заказчик: %v\n", order.FullName)
					msgText += fmt.Sprintf("Телефон для связи: %v\n", order.PhoneNumber)
					msgText += fmt.Sprintf("%s %v\n", botTelegram.DeliveryAddress, order.DeliveryAddress)
					msgText += "Список заказанных товаров :\n"
					currentOrderID = order.OrderID
				}

				msgText += fmt.Sprintf("%v кол-во: %v цена: %v руб.\n", order.Purchase.Item, order.Purchase.Quantity, order.Purchase.Cost)

				if i == len(orders)-1 || (i+1 < len(orders) && orders[i+1].OrderID != currentOrderID) {
					msgText += fmt.Sprintf("Общая стоимость:  %v руб.\n", order.TotalCost)
					msgText += "------------------------------------------------------------\n"

					// Отправляем сообщение в групповой чат
					// Получите ID группового чата из переменной окружения

				}
			}
		}
		if msgText != "" {
			m.sendOrderMessage(msgText)
		}
	} else {
		msgText := fmt.Sprintf("На %s заказов нет, отдыхаем", now)
		m.sendOrderMessage(msgText)
	}
}

// StartDailyCheck запускает Go-рутину для ежедневной проверки базы данных.
func (m *MessageService) StartDailyCheck() {
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
			m.CheckDatabase()

		}
	}()
}

// sendOrderMessage отправляет сообщение с заказом
func (m *MessageService) sendOrderMessage(msgText string) {
	cfg := config.LoadConfig(m.app)
	//groupChatIDStr := os.Getenv("TELEGRAM_GROUPCHATID")
	//groupChatID, err := strconv.ParseInt(groupChatIDStr, 10, 64)
	groupChatID, err := strconv.ParseInt(cfg.TGmesagetoken, 10, 64)
	if err != nil {
		m.app.Log.Errorf("Ошибка преобразования ID группового чата: %v", err)
		return
	}

	message := tgbotapi.NewMessage(groupChatID, msgText)
	_, err = m.bot.Send(message)
	if err != nil {
		m.app.Log.Errorf("%v", err)
	}
}
