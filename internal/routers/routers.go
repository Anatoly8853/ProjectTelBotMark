package routers

import (
	"ProjectTelBotMark/config"
	"ProjectTelBotMark/internal/dataSources"
	"ProjectTelBotMark/internal/dateutil"
	"ProjectTelBotMark/internal/structs"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	slogavp "github.com/Anatoly8853/slog-avp/v2"
	"github.com/gin-gonic/gin"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/goccy/go-json"
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

const UploadsDir = "./internal/routers/html/uploads"

type GroupKey struct {
	UserID       int64
	DeliveryDate string
	DateTime     string
}

type Handler struct {
	repo *dataSources.Repository
	app  *slogavp.Application
	bot  *tgbotapi.BotAPI
	cfg  config.Config
	//tmpl *template.Template
}

func NewGinRouter(repo *dataSources.Repository, app *slogavp.Application, bot *tgbotapi.BotAPI, cfg config.Config) *Handler {
	return &Handler{repo: repo, app: app, bot: bot, cfg: cfg}
}

// GinRouter Gin Router функция запускает сервер.
func (h *Handler) GinRouter() {

	gin.SetMode(gin.ReleaseMode)
	if strings.TrimSpace(h.cfg.AdminUsername) == "" || len(h.cfg.AdminPassword) < 12 {
		h.app.Log.Error("GinRouter: веб-интерфейс не запущен: задайте ADMIN_USERNAME и ADMIN_PASSWORD (не менее 12 символов) в app.env")
		return
	}
	router := gin.Default()
	router.MaxMultipartMemory = 8 << 20
	router.Use(gin.BasicAuth(gin.Accounts{h.cfg.AdminUsername: h.cfg.AdminPassword}))
	router.Use(uploadBodyLimit(12 << 20))
	router.Use(csrfProtection())
	funcMap := template.FuncMap{
		"toJSON": func(v interface{}) string {
			a, _ := json.Marshal(v)
			return string(a)
		},
	}
	router.SetFuncMap(funcMap)
	router.Static("assets", "./internal/routers/html/front/")
	router.Static("jpeg", "./jpeg")      // Новый маршрут для папки jpeg
	router.Static("uploads", UploadsDir) // Новый маршрут для папки jpeg
	router.LoadHTMLGlob("./internal/routers/html/templates/*")
	//h.tmpl = template.Must(template.ParseFS(templatesFS, "internal/routers/html/templates/*.html"))

	router.GET("/", h.handlerIndex)
	router.GET("/info", handlerInfo)
	router.POST("/info", h.postHandlerInfo)
	router.GET("/highcharts", handlerHighcharts)
	router.GET("/data", h.handlerHighchartsData)
	router.GET("/black", h.blackList)
	router.POST("/black/update", h.updateBlackList)
	router.GET("/jsonred", func(c *gin.Context) {
		c.HTML(http.StatusOK, "jsonred.html", gin.H{
			"categories":   structs.Categories,
			"Classjsonred": "current",
		})
	})
	//router.POST("/jsonred/save", saveCategories)

	router.GET("/product", h.handlerProducts) // главная страница со списком
	router.GET("/products", h.handlerProductsPage)
	router.GET("/product/add", h.showAddForm)           // форма добавления
	router.POST("/product/add", h.addProduct)           // обработка добавления
	router.GET("/product/edit/:id", h.editProduct)      // форма редактирования
	router.POST("/product/update/:id", h.updateProduct) // обработка редактирования
	router.POST("/product/delete/:id", h.deleteProduct) // удаление
	router.GET("/product/export", h.exportExcel)

	addr := strings.TrimSpace(h.cfg.WebAddr)
	if addr == "" {
		addr = "127.0.0.1:8090"
	}
	if (h.cfg.TLSCertFile == "") != (h.cfg.TLSKeyFile == "") {
		h.app.Log.Error("GinRouter: TLS_CERT_FILE и TLS_KEY_FILE должны быть заданы вместе")
		return
	}
	if h.cfg.TLSCertFile != "" {
		h.app.Log.Infof("GinRouter: веб-интерфейс запущен с HTTPS на %s", addr)
		if err := router.RunTLS(addr, h.cfg.TLSCertFile, h.cfg.TLSKeyFile); err != nil {
			h.app.Log.Errorf("GinRouter: ошибка запуска HTTPS-сервера: %v", err)
		}
		return
	}
	host, _, splitErr := net.SplitHostPort(addr)
	if splitErr != nil || (host != "localhost" && net.ParseIP(host) == nil) || (net.ParseIP(host) != nil && !net.ParseIP(host).IsLoopback()) {
		h.app.Log.Infof("GinRouter: сервер работает без TLS; не публикуйте порт вне доверенной локальной сети или VPN")
	}
	h.app.Log.Infof("GinRouter: веб-интерфейс запущен на %s", addr)
	if err := router.Run(addr); err != nil {
		h.app.Log.Errorf("GinRouter: ошибка запуска сервера: %v", err)
	}
}

// handlerHighchartsData json для Highcharts.
func (h *Handler) handlerHighchartsData(c *gin.Context) {

	params := structs.QueryParams{
		//StartDate: time.Now().AddDate(0, 0, -1).Format("2006-01-02"),
		//EndDate:   time.Now().Format("2006-01-02"),
		OrderBy: "tg_user_orders_date.delivery_date",
		//OrderDir: "DESC",
		OrderDir: "ASC",
	}

	orders, err := h.repo.GetOrdersWithFilters(params)
	if err != nil {
		h.app.Log.Errorf("handlerHighchartsData: ошибка запроса заказов: %v", err)
		c.String(http.StatusInternalServerError, "Не удалось загрузить данные графика")
		return
	}
	var data [][]int64
	dateSums := make(map[int64]int)
	var currentOrderID int64

	for _, order := range orders {
		t, err := dateutil.Parse(order.DeliveryDate)
		if err != nil {
			h.app.Log.Errorf("handlerHighchartsData: ошибка разбора даты доставки %q: %v", order.DeliveryDate, err)
			continue
		}
		timestamp := t.Unix() * 1000
		if order.OrderID != currentOrderID {
			dateSums[timestamp] += order.TotalCost
			currentOrderID = order.OrderID
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

// handlerHighcharts график Highcharts.
func handlerHighcharts(c *gin.Context) {
	c.HTML(http.StatusOK, "highcharts.html", gin.H{
		"Title":    "Заказы Салаты корейские",
		"Classreg": "current"})
}

// updateBlackList обновляем черный список.
func (h *Handler) updateBlackList(c *gin.Context) {

	userID, err := strconv.Atoi(c.PostForm("user_id"))
	if err != nil || userID <= 0 {
		c.String(http.StatusBadRequest, "Некорректный ID пользователя")
		h.app.Log.Errorf("updateBlackList: некорректный ID пользователя: %q", c.PostForm("user_id"))
		return
	}

	userBlackStr := c.PostForm(fmt.Sprintf("user_black_%d", userID))
	userBlack := 0
	if userBlackStr == "1" {
		userBlack = 1
	}

	err = h.repo.UpdateUserBlack(userID, userBlack)
	if err != nil {
		c.String(http.StatusInternalServerError, "Не удалось обновить чёрный список")
		h.app.Log.Errorf("updateBlackList: ошибка обновления пользователя %d: %v", userID, err)
		return
	}

	users, err := h.repo.GetUsers()
	if err != nil {
		h.app.Log.Errorf("updateBlackList: ошибка получения списка пользователей: %v", err)
		c.String(http.StatusInternalServerError, "Не удалось обновить список пользователей")
		return
	}

	c.HTML(http.StatusOK, "user_black.html", gin.H{
		"Title":      "Заказы Салаты корейские",
		"Users":      users,
		"Message":    "Данные успешно изменены",
		"Classblack": "current",
	})
}

// blackList добавляем в черный список.
func (h *Handler) blackList(c *gin.Context) {

	users, err := h.repo.GetUsers()
	if err != nil {
		h.app.Log.Errorf("blackList: ошибка получения списка пользователей: %v", err)
		c.String(http.StatusInternalServerError, "Не удалось загрузить чёрный список")
		return
	}

	c.HTML(http.StatusOK, "user_black.html", gin.H{
		"Users":      users,
		"Title":      "Заказы Салаты корейские",
		"Classblack": "current",
	})
}

// handlerInfo код страницы инфо.
func handlerInfo(c *gin.Context) {

	c.HTML(http.StatusOK, "upload.html", gin.H{
		"Title":    "Заказы Салаты корейские",
		"Classupl": "current"})

}

// postHandlerInfo отправка сообщения, в телеграм инфо.
func (h *Handler) postHandlerInfo(c *gin.Context) {
	var messageInfo structs.MessageInfo
	if err := c.ShouldBind(&messageInfo); err != nil {
		h.app.Log.Errorf("postHandlerInfo: ошибка чтения формы: %v", err)
		c.String(http.StatusBadRequest, "Не удалось прочитать форму")
		return
	}
	messageInfo.Info = strings.TrimSpace(messageInfo.Info)
	messageInfo.InfoText = strings.TrimSpace(messageInfo.InfoText)
	if messageInfo.Info == "" || messageInfo.InfoText == "" {
		c.String(http.StatusBadRequest, "Заполните тип информации и текст сообщения")
		return
	}

	imagePath, err := h.saveUploadedImage(c, "Image")
	if err != nil {
		h.app.Log.Errorf("postHandlerInfo: ошибка загрузки изображения: %v", err)
		c.String(http.StatusBadRequest, "Не удалось загрузить изображение: %v", err)
		return
	}
	if imagePath != "" {
		defer func() {
			if err := os.Remove(filepath.Join(UploadsDir, filepath.Base(imagePath))); err != nil && !os.IsNotExist(err) {
				h.app.Log.Errorf("postHandlerInfo: не удалось удалить временное изображение %q: %v", imagePath, err)
			}
		}()
	}

	users := h.repo.UserID()
	if len(users) == 0 {
		c.String(http.StatusOK, "Пользователей для рассылки нет")
		return
	}

	msgText := messageInfo.Info + "\n" + messageInfo.InfoText
	sentCount := 0
	failedCount := 0
	for _, user := range users {
		if _, err := h.bot.Send(tgbotapi.NewMessage(user.UserID, msgText)); err != nil {
			failedCount++
			h.app.Log.Errorf("postHandlerInfo: не удалось отправить текст пользователю %d: %v", user.UserID, err)
			continue
		}
		if imagePath != "" {
			photo := tgbotapi.NewInputMediaPhoto(tgbotapi.FilePath(filepath.Join(UploadsDir, filepath.Base(imagePath))))
			if _, err := h.bot.SendMediaGroup(tgbotapi.NewMediaGroup(user.UserID, []interface{}{photo})); err != nil {
				failedCount++
				h.app.Log.Errorf("postHandlerInfo: не удалось отправить изображение пользователю %d: %v", user.UserID, err)
				continue
			}
		}
		sentCount++
	}
	h.app.Log.Infof("postHandlerInfo: рассылка завершена: успешно=%d, ошибок=%d", sentCount, failedCount)
	if failedCount > 0 {
		c.String(http.StatusOK, "Рассылка завершена частично. Успешно: %d, с ошибками: %d. Подробности записаны в журнал.", sentCount, failedCount)
		return
	}
	c.String(http.StatusOK, "Извещение успешно отправлено всем пользователям")
}

// handlerIndex Главная страница с выводом всех значений из базы.
func (h *Handler) handlerIndex(c *gin.Context) {

	now := time.Now().Format("02.01.2006") // формат даты в виде DD.MM.YYYY

	params := getQueryParams()

	orders, err := h.repo.GetOrdersWithFilters(params)
	if err != nil {
		h.app.Log.Errorf("handlerIndex: ошибка запроса заказов: %v", err)
		c.String(http.StatusInternalServerError, "Не удалось загрузить заказы")
		return
	}

	var payloads []structs.OrderInfo
	var webnums []structs.Webnums

	if len(orders) > 0 {
		var currentOrderID int64
		var countOrderID int64
		var num int
		var Index int

		for i, order := range orders {
			var payload structs.OrderInfo
			var webnum structs.Webnums

			// Парсинг строки времени в тип time.Time
			formattedDeliveryDate := formatDate(order.DeliveryDate)

			if formattedDeliveryDate == now {
				payload.CurrentDate = true
			}

			if order.OrderID != countOrderID {
				countOrderID = order.OrderID
				num = 1
			} else {
				num++
			}

			// Группировка по дате доставки
			if order.OrderID != currentOrderID {
				payload.OrderDb.DateTime = order.DateTime
				payload.OrderDb.DeliveryDate = formattedDeliveryDate
				payload.OrderDb.FullName = order.FullName
				payload.OrderDb.PhoneNumber = order.PhoneNumber
				payload.OrderDb.DeliveryAddress = order.DeliveryAddress
				payload.OrderDb.Comment = strings.TrimSpace(order.Comment)
				if payload.OrderDb.Comment != "" {
					h.app.Log.Debugf("handlerIndex: комментарий загружен для веб-интерфейса: orderID=%d, длина=%d", order.OrderID, len([]rune(payload.OrderDb.Comment)))
				}
				currentOrderID = order.OrderID
				payload.OrderDb.TotalCost = order.TotalCost
				Index++
				payload.Index = Index
			}

			payload.OrderDb.Purchase.Item = order.Purchase.Item
			payload.OrderDb.Purchase.Quantity = order.Purchase.Quantity
			payload.OrderDb.Purchase.Cost = order.Purchase.Cost

			if i == len(orders)-1 || (i+1 < len(orders) && orders[i+1].OrderID != order.OrderID) {
				webnum.Num = num
				webnums = append(webnums, webnum)
			}

			payloads = append(payloads, payload)
		}
	}

	renderHTML(c, payloads, webnums)
}

// mergeSlidesWithDeliveryDate объединяем два слайда для добавления к основному слайду Num.
func mergeSlidesWithDeliveryDate(orders []structs.OrderInfo, nums []structs.Webnums) []structs.OrderInfo {
	var result []structs.OrderInfo
	numIndex := 0

	for _, order := range orders {
		if order.OrderDb.DeliveryDate != "" && numIndex < len(nums) {
			order.Num = nums[numIndex].Num
			numIndex++
		}
		result = append(result, order)
	}

	return result
}

func getQueryParams() structs.QueryParams {
	return structs.QueryParams{
		OrderBy:  "tg_user_orders_date.delivery_date",
		OrderDir: "DESC",
	}
}

func renderHTML(c *gin.Context, payloads []structs.OrderInfo, webnums []structs.Webnums) {
	rty := mergeSlidesWithDeliveryDate(payloads, webnums)
	c.HTML(http.StatusOK, "index.html", gin.H{
		"Title":    "Заказы Салаты корейские",
		"Classind": "current",
		"payload":  rty})
}

// formatDate красиво форматирует дату
func formatDate(dateStr string) string {
	formatted, err := dateutil.Format(dateStr)
	if err != nil {
		return dateStr
	}
	return formatted
}
