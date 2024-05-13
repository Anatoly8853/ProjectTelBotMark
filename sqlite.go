// main пакет.
package main

import (
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"regexp"
	"strconv"
	"time"
)

// db одк
var db *sql.DB

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

// InitDB - инициализация базы данных.
func InitDB() {
	// Открываем соединение с базой данных SQLite
	var err error
	db, err = sql.Open("sqlite3", "./data.db")
	if err != nil {
		logError("Error opening database:", err)
		log.Fatal("Error opening database:", err)
	}

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS tg_user_Chat_ID (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER UNIQUE	
	)		
	    `)
	if err != nil {
		logError("Error creating table:", err)
		log.Fatal("Error creating reminders table:", err)
	}

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS tg_user_orders (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    order_id INTEGER,
    item_name INTEGER,
    quantity INTEGER,
    cost INTEGER,
    FOREIGN KEY (order_id) REFERENCES tg_user_orders_date(id),   
	FOREIGN KEY (item_name) REFERENCES tg_user_order_id(id) 
	)
	`)
	if err != nil {
		logError("Error creating table:", err)
		log.Fatal("Error creating reminders table:", err)
	}

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS tg_user_order_id (
    id INTEGER PRIMARY KEY AUTOINCREMENT,    
    item_name TEXT     
	)
	`)
	if err != nil {
		logError("Error creating table:", err)
		log.Fatal("Error creating reminders table:", err)
	}

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS tg_user_phone_number (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	phone_number TEXT UNIQUE
	)
	`)
	if err != nil {
		logError("Error creating table:", err)
		log.Fatal("Error creating reminders table:", err)
	}

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS tg_user_full_name (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
    full_name TEXT UNIQUE
	)
	`)
	if err != nil {
		logError("Error creating table:", err)
		log.Fatal("Error creating reminders table:", err)
	}

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS tg_user_delivery_address (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	delivery_address TEXT UNIQUE
	)
	`)
	if err != nil {
		logError("Error creating table:", err)
		log.Fatal("Error creating reminders table:", err)
	}

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS tg_user_orders_date (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER,
    phone_number_id INTEGER,
    full_name_id INTEGER,
    delivery_address_id INTEGER,
    date_time TEXT,
    delivery_date TEXT,
    total_cost INTEGER,
    FOREIGN KEY (user_id) REFERENCES tg_user_Chat_ID(id),
    FOREIGN KEY (phone_number_id) REFERENCES tg_user_phone_number(id),
    FOREIGN KEY (full_name_id) REFERENCES tg_user_full_name(id),
    FOREIGN KEY (delivery_address_id) REFERENCES tg_user_delivery_address(id)
	)	
	`)
	if err != nil {
		logError("Error creating table:", err)
		log.Fatal("Error creating reminders table:", err)
	}

}

// CloseDB - закрытие соединения с базой данных.
func CloseDB() {
	if db != nil {
		err := db.Close()
		if err != nil {
			logError("Error closing database:", err)
			return
		}
	}
}

func getOrInsertValues(tx *sql.Tx, tables []string, columns []string, values []interface{}) ([]int64, error) {
	var ids []int64
	for i, table := range tables {
		column := columns[i]
		value := values[i]

		var idd int64
		err := tx.QueryRow(fmt.Sprintf(`SELECT id FROM %s WHERE %s = ?`, table, column), value).Scan(&idd)
		if err != nil && err != sql.ErrNoRows {
			logError("Error selecting row: ", err)
			return nil, err
		}

		if idd == 0 {
			res, err := tx.Exec(fmt.Sprintf("INSERT INTO %s (%s) VALUES (?)", table, column), value)
			if err != nil {
				logError("Error inserting row:", err)
				return nil, err
			}

			idd, err = res.LastInsertId()
			if err != nil {
				logError("Error inserting row:", err)
				return nil, err
			}
		}

		ids = append(ids, idd)
	}

	return ids, nil
}

// ProcessOrder Функция для парсинга деталей заказа и добавления в базу данных.
func ProcessOrder(order string, db *sql.DB, userID int64) error {
	// Регулярные выражения для извлечения информации
	phoneRegex := regexp.MustCompile(`Номер телефона: (\+\d+)`)
	addressRegex := regexp.MustCompile(`Адрес доставки: (.+)`)
	dateRegex := regexp.MustCompile(`Дата доставки: (\d{2}\.\d{2}\.\d{4})`)
	nameRegex := regexp.MustCompile(`ФИО: (.+)`)
	itemRegex := regexp.MustCompile(`Товар: (.+?) кол-во: (\d+) цена: (\d+)\s*руб\.`)
	totalCostRegex := regexp.MustCompile(`Общая стоимость: (\d+)\sруб\.`)

	// Извлечение основной информации о заказе
	phoneNumber := phoneRegex.FindStringSubmatch(order)[1]
	deliveryAddress := addressRegex.FindStringSubmatch(order)[1]
	deliveryDate := dateRegex.FindStringSubmatch(order)[1]
	fullName := nameRegex.FindStringSubmatch(order)[1]
	totalCost, _ := strconv.Atoi(totalCostRegex.FindStringSubmatch(order)[1])
	// Извлечение информации о товарах
	items := itemRegex.FindAllStringSubmatch(order, -1)

	// Начало транзакции
	tx, err := db.Begin()
	if err != nil {
		logError("Error starting transaction: ", err)
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	// Форматирование текущей даты и времени в указанный формат
	formattedDateTime := time.Now().Format("15:04 02.01.2006")

	// Вставка новой записи в tg_user_Chat_ID
	// Проверяем, существует ли пользователь с таким user_id
	tables := []string{"tg_user_Chat_ID", "tg_user_full_name", "tg_user_phone_number", "tg_user_delivery_address"}
	columns := []string{"user_id", "full_name", "phone_number", "delivery_address"}
	values := []interface{}{userID, fullName, phoneNumber, deliveryAddress}

	ids, err := getOrInsertValues(tx, tables, columns, values)
	if err != nil {
		// Обработать ошибку
		logError("Error inserting row: ", err)
		return err
	}

	existingUserID := ids[0]
	existingFullNameID := ids[1]
	existingPhoneID := ids[2]
	existingAddressID := ids[3]

	// Вставляем новую запись в tg_orders_date
	var existingOrdersDateID int64
	stmt, err := tx.Prepare(`INSERT INTO tg_user_orders_date (user_id, phone_number_id, full_name_id, delivery_address_id, date_time, delivery_date, total_cost) VALUES (?, ?, ?, ?, ?, ?,?)`)
	if err != nil {
		logError("Error inserting row: ", err)
		return err
	}

	res, err := stmt.Exec(existingUserID, existingPhoneID, existingFullNameID, existingAddressID, formattedDateTime, deliveryDate, totalCost)
	if err != nil {
		logError("Error inserting row: ", err)
		return err
	}
	// Получение ID вставленной записи
	existingOrdersDateID, err = res.LastInsertId()
	if err != nil {
		logError("Error inserting row: ", err)
		return err
	}

	// Вставляем новые записи в tg_user_orders
	stmt, err = tx.Prepare(`INSERT INTO tg_user_orders (order_id, item_name, quantity, cost) VALUES (?, ?, ?, ?)`)
	if err != nil {
		logError("Error inserting row: ", err)
		return err
	}

	for _, item := range items {
		itemName := item[1]
		quantity, _ := strconv.Atoi(item[2])
		cost, _ := strconv.Atoi(item[3])

		// Вставка новой записи в tg_user_Chat_ID
		// Проверяем, существует ли пользователь с таким user_id
		tables = []string{"tg_user_order_id"}
		columns = []string{"item_name"}
		values = []interface{}{itemName}

		ids, err = getOrInsertValues(tx, tables, columns, values)
		if err != nil {
			// Обработать ошибку
			logError("Error inserting row: ", err)
			return err
		}

		_, err = stmt.Exec(existingOrdersDateID, ids[0], quantity, cost)
		if err != nil {
			logError("Error inserting query: ", err)
			return err
		}
	}
	return nil
}

// GetOrdersByUserID Функция для выбора из базы данных.
func GetOrdersByUserID(db *sql.DB, userID int64, limit uint8) ([]OrderInfo, error) {
	// Выполняем запрос к базе данных
	rows, err := db.Query(`
        SELECT
            tg_user_Chat_ID.user_id,
            tg_user_full_name.full_name,
            tg_user_orders_date.date_time,
            tg_user_orders_date.total_cost,
            tg_user_orders_date.delivery_date,
            tg_user_phone_number.phone_number,
            tg_user_delivery_address.delivery_address,
            tg_user_order_id.item_name,
            tg_user_orders.quantity,
            tg_user_orders.cost
        FROM
            tg_user_Chat_ID
                JOIN tg_user_orders_date ON tg_user_Chat_ID.id = tg_user_orders_date.user_id
                JOIN tg_user_phone_number ON tg_user_orders_date.phone_number_id = tg_user_phone_number.id
                JOIN tg_user_delivery_address ON tg_user_orders_date.delivery_address_id = tg_user_delivery_address.id
                JOIN tg_user_orders ON tg_user_orders_date.id = tg_user_orders.order_id
                JOIN tg_user_full_name ON tg_user_orders_date.full_name_id = tg_user_full_name.id
                JOIN tg_user_order_id ON tg_user_orders.item_name = tg_user_order_id.id
        WHERE
            tg_user_Chat_ID.user_id = ?
        ORDER BY
            tg_user_orders_date.date_time DESC
        LIMIT ?`, userID, limit)
	if err != nil {
		logError("Error preparing query GetOrdersByUserID by user: ", err)
		return nil, err
	}

	// Создаем слайс для хранения результатов
	var orders []OrderInfo

	// Обрабатываем каждую строку результата
	for rows.Next() {
		var order OrderInfo
		err = rows.Scan(
			&order.UserID,
			&order.FullName,
			&order.DateTime,
			&order.TotalCost,
			&order.DeliveryDate,
			&order.PhoneNumber,
			&order.DeliveryAddress,
			&order.ItemName,
			&order.Quantity,
			&order.Cost,
		)
		if err != nil {
			logError("Error getting orders by user: ", err)
			return nil, err
		}
		orders = append(orders, order)
	}

	// Проверяем наличие ошибок после rows.Next()
	if err = rows.Err(); err != nil {
		logError("Проверяем наличие ошибок после rows.Next():", err)
		return nil, err
	}

	return orders, nil
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

// GetOrdersWithFilters Функция для выбора данных из базы данных с фильтрацией и сортировкой.
func GetOrdersWithFilters(db *sql.DB, params QueryParams) ([]OrderInfo, error) {
	// Создаем базовый запрос
	query := `
		SELECT
			tg_user_Chat_ID.user_id,
			tg_user_full_name.full_name,
			tg_user_orders_date.date_time,
			tg_user_orders_date.total_cost,
			tg_user_orders_date.delivery_date,
			tg_user_phone_number.phone_number,
			tg_user_delivery_address.delivery_address,
			tg_user_order_id.item_name,			
			tg_user_orders.quantity,
			tg_user_orders.cost
		FROM
			tg_user_Chat_ID
				JOIN tg_user_orders_date ON tg_user_Chat_ID.id = tg_user_orders_date.user_id
                JOIN tg_user_phone_number ON tg_user_orders_date.phone_number_id = tg_user_phone_number.id
                JOIN tg_user_delivery_address ON tg_user_orders_date.delivery_address_id = tg_user_delivery_address.id
                JOIN tg_user_orders ON tg_user_orders_date.id = tg_user_orders.order_id
                JOIN tg_user_full_name ON tg_user_orders_date.full_name_id = tg_user_full_name.id
                JOIN tg_user_order_id ON tg_user_orders.item_name = tg_user_order_id.id
	`

	// Добавляем фильтры к запросу
	queryArgs := []interface{}{}
	if params.UserID != 0 {
		query += " WHERE tg_user_Chat_ID.user_id = ?"
		queryArgs = append(queryArgs, params.UserID)
	}
	if !params.StartDate.IsZero() {
		if len(queryArgs) > 0 {
			query += " AND tg_user_orders_date.date_time >= ?"
		} else {
			query += " WHERE tg_user_orders_date.date_time >= ?"
		}
		queryArgs = append(queryArgs, params.StartDate)
	}
	if !params.EndDate.IsZero() {
		if len(queryArgs) > 0 {
			query += " AND tg_user_orders_date.date_time <= ?"
		} else {
			query += " WHERE tg_user_orders_date.date_time <= ?"
		}
		queryArgs = append(queryArgs, params.EndDate)
	}

	// Добавляем сортировку к запросу
	if params.OrderBy != "" {
		query += " ORDER BY " + params.OrderBy
		if params.OrderDir != "" {
			query += " " + params.OrderDir
		}
	}

	// Выполняем запрос к базе данных
	rows, err := db.Query(query, queryArgs...)
	if err != nil {
		logError("Error Выполняем запрос к базе данных: ", err)
		return nil, err
	}

	// Создаем слайс для хранения результатов
	var orders []OrderInfo

	// Обрабатываем каждую строку результата
	for rows.Next() {
		var order OrderInfo
		err = rows.Scan(
			&order.UserID,
			&order.FullName,
			&order.DateTime,
			&order.TotalCost,
			&order.DeliveryDate,
			&order.PhoneNumber,
			&order.DeliveryAddress,
			&order.ItemName,
			&order.Quantity,
			&order.Cost,
		)
		if err != nil {
			logError("Проверяем наличие ошибок после rows.Scan(): ", err)
			return nil, err
		}
		orders = append(orders, order)
	}

	// Проверяем наличие ошибок после rows.Next()
	if err = rows.Err(); err != nil {
		logError("Error Проверяем наличие ошибок после rows.Next(): ", err)
		return nil, err
	}

	return orders, nil
}
