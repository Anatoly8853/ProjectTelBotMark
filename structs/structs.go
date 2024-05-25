package structs

import "time"

type User struct {
	UserID int64
}

// Purchase товары.
type Purchase struct {
	Item     string `json:"item"`
	Quantity int    `json:"quantity"`
	Cost     int    `json:"cost"` // Стоимость одного товара
}

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

// QueryParams Структура для хранения параметров фильтрации и сортировки.
type QueryParams struct {
	OrderBy   string
	OrderDir  string
	UserID    User
	StartDate time.Time
	EndDate   time.Time
}

// Webnums структура.
type Webnums struct {
	Num int
}

// OrderInfo основная структура данных для заказа продуктов.
type OrderInfo struct {
	UserID          User   `json:"user_id"`
	FullName        string `json:"full_name"`
	DateTime        string `json:"date_time"`
	TotalCost       int    `json:"total_cost"`
	DeliveryDate    string `json:"delivery_date"`
	PhoneNumber     string `json:"phone_number"`
	DeliveryAddress string `json:"delivery_address"`
	Purchase        Purchase
	Num             int  `json:"num"`
	CurrentDate     bool `json:"current_date"`
}
