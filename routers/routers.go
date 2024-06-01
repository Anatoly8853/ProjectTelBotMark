package routers

import (
	"ProjectTelBotMark/dataSources"
	"ProjectTelBotMark/loggers"
	"ProjectTelBotMark/structs"
	"github.com/gin-gonic/gin"
	"github.com/gookit/slog"
	"sort"
	"time"
)

// router создаем роутер.
var router *gin.Engine

var logger *slog.Logger // Глобальный логгер

// GinRouter Gin Router функция запускает сервер.
func GinRouter() {
	// Настраиваем логгер
	logger = loggers.SetupLogger()
	gin.SetMode(gin.ReleaseMode)
	router = gin.Default()
	router.Static("/assets/", "front/")
	router.LoadHTMLGlob("html/templates/*")
	router.GET("/", HandlerIndex)
	err := router.Run(":8090")
	if err != nil {
		logger.Errorf("GinRouter %v", err)
		return
	}
}

/*
// Получить все заказы, отсортированные по дате по убыванию
params := QueryParams{
OrderBy:  "tg_user_orders_date.date_time",
OrderDir: "DESC",
}
orders, err := GetOrdersWithFilters(db, params)

// Получить заказы для определенного пользователя за указанный период времени
params := QueryParams{
UserID:    123,
StartDate: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
EndDate:   time.Date(2023, 12, 31, 23, 59, 59, 0, time.UTC),
OrderBy:   "tg_user_orders_date.date_time",
OrderDir:  "ASC",
}
orders, err := GetOrdersWithFilters(db, params)
*/

// HandlerIndex Главная страница с выводом всех значений из базы.
func HandlerIndex(c *gin.Context) {
	// Настраиваем логгер
	logger = loggers.SetupLogger()

	now := time.Now().Format("02.01.2006") // формат даты в виде DD.MM.YYYY

	params := structs.QueryParams{
		//OrderBy: "tg_user_orders_date.date_time",
		OrderBy:  "tg_user_orders_date.delivery_date",
		OrderDir: "DESC",
		//OrderDir: "ASC",
	}

	orders, err := dataSources.GetOrdersWithFilters(dataSources.DB, params)
	if err != nil {
		logger.Errorf("ошибка запрос к базе %v", err)
		return
	}

	sort.Sort(ByDeliveryDate(orders))

	var payloads []structs.OrderInfo
	var webnums []structs.Webnums

	if len(orders) > 0 {
		var currentDeliveryDate string // Объявляем переменную для отслеживания текущей даты доставки
		var currentUserID int64
		var currentDateTime string
		var currentDateTime2 string
		var num int
		var Index int

		for i, order := range orders {
			var payload structs.OrderInfo
			var webnum structs.Webnums

			// Парсинг строки времени в тип time.Time
			t, err := time.Parse("02.01.2006", order.DeliveryDate)
			if err != nil {
				logger.Errorf("Ошибка парсинга строки времени: %v", err)
				return
			}

			// Форматирование времени в нужный формат
			formattedDate := t.Format("02.01.2006")

			if formattedDate == now {
				payload.CurrentDate = true
			}

			if order.DateTime != currentDateTime2 {
				currentDateTime2 = order.DateTime
				num = 1
			} else {
				num++
			}

			// Группировка по дате доставки

			if order.UserID.UserID != currentUserID || order.DeliveryDate != currentDeliveryDate || order.DateTime != currentDateTime {
				payload.DateTime = order.DateTime
				payload.DeliveryDate = order.DeliveryDate
				payload.FullName = order.FullName
				payload.PhoneNumber = order.PhoneNumber
				payload.DeliveryAddress = order.DeliveryAddress
				currentDeliveryDate = order.DeliveryDate
				currentUserID = order.UserID.UserID
				currentDateTime = order.DateTime
				payload.TotalCost = order.TotalCost
				Index++
				payload.Index = Index
			}

			payload.Purchase.Item = order.Purchase.Item
			payload.Purchase.Quantity = order.Purchase.Quantity
			payload.Purchase.Cost = order.Purchase.Cost

			if i == len(orders)-1 || (i+1 < len(orders) && orders[i+1].DateTime != currentDateTime2) {
				webnum.Num = num
				webnums = append(webnums, webnum)
			}

			payloads = append(payloads, payload)
		}
	}

	rty := MergeSlidesWithDeliveryDate(payloads, webnums)

	// Здесь должен быть ваш код для получения данных, например payload
	/*
		data := map[string]interface{}{
			"Title":       "Заказы Салаты корейские",
			"Classind":    "current",
			"payload":     rty, // предполагаем, что payload это данные, которые вы хотите отобразить
			"currentDate": now,
		}

	*/
	//fmt.Println(rty)

	c.HTML(200, "index.html", gin.H{
		"Title":    "Заказы Салаты корейские",
		"Classind": "current",
		"payload":  rty})
}

// MergeSlidesWithDeliveryDate объединяем два слайда для добавления к основному слайду Num.
func MergeSlidesWithDeliveryDate(orders []structs.OrderInfo, nums []structs.Webnums) []structs.OrderInfo {
	var result []structs.OrderInfo
	numIndex := 0

	for _, order := range orders {
		if order.DeliveryDate != "" {
			order.Num = nums[numIndex].Num
			numIndex++
		}
		result = append(result, order)
	}

	return result
}

type ByDeliveryDate []structs.OrderInfo

func (a ByDeliveryDate) Len() int      { return len(a) }
func (a ByDeliveryDate) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByDeliveryDate) Less(i, j int) bool {
	layout := "02.01.2006"
	dateTime1, _ := time.Parse(layout, a[i].DeliveryDate)
	dateTime2, _ := time.Parse(layout, a[j].DeliveryDate)
	return dateTime1.After(dateTime2)
}
