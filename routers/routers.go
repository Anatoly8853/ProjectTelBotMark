package routers

import (
	"ProjectTelBotMark/dataSources"
	"ProjectTelBotMark/structs"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	`sort`
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/goccy/go-json"
	`github.com/gookit/slog`
)

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

type Application struct {
	Log *slog.Logger
}

// Data представляет все данные из JSON файла
type Data map[string]structs.Category

// router создаем роутер.
var router *gin.Engine

// GinRouter Gin Router функция запускает сервер.
func GinRouter(app *structs.Application, bot *tgbotapi.BotAPI) {

	gin.SetMode(gin.ReleaseMode)
	router = gin.Default()
	router.SetFuncMap(template.FuncMap{
		"toJSON": func(v interface{}) string {
			a, _ := json.Marshal(v)
			return string(a)
		},
	})
	router.Static("/assets/", "front/")
	router.Static("/jpeg", "./jpeg") // Новый маршрут для папки jpeg
	router.LoadHTMLGlob("html/templates/*")

	router.GET("/", handlerIndex(app))
	router.GET("/info", handlerInfo)
	router.POST("/info", postHandlerInfo(app, bot))
	router.GET("/highcharts", handlerHighcharts)
	router.GET("/data", handlerHighchartsData(app))
	router.GET("/black", blackList(app))
	router.POST("/black/update", updateBlackList(app))
	router.GET("/jsonred", func(c *gin.Context) {
		c.HTML(http.StatusOK, "jsonred.html", gin.H{
			"categories":   structs.Categories,
			"Classjsonred": "current",
		})
	})
	router.POST("/jsonred/save", saveCategories)
	app.Log.Info("Starting server on port 8080")
	err := router.Run(":8090")
	if err != nil {
		app.Log.Errorf("GinRouter %v", err)
		return
	}
}

//handlerHighchartsData json для Highcharts.
func handlerHighchartsData(app *structs.Application) gin.HandlerFunc {
	return func(c *gin.Context) {
		params := structs.QueryParams{
			//StartDate: time.Now().AddDate(0, 0, -1).Format("2006-01-02"),
			//EndDate:   time.Now().Format("2006-01-02"),
			OrderBy: "tg_user_orders_date.delivery_date",
			//OrderDir: "DESC",
			OrderDir: "ASC",
		}

		orders, err := dataSources.GetOrdersWithFilters(params)
		if err != nil {
			app.Log.Errorf("ошибка запрос к базе %v", err)
			return
		}
		var data [][]int64
		dateSums := make(map[int64]int)
		var DateTime string

		for _, order := range orders {
			t, err := time.Parse(time.RFC3339, order.DeliveryDate)
			if err != nil {
				app.Log.Errorf("Error parsing date: %v", err)
				continue
			}
			timestamp := t.Unix() * 1000
			if order.DateTime != DateTime {
				dateSums[timestamp] += order.TotalCost
				DateTime = order.DateTime
			}
		}

		// Создаем массив с ключами и сортируем их
		var keys []int64
		for k := range dateSums {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

		for _, k := range keys {
			data = append(data, []int64{k, int64(dateSums[k])})
		}

		c.JSON(http.StatusOK, data)
	}
}

//handlerHighcharts график Highcharts.
func handlerHighcharts(c *gin.Context) {
	c.HTML(http.StatusOK, "highcharts.html", gin.H{
		"Title":    "Заказы Салаты корейские",
		"Classreg": "current"})
}

//updateBlackList обновляем черный список.
func updateBlackList(app *structs.Application) gin.HandlerFunc {
	return func(c *gin.Context) {

		userID, err := strconv.Atoi(c.PostForm("user_id"))
		if err != nil {
			c.String(http.StatusBadRequest, fmt.Sprintf("Invalid user ID: %v", err))
			app.Log.Errorf("Invalid user ID: %v", c.PostForm("user_id"))
			return
		}

		userBlackStr := c.PostForm(fmt.Sprintf("user_black_%d", userID))
		userBlack := 0
		if userBlackStr == "1" {
			userBlack = 1
		}

		err = dataSources.UpdateUserBlack(userID, userBlack)
		if err != nil {
			c.String(http.StatusInternalServerError, fmt.Sprintf("Error updating user: %v", err))
			app.Log.Errorf("Error updating user: %v", err)
			return
		}

		users, err := dataSources.GetUsers()
		if err != nil {
			app.Log.Errorf("Error getting users: %v", err)
			return
		}

		c.HTML(http.StatusOK, "user_black.html", gin.H{
			"Title":      "Заказы Салаты корейские",
			"Users":      users,
			"Message":    "Данные успешно изменены",
			"Classblack": "current",
		})
	}
}

//blackList добавляем в черный список.
func blackList(app *structs.Application) gin.HandlerFunc {
	return func(c *gin.Context) {

		users, err := dataSources.GetUsers()
		if err != nil {
			app.Log.Errorf("Error getting users: %v", err)
			c.HTML(http.StatusOK, "user_black.html", gin.H{
				"Title":      "Заказы Салаты корейские",
				"Message":    "Ошибка обработки формы",
				"Classblack": "current",
			})
			return
		}

		c.HTML(http.StatusOK, "user_black.html", gin.H{
			"Users":      users,
			"Title":      "Заказы Салаты корейские",
			"Classblack": "current",
		})
	}
}

// handlerInfo код страницы инфо.
func handlerInfo(c *gin.Context) {

	c.HTML(200, "upload.html", gin.H{
		"Title":    "Заказы Салаты корейские",
		"Classupl": "current"})

}

// postHandlerInfo отправка сообщения, в телеграм инфо.
func postHandlerInfo(app *structs.Application, bot *tgbotapi.BotAPI) gin.HandlerFunc {
	return func(c *gin.Context) {
		var messageInfo structs.MessageInfo

		// Bind form data to struct
		err := c.Bind(&messageInfo)
		if err != nil {

			app.Log.Errorf("PostHandlerInfo Bind: %v", err)
			c.HTML(http.StatusOK, "upload.html", gin.H{
				"Title":    "Заказы Салаты корейские",
				"Message":  "Ошибка обработки формы",
				"Classupl": "current"})
			return

		}

		// Check if Info and InfoText are not empty
		if messageInfo.Info == "" || messageInfo.InfoText == "" {

			c.HTML(http.StatusOK, "upload.html", gin.H{
				"Title":    "Заказы Салаты корейские",
				"Message":  "Поля Тип информации и Сообщение для отправки не должны быть пустыми ",
				"Classupl": "current"})
			return
		}

		// Get uploaded file
		file, err := c.FormFile("Image")
		if err != nil && err != http.ErrMissingFile {

			app.Log.Errorf("PostHandlerInfo FormFile: %v", err)
			c.HTML(http.StatusOK, "upload.html", gin.H{
				"Title":    "Заказы Салаты корейские",
				"Message":  "Ошибка загрузки файла",
				"Classupl": "current"})
			return

		}

		// Prepare message text
		msgText := messageInfo.Info + "\n" + messageInfo.InfoText + "\n"

		// Get user IDs
		users := dataSources.UserID(app)

		if users != nil {

			for _, user := range users {

				// Send text message
				message := tgbotapi.NewMessage(user.UserID, msgText)

				_, err = bot.Send(message)
				if err != nil {

					app.Log.Errorf("PostHandlerInfo SendText: %v", err)
					c.HTML(http.StatusOK, "upload.html", gin.H{
						"Title":    "Заказы Салаты корейские",
						"Message":  "Произошла ошибка при отправке текстового сообщения, попробуйте еще раз",
						"Classupl": "current"})
					return

				}

				// Send image if uploaded
				if file != nil {

					savePath := "./uploads/"
					if _, err = os.Stat(savePath); os.IsNotExist(err) {

						err = os.MkdirAll(savePath, os.ModePerm)
						if err != nil {

							app.Log.Errorf("PostHandlerInfo Mkdir: %v", err)
							c.HTML(http.StatusOK, "upload.html", gin.H{
								"Title":    "Заказы Салаты корейские",
								"Message":  "Произошла ошибка при создании директории для сохранения изображения",
								"Classupl": "current"})
							return

						}
					}

					filePath := filepath.Join(savePath, file.Filename)
					err = c.SaveUploadedFile(file, filePath)
					if err != nil {

						app.Log.Errorf("PostHandlerInfo SaveFile: %v", err)
						c.HTML(http.StatusOK, "upload.html", gin.H{
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

						app.Log.Errorf("PostHandlerInfo SendPhoto: %v", err)
						c.HTML(http.StatusOK, "upload.html", gin.H{
							"Title":    "Заказы Салаты корейские",
							"Message":  "Произошла ошибка при отправке изображения, попробуйте еще раз",
							"Classupl": "current"})
						return

					}
				}
			}

			c.HTML(http.StatusOK, "upload.html", gin.H{
				"Title":    "Заказы Салаты корейские",
				"Message":  "Извещение успешно отправлено",
				"Classupl": "current"})
		} else {

			c.HTML(http.StatusOK, "upload.html", gin.H{
				"Title":    "Заказы Салаты корейские",
				"Message":  "Пока некому отправлять",
				"Classupl": "current"})
		}
	}
}

// handlerIndex Главная страница с выводом всех значений из базы.
func handlerIndex(app *structs.Application) gin.HandlerFunc {
	return func(c *gin.Context) {

		now := time.Now().Format("02.01.2006") // формат даты в виде DD.MM.YYYY

		params := structs.QueryParams{
			//StartDate: time.Now().AddDate(0, 0, -1).Format("2006-01-02"),
			//EndDate:   time.Now().Format("2006-01-02"),
			OrderBy:  "tg_user_orders_date.delivery_date",
			OrderDir: "DESC",
		}

		orders, err := dataSources.GetOrdersWithFilters(params)
		if err != nil {
			app.Log.Errorf("ошибка запрос к базе %v", err)
			return
		}

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
					app.Log.Errorf("Ошибка парсинга строки времени: %v", err)
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

		rty := mergeSlidesWithDeliveryDate(payloads, webnums)

		c.HTML(http.StatusOK, "index.html", gin.H{
			"Title":    "Заказы Салаты корейские",
			"Classind": "current",
			"payload":  rty})
		app.Log.Info("Стартовая страница")
	}
}

// mergeSlidesWithDeliveryDate объединяем два слайда для добавления к основному слайду Num.
func mergeSlidesWithDeliveryDate(orders []structs.OrderInfo, nums []structs.Webnums) []structs.OrderInfo {
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
