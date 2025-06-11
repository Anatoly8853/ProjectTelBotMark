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
	Info      string `json:"info,omitempty" form:"Info"`
	InfoText  string `json:"info_text,omitempty" form:"InfoText"`
	ImagePath string `json:"image_path,omitempty" form:"ImagePath"`
}

// Purchase товары.
type Purchase struct {
	Item     string `json:"item"`
	Quantity int    `json:"quantity"`
	Cost     int    `json:"cost"` // Стоимость одного товара
}

// ButtonData Кнопки.
type ButtonData struct {
	Text     string `json:"text,omitempty"`
	Callback string `json:"callback,omitempty"`
	Photo    string `json:"photo,omitempty"`
}

// Category представляет категорию с именем и кнопками.
type Category struct {
	ID           int          `json:"id,omitempty"`
	CategoryName string       `json:"categoryName,omitempty"`
	CategoryMsg  string       `json:"categoryMsg,omitempty"`
	Photo        string       `json:"photo,omitempty"`
	Buttons      []ButtonData `json:"buttons,omitempty"`
	ButtonsM     []ButtonData `json:"buttonsM,omitempty"`
}

// Categories Распаковка данных.
var Categories = make(map[string]Category)

// Baskets карта для хранения данных о покупках для каждого пользователя.
var Baskets = make(map[int64][]Purchase)

// QueryParams Структура для хранения параметров фильтрации и сортировки.
type QueryParams struct {
	OrderBy   string `json:"order_by,omitempty"`
	OrderDir  string `json:"order_dir,omitempty"`
	UserID    int64  `json:"user_id,omitempty"`
	StartDate string `json:"start_date,omitempty"`
	EndDate   string `json:"end_date,omitempty"`
}

// Webnums структура.
type Webnums struct {
	Num int `json:"num,omitempty"`
}

type OrderDb struct {
	UserID          User   `json:"user_id,omitempty"`
	FullName        string `json:"full_name,omitempty"`
	DateTime        string `json:"date_time,omitempty"`
	TotalCost       int    `json:"total_cost,omitempty"`
	DeliveryDate    string `json:"delivery_date,omitempty"`
	PhoneNumber     string `json:"phone_number,omitempty"`
	DeliveryAddress string `json:"delivery_address,omitempty"`
	Purchase        Purchase
}

// OrderInfo основная структура данных для заказа продуктов.
type OrderInfo struct {
	OrderDb     OrderDb `json:"order_db"`
	Num         int     `json:"num,omitempty"`
	CurrentDate bool    `json:"current_date,omitempty"`
	Index       int     `json:"index,omitempty"`
}

type Order struct {
	DeliveryDate string `json:"delivery_date,omitempty"`
	TotalCost    int    `json:"total_cost,omitempty"`
}

type Product struct {
	ID          int
	Title       string
	Description string
	Price       int
	Category    string
	Image       string
	Packaging   string
	Text        string `json:"texy ,omitempty"`
	Callback    string `json:"callback ,omitempty"`
}

type PageData struct {
	Products   []Product
	Categories []string
}

type PageProduct struct {
	Search     string
	Category   string
	Sort       string
	Page       int
	Limit      int
	Total      int
	TotalPages int
}
