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

var db *sql.DB

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
}

// InitDB - инициализация базы данных
func InitDB() {
	// Открываем соединение с базой данных SQLite
	var err error
	db, err = sql.Open("sqlite3", "./data.db")
	if err != nil {
		log.Fatal("Error opening database:", err)
	}

	// Создаем таблицу для хранения пользователей
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			chat_id INTEGER,
			username TEXT,
			command TEXT,
			timestamp DATETIME
		)
	`)
	if err != nil {
		log.Fatal("Error creating users table:", err)
	}

	// Создаем таблицу для хранения напоминаний
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS reminders (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			date_time DATETIME,
			message TEXT,
			user_id INTEGER
		)
	`)
	if err != nil {
		log.Fatal("Error creating reminders table:", err)
	}

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS orders (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		date_time DATETIME,
		user_id INTEGER,
		phone_number TEXT, 
		delivery_address TEXT, 
		delivery_date TEXT, 
		full_name TEXT,
		item_name TEXT, 
		quantity INTEGER, 
		cost INTEGER,
		total_cost INTEGER 
	)
	    `)
	if err != nil {
		log.Fatal("Error creating reminders table:", err)
	}

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS tg_user_Chat_ID (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER UNIQUE	
	)		
	    `)
	if err != nil {
		log.Fatal("Error creating reminders table:", err)
	}

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS tg_user_orders (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    order_id INTEGER,
    item_name TEXT,
    quantity INTEGER,
    cost INTEGER,
    FOREIGN KEY (order_id) REFERENCES tg_user_orders_date(id)   
	)
	`)
	if err != nil {
		log.Fatal("Error creating reminders table:", err)
	}

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS tg_user_phone_number (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	phone_number TEXT UNIQUE
	)
	`)
	if err != nil {
		log.Fatal("Error creating reminders table:", err)
	}

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS tg_user_full_name (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
    full_name TEXT UNIQUE
	)
	`)
	if err != nil {
		log.Fatal("Error creating reminders table:", err)
	}

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS tg_user_delivery_address (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	delivery_address TEXT UNIQUE
	)
	`)
	if err != nil {
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
		log.Fatal("Error creating reminders table:", err)
	}

}

// CloseDB - закрытие соединения с базой данных
func CloseDB() {
	if db != nil {
		err := db.Close()
		if err != nil {
			log.Println("Error closing database:", err)
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
			return nil, fmt.Errorf("error querying rows getOrInsertValues %s:", err)
		}
		fmt.Println("getOrInsertValues idd -------------------------------------------", idd)
		if idd == 0 {
			res, err := tx.Exec(fmt.Sprintf("INSERT INTO %s (%s) VALUES (?)", table, column), value)
			if err != nil {
				return nil, fmt.Errorf("error Exec getOrInsertValues %s:", err)
			}

			idd, err = res.LastInsertId()
			if err != nil {
				return nil, fmt.Errorf("error LastInsertId getOrInsertValues %s:", err)
			}
		}

		ids = append(ids, idd)
	}

	return ids, nil
}

// Функция для парсинга деталей заказа и добавления в базу данных
func processOrder(order string, db *sql.DB, userID int64) error {
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
	//fmt.Println("items-------------------------------------------------------------------", items)

	// Начало транзакции
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("Error starting transaction processOrder %s:", err)
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
		return fmt.Errorf("Error preparing query processOrder %s:", err)
	}
	//defer stmt.Close()

	res, err := stmt.Exec(existingUserID, existingPhoneID, existingFullNameID, existingAddressID, formattedDateTime, deliveryDate, totalCost)
	if err != nil {
		return fmt.Errorf("Error executing query %s:", err)
	}
	// Получение ID вставленной записи
	existingOrdersDateID, err = res.LastInsertId()
	if err != nil {
		return fmt.Errorf("Error executing query %s:", err)
	}

	// Вставляем новые записи в tg_user_orders
	stmt, err = tx.Prepare(`INSERT INTO tg_user_orders (order_id, item_name, quantity, cost) VALUES (?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("Error preparing query %s:", err)
	}
	//defer stmt.Close()

	for _, item := range items {
		itemName := item[1]
		quantity, _ := strconv.Atoi(item[2])
		cost, _ := strconv.Atoi(item[3])

		_, err = stmt.Exec(existingOrdersDateID, itemName, quantity, cost)
		if err != nil {
			return fmt.Errorf("Error executing query %s:", err)
		}
	}

	return nil
}

// Функция для выбора из базы данных
func GetOrdersByUserID(db *sql.DB, userID int64) ([]OrderInfo, error) {
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
    tg_user_orders.item_name,
    tg_user_orders.quantity,
    tg_user_orders.cost
FROM
    tg_user_Chat_ID
        JOIN
    tg_user_orders_date ON tg_user_Chat_ID.id = tg_user_orders_date.user_id
        JOIN
    tg_user_phone_number ON tg_user_orders_date.phone_number_id = tg_user_phone_number.id
        JOIN
    tg_user_delivery_address ON tg_user_orders_date.delivery_address_id = tg_user_delivery_address.id
        JOIN
    tg_user_orders ON tg_user_orders_date.id = tg_user_orders.order_id
        JOIN
    tg_user_full_name ON tg_user_orders_date.full_name_id = tg_user_full_name.id
WHERE
    tg_user_Chat_ID.user_id  = ?`, userID)
	if err != nil {
		return nil, fmt.Errorf("Error preparing query GetOrdersByUserID %s:", err)
	}
	//defer rows.Close()

	// Создаем слайс для хранения результатов
	var orders []OrderInfo

	// Обрабатываем каждую строку результата
	// Обрабатываем каждую строку результата
	for rows.Next() {
		var order OrderInfo
		err := rows.Scan(
			&order.UserID,
			&order.FullName,
			&order.DateTime, // Сканируем как string
			&order.TotalCost,
			&order.DeliveryDate,
			&order.PhoneNumber,
			&order.DeliveryAddress,
			&order.ItemName,
			&order.Quantity,
			&order.Cost,
		)
		if err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}

	// Проверяем наличие ошибок после rows.Next()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return orders, nil
}

// Структура для хранения параметров фильтрации и сортировки
type QueryParams struct {
	OrderBy   string
	OrderDir  string
	UserID    int64
	StartDate time.Time
	EndDate   time.Time
	// Добавьте другие необходимые фильтры
}

// Функция для выбора данных из базы данных с фильтрацией и сортировкой
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
			tg_user_orders.item_name,
			tg_user_orders.quantity,
			tg_user_orders.cost
		FROM
			tg_user_Chat_ID
				JOIN tg_user_orders_date ON tg_user_Chat_ID.id = tg_user_orders_date.user_id
				JOIN tg_user_phone_number ON tg_user_orders_date.phone_number_id = tg_user_phone_number.id
				JOIN tg_user_delivery_address ON tg_user_orders_date.delivery_address_id = tg_user_delivery_address.id
				JOIN tg_user_orders ON tg_user_orders_date.id = tg_user_orders.order_id
				JOIN tg_user_full_name ON tg_user_orders_date.full_name_id = tg_user_full_name.id
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
		return nil, err
	}
	//defer rows.Close()

	// Создаем слайс для хранения результатов
	var orders []OrderInfo

	// Обрабатываем каждую строку результата
	for rows.Next() {
		var order OrderInfo
		err := rows.Scan(
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
			return nil, err
		}
		orders = append(orders, order)
	}

	// Проверяем наличие ошибок после rows.Next()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return orders, nil
}
