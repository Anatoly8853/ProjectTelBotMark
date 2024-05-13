package main

import "time"

// ButtonData Кнопки.
type ButtonData struct {
	Text     string `json:"text"`
	Callback string `json:"callback"`
}

// Categories Распаковка данных.
var Categories map[string]struct {
	CategoryName string       `json:"categoryName"`
	Photo        string       `json:"photo"`
	Buttons      []ButtonData `json:"buttons"`
	ButtonsM     []ButtonData `json:"buttonsM"`
}

// Purchase товары.
type Purchase struct {
	Item     string
	Quantity int
	Cost     int // Стоимость одного товара
	//Timestamp time.Time
}

// OrderInfo оснавная структура данных для заказа продуктов.
type OrderInfo struct {
	UserID          int64  `json:"user_id"`
	FullName        string `json:"full_name"`
	DateTime        string `json:"date_time"`
	TotalCost       int    `json:"total_cost"`
	DeliveryDate    string `json:"delivery_date"`
	PhoneNumber     string `json:"phone_number"`
	DeliveryAddress string `json:"delivery_address"`
	ItemName        string `json:"item_name"`
	Quantity        int    `json:"quantity"`
	Cost            int    `json:"cost"`
	Num             int    `json:"num"`
	CurrentDate     bool   `json:"current_date"`
}

// QueryParams Структура для хранения параметров фильтрации и сортировки.
type QueryParams struct {
	OrderBy   string
	OrderDir  string
	UserID    int64
	StartDate time.Time
	EndDate   time.Time
	// Добавьте другие необходимые фильтры
}

// Webnums структура.
type Webnums struct {
	Num int
}
