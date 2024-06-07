package routers

import (
	"ProjectTelBotMark/dataSources"
	"ProjectTelBotMark/loggers"
	"ProjectTelBotMark/structs"
	"github.com/gin-gonic/gin"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/gookit/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// router создаем роутер.
var router *gin.Engine

var logger *slog.Logger // Глобальный логгер

// GinRouter Gin Router функция запускает сервер.
func GinRouter(bot *tgbotapi.BotAPI) {
	// Настраиваем логгер
	logger = loggers.SetupLogger()
	gin.SetMode(gin.ReleaseMode)
	router = gin.Default()
	router.Static("/assets/", "front/")
	router.LoadHTMLGlob("html/templates/*")
	router.GET("/", HandlerIndex)
	router.GET("/info", HandlerInfo)
	router.POST("/info", PostHandlerInfo(bot))
	router.GET("/purchases", PurchasesHandler)
	router.GET("/purchase", func(c *gin.Context) {
		c.HTML(http.StatusOK, "purchases.html", gin.H{
			"Title":    "Заказы Салаты корейские",
			"Classreg": "current"})
	})

	err := router.Run(":8090")
	if err != nil {
		logger.Errorf("GinRouter %v", err)
		return
	}
}

// HandlerInfo код страницы инфо.
func HandlerInfo(c *gin.Context) {

	c.HTML(200, "upload.html", gin.H{
		"Title":    "Заказы Салаты корейские",
		"Classupl": "current"})

}

// PostHandlerInfo отправка сообщений в телеграмм инфо.
func PostHandlerInfo(bot *tgbotapi.BotAPI) gin.HandlerFunc {
	return func(c *gin.Context) {
		var messageInfo structs.MessageInfo

		// Bind form data to struct
		err := c.Bind(&messageInfo)
		if err != nil {

			logger.Errorf("PostHandlerInfo Bind: %v", err)
			c.HTML(200, "upload.html", gin.H{
				"Title":    "Заказы Салаты корейские",
				"Message":  "Ошибка обработки формы",
				"Classupl": "current"})
			return

		}

		// Check if Info and InfoText are not empty
		if messageInfo.Info == "" || messageInfo.InfoText == "" {

			c.HTML(200, "upload.html", gin.H{
				"Title":    "Заказы Салаты корейские",
				"Message":  "Поля Тип информации и Сообщение для отправки не должны быть пустыми ",
				"Classupl": "current"})
			return
		}

		// Get uploaded file
		file, err := c.FormFile("Image")
		if err != nil && err != http.ErrMissingFile {

			logger.Errorf("PostHandlerInfo FormFile: %v", err)
			c.HTML(200, "upload.html", gin.H{
				"Title":    "Заказы Салаты корейские",
				"Message":  "Ошибка загрузки файла",
				"Classupl": "current"})
			return

		}

		// Prepare message text
		msgText := messageInfo.Info + "\n" + messageInfo.InfoText + "\n"

		// Get user IDs
		users := dataSources.UserID()

		if users != nil {

			for _, user := range users {

				// Send text message
				message := tgbotapi.NewMessage(user.UserID, msgText)

				_, err = bot.Send(message)
				if err != nil {

					logger.Errorf("PostHandlerInfo SendText: %v", err)
					c.HTML(200, "upload.html", gin.H{
						"Title":    "Заказы Салаты корейские",
						"Message":  "Произошла ошибка при отправке текстового сообщения, попробуйте еще раз",
						"Classupl": "current"})
					return

				}

				// Send image if uploaded
				if file != nil {

					savePath := "./uploads/"
					if _, err := os.Stat(savePath); os.IsNotExist(err) {

						err = os.MkdirAll(savePath, os.ModePerm)
						if err != nil {

							logger.Errorf("PostHandlerInfo Mkdir: %v", err)
							c.HTML(200, "upload.html", gin.H{
								"Title":    "Заказы Салаты корейские",
								"Message":  "Произошла ошибка при создании директории для сохранения изображения",
								"Classupl": "current"})
							return

						}
					}

					filePath := filepath.Join(savePath, file.Filename)
					err := c.SaveUploadedFile(file, filePath)
					if err != nil {

						logger.Errorf("PostHandlerInfo SaveFile: %v", err)
						c.HTML(200, "upload.html", gin.H{
							"Title":    "Заказы Салаты корейские",
							"Message":  "Произошла ошибка при сохранении изображения, попробуйте еще раз",
							"Classupl": "current"})
						return

					}

					// Создаем медиа-объект для изображения
					photo := tgbotapi.NewInputMediaPhoto(tgbotapi.FilePath(filePath))

					// Создаем медиагруппу с изображением и URL
					mediaGroup := tgbotapi.NewMediaGroup(user.UserID, []interface{}{photo})
					_, err = bot.SendMediaGroup(mediaGroup)
					if err != nil {

						logger.Errorf("PostHandlerInfo SendPhoto: %v", err)
						c.HTML(200, "upload.html", gin.H{
							"Title":    "Заказы Салаты корейские",
							"Message":  "Произошла ошибка при отправке изображения, попробуйте еще раз",
							"Classupl": "current"})
						return

					}
				}
			}

			c.HTML(200, "upload.html", gin.H{
				"Title":    "Заказы Салаты корейские",
				"Message":  "Извещение успешно отправлено",
				"Classupl": "current"})
		} else {

			c.HTML(200, "upload.html", gin.H{
				"Title":    "Заказы Салаты корейские",
				"Message":  "Пока некому отправлять",
				"Classupl": "current"})
		}
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
		//StartDate: time.Now().AddDate(0, 0, -1).Format("2006-01-02"),
		//EndDate:   time.Now().Format("2006-01-02"),
		OrderBy:  "tg_user_orders_date.delivery_date",
		OrderDir: "DESC",
	}

	orders, err := dataSources.GetOrdersWithFilters(dataSources.DB, params)
	if err != nil {
		logger.Errorf("ошибка запрос к базе %v", err)
		return
	}
	//fmt.Println("orders", orders)

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
			t, err := time.Parse(time.RFC3339, order.DeliveryDate)
			if err != nil {
				logger.Errorf("Ошибка парсинга строки времени: %v", err)
				return
			}

			// Форматирование времени в нужный формат
			formattedDeliveryDate := t.Format("02.01.2006")

			if formattedDeliveryDate == now {
				payload.CurrentDate = true
			}

			if order.DateTime != currentDateTime2 {
				currentDateTime2 = order.DateTime
				num = 1
			} else {
				num++
			}
			// Группировка по дате доставки
			if order.UserID.UserID != currentUserID || formattedDeliveryDate != currentDeliveryDate || order.DateTime != currentDateTime {
				payload.OrderDb.DateTime = order.DateTime
				payload.OrderDb.DeliveryDate = formattedDeliveryDate
				payload.OrderDb.FullName = order.FullName
				payload.OrderDb.PhoneNumber = order.PhoneNumber
				payload.OrderDb.DeliveryAddress = order.DeliveryAddress
				currentDeliveryDate = formattedDeliveryDate
				currentUserID = order.UserID.UserID
				currentDateTime = order.DateTime
				payload.OrderDb.TotalCost = order.TotalCost
				Index++
				payload.Index = Index
			}

			payload.OrderDb.Purchase.Item = order.Purchase.Item
			payload.OrderDb.Purchase.Quantity = order.Purchase.Quantity
			payload.OrderDb.Purchase.Cost = order.Purchase.Cost

			if i == len(orders)-1 || (i+1 < len(orders) && orders[i+1].DateTime != currentDateTime2) {
				webnum.Num = num
				webnums = append(webnums, webnum)
			}

			payloads = append(payloads, payload)
		}
	}

	rty := MergeSlidesWithDeliveryDate(payloads, webnums)

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
		if order.OrderDb.DeliveryDate != "" {
			order.Num = nums[numIndex].Num
			numIndex++
		}
		result = append(result, order)
	}

	return result
}

func PurchasesHandler(c *gin.Context) {
	period := c.Query("period")

	var startDate, endDate string

	switch period {
	case "day":
		startDate = time.Now().AddDate(0, 0, -1).Format("2006-01-02")
		endDate = time.Now().Format("2006-01-02")
	case "week":
		startDate = time.Now().AddDate(0, 0, -7).Format("2006-01-02")
		endDate = time.Now().Format("2006-01-02")
	case "month":
		startDate = time.Now().AddDate(0, -1, 0).Format("2006-01-02")
		endDate = time.Now().Format("2006-01-02")
	case "year":
		startDate = time.Now().AddDate(-1, 0, 0).Format("2006-01-02")
		endDate = time.Now().Format("2006-01-02")
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid period"})
		return
	}

	orders, err := dataSources.GetOrdersByDeliveryDateRange(dataSources.DB, startDate, endDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	labels := []string{}
	amounts := []int{}
	for _, order := range orders {
		// Парсинг строки времени в тип time.Time
		t, err := time.Parse(time.RFC3339, order.DeliveryDate)
		if err != nil {
			logger.Errorf("Ошибка парсинга строки времени: %v", err)
			return
		}

		// Форматирование времени в нужный формат
		formattedDeliveryDate := t.Format("2006-01-02")
		labels = append(labels, formattedDeliveryDate)
		amounts = append(amounts, order.TotalCost)
	}

	c.JSON(http.StatusOK, gin.H{
		"labels":  labels,
		"amounts": amounts,
	})
}
