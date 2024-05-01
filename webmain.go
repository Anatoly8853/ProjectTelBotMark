package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
)

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

// Главная страница с выводои всех значений из базы
func handlerIndex(c *gin.Context) {

	params := QueryParams{
		//OrderBy: "tg_user_orders_date.date_time",
		OrderBy:  "tg_user_orders_date.delivery_date",
		OrderDir: "DESC",
		//OrderDir: "ASC",
	}

	orders, err := GetOrdersWithFilters(db, params)
	if err != nil {
		fmt.Println("ошибка в функции handlerIndex запрос к базе GetOrdersWithFilters", err)
		return
	}

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
				//payload.TotalCost = order.TotalCost
			}

			payloads = append(payloads, payload)
		}
	}

	rty := mergeSlidesWithDeliveryDate(payloads, webnums)

	c.HTML(200, "index.html", gin.H{
		"Title":    "Телефонный справочник",
		"Classind": "current",
		"payload":  rty})
}

func mergeSlidesWithDeliveryDate(orders []OrderInfo, nums []Webnums) []OrderInfo {
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
