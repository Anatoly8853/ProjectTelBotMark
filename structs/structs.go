package structs

type UserBlack struct {
	UserName  string `json:"user_name,omitempty"`
	UserBlack int    `json:"user_black,omitempty"`
}

type User struct {
	UserID    int64     `json:"user_id,omitempty"`
	UserBlack UserBlack `json:"user_blacks,omitempty"`
}

type MessageInfo struct {
	Info      string `json:"info" form:"Info"`
	InfoText  string `json:"info_text" form:"InfoText"`
	ImagePath string `json:"image_path" form:"ImagePath"`
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

// Category представляет категорию с именем и кнопками.
type Category struct {
	CategoryName string       `json:"categoryName,omitempty"`
	CategoryMsg  string       `json:"categoryMsg,omitempty"`
	Photo        string       `json:"photo"`
	Buttons      []ButtonData `json:"buttons"`
	ButtonsM     []ButtonData `json:"buttonsM,omitempty"`
}

// Categories Распаковка данных.
var Categories map[string]Category

// QueryParams Структура для хранения параметров фильтрации и сортировки.
type QueryParams struct {
	OrderBy   string
	OrderDir  string
	UserID    User
	StartDate string
	EndDate   string
}

// Webnums структура.
type Webnums struct {
	Num int
}

type OrderDb struct {
	UserID          User   `json:"user_id,omitempty"`
	FullName        string `json:"full_name"`
	DateTime        string `json:"date_time"`
	TotalCost       int    `json:"total_cost"`
	DeliveryDate    string `json:"delivery_date"`
	PhoneNumber     string `json:"phone_number"`
	DeliveryAddress string `json:"delivery_address"`
	Purchase        Purchase
}

// OrderInfo основная структура данных для заказа продуктов.
type OrderInfo struct {
	OrderDb     OrderDb
	Num         int  `json:"num,omitempty"`
	CurrentDate bool `json:"current_date,omitempty"`
	Index       int  `json:"index,omitempty"`
}

type Order struct {
	DeliveryDate string `json:"delivery_date,omitempty"`
	TotalCost    int    `json:"total_cost,omitempty"`
}
