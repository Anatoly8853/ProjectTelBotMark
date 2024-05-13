// main пакет
package main

import (
	"github.com/gin-gonic/gin"
	"log"
	"sort"
	"time"
)

// Webnums структура.
type Webnums struct {
	Num int
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

// HandlerIndex Главная страница с выводои всех значений из базы.
func HandlerIndex(c *gin.Context) {

	now := time.Now().Format("02.01.2006") // формат даты в виде DD.MM.YYYY

	params := QueryParams{
		//OrderBy: "tg_user_orders_date.date_time",
		OrderBy: "tg_user_orders_date.delivery_date",
		//OrderDir: "DESC",
		OrderDir: "ASC",
	}

	orders, err := GetOrdersWithFilters(db, params)
	if err != nil {
		log.Println("ошибка в функции handlerIndex запрос к базе GetOrdersWithFilters", err)
		return
	}

	sort.Sort(ByDeliveryDate(orders))

	var payloads []OrderInfo
	var webnums []Webnums

	if len(orders) > 0 {
		var currentDeliveryDate string // Объявляем переменную для отслеживания текущей даты доставки
		var currentUserID int64
		var currentTotalCost int
		var currentDateTime string
		var currentDateTime2 string
		var num int

		for i, order := range orders {
			var payload OrderInfo
			var webnum Webnums

			// Парсинг строки времени в тип time.Time
			t, err := time.Parse("02.01.2006", order.DeliveryDate)
			if err != nil {
				log.Println("Ошибка парсинга строки времени:", err)
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
			if order.UserID != currentUserID || order.DeliveryDate != currentDeliveryDate || order.DateTime != currentDateTime {
				payload.DateTime = order.DateTime
				payload.DeliveryDate = order.DeliveryDate
				payload.FullName = order.FullName
				payload.PhoneNumber = order.PhoneNumber
				payload.DeliveryAddress = order.DeliveryAddress
				currentDeliveryDate = order.DeliveryDate
				currentUserID = order.UserID
				currentDateTime = order.DateTime

				if order.TotalCost != currentTotalCost {
					currentTotalCost = order.TotalCost
					payload.TotalCost = order.TotalCost

				}

			}

			payload.ItemName = order.ItemName
			payload.Quantity = order.Quantity
			payload.Cost = order.Cost

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
		"Title":    "Телефонный справочник",
		"Classind": "current",
		"payload":  rty})
}

// MergeSlidesWithDeliveryDate объединяем два слайда для добавление к основному слайду Num.
func MergeSlidesWithDeliveryDate(orders []OrderInfo, nums []Webnums) []OrderInfo {
	var result []OrderInfo
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

type ByDeliveryDate []OrderInfo

func (a ByDeliveryDate) Len() int      { return len(a) }
func (a ByDeliveryDate) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByDeliveryDate) Less(i, j int) bool {
	layout := "02.01.2006"
	dateTime1, _ := time.Parse(layout, a[i].DeliveryDate)
	dateTime2, _ := time.Parse(layout, a[j].DeliveryDate)
	return dateTime1.After(dateTime2)
}
