package dataSources

import (
	"ProjectTelBotMark/loggers"
	"ProjectTelBotMark/structs"
	"database/sql"
	"fmt"
	"github.com/gookit/slog"
	_ "github.com/mattn/go-sqlite3"
	"regexp"
	"strconv"
	"time"
)

// Настраиваем логгер
var logger *slog.Logger // Глобальный логгер

// DB одк
var DB *sql.DB

// InitDB - инициализация базы данных.
func InitDB() {
	// Настраиваем логгер
	logger = loggers.SetupLogger()
	// Открываем соединение с базой данных SQLite
	var err error
	DB, err = sql.Open("sqlite3", "./data.db")
	if err != nil {
		logger.Fatalf("Error opening database: %v", err)
	}

	_, err = DB.Exec(`
	CREATE TABLE IF NOT EXISTS tg_user_Chat_ID (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER UNIQUE	
	)		
	    `)
	if err != nil {
		logger.Fatalf("Error creating reminders table: %v", err)
	}

	_, err = DB.Exec(`
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
		logger.Fatalf("Error creating reminders table: %v", err)
	}

	_, err = DB.Exec(`
	CREATE TABLE IF NOT EXISTS tg_user_order_id (
    id INTEGER PRIMARY KEY AUTOINCREMENT,    
    item_name TEXT     
	)
	`)
	if err != nil {
		logger.Fatalf("Error creating reminders table: %v", err)
	}

	_, err = DB.Exec(`
	CREATE TABLE IF NOT EXISTS tg_user_phone_number (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	phone_number TEXT UNIQUE
	)
	`)
	if err != nil {
		logger.Fatalf("Error creating reminders table: %v", err)
	}

	_, err = DB.Exec(`
	CREATE TABLE IF NOT EXISTS tg_user_full_name (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
    full_name TEXT UNIQUE
	)
	`)
	if err != nil {
		logger.Fatalf("Error creating reminders table: %v", err)
	}

	_, err = DB.Exec(`
	CREATE TABLE IF NOT EXISTS tg_user_delivery_address (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	delivery_address TEXT UNIQUE
	)
	`)
	if err != nil {
		logger.Fatalf("Error creating reminders table: %v", err)
	}

	_, err = DB.Exec(`
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
		logger.Fatalf("Error creating reminders table: %v", err)
	}

}

// CloseDB - закрытие соединения с базой данных.
func CloseDB() {
	// Настраиваем логгер
	logger = loggers.SetupLogger()
	if DB != nil {
		err := DB.Close()
		if err != nil {
			logger.Errorf("Error closing database: %v", err)
			return
		}
	}
}

func UserID() []structs.User {
	// Настраиваем логгер
	logger = loggers.SetupLogger()
	row, err := DB.Query(`SELECT user_id FROM tg_user_Chat_ID`)
	if err != nil {
		logger.Errorf("Error getting user id: %v", err)
		return nil
	}

	var user []structs.User

	for row.Next() {
		u := structs.User{}
		err = row.Scan(&u.UserID)
		if err != nil {
			logger.Errorf("Error getting user id: %v", err)
			return nil
		}
		user = append(user, u)
	}
	return user
}

// getOrInsertValues Функция для получения или вставки значений в таблицы базы данных.
func getOrInsertValues(tx *sql.Tx, tables []string, columns []string, values []interface{}) ([]int64, error) {
	// Настраиваем логгер
	logger = loggers.SetupLogger()

	var ids []int64

	// Проверяем, вставляем ли данные в одну таблицу
	if len(tables) == 1 {
		// Вставка данных в одну таблицу с несколькими колонками
		table := tables[0]
		var idd int64

		// Создаем динамический запрос для выбора строки
		selectQuery := "SELECT id FROM " + table + " WHERE "
		insertQuery := "INSERT INTO " + table + " ("
		valuesPlaceholders := "VALUES ("

		for i, column := range columns {
			if i > 0 {
				selectQuery += " AND "
				insertQuery += ", "
				valuesPlaceholders += ", "
			}
			selectQuery += fmt.Sprintf("%s = ?", column)
			insertQuery += column
			valuesPlaceholders += "?"
		}
		insertQuery += ") " + valuesPlaceholders + ")"

		// Проверяем, существует ли строка
		err := tx.QueryRow(selectQuery, values...).Scan(&idd)
		if err != nil && err != sql.ErrNoRows {
			logger.Errorf("Ошибка при выборе строки: %v", err)
			return nil, err
		}

		if idd == 0 {
			// Вставляем новую строку, если она не существует
			res, err := tx.Exec(insertQuery, values...)
			if err != nil {
				logger.Errorf("Ошибка при вставке строки: %v", err)
				return nil, err
			}

			idd, err = res.LastInsertId()
			if err != nil {
				logger.Errorf("Ошибка при вставке строки: %v", err)
				return nil, err
			}
		}

		ids = append(ids, idd)
	} else {
		// Вставка данных в несколько таблиц с одной колонкой в каждой
		for i, table := range tables {
			column := columns[i]
			value := values[i]

			var idd int64
			query := fmt.Sprintf("SELECT id FROM %s WHERE %s = ?", table, column)
			err := tx.QueryRow(query, value).Scan(&idd)
			if err != nil && err != sql.ErrNoRows {
				logger.Errorf("Ошибка при выборе строки: %v", err)
				return nil, err
			}

			if idd == 0 {
				insertQuery := fmt.Sprintf("INSERT INTO %s (%s) VALUES (?)", table, column)
				res, err := tx.Exec(insertQuery, value)
				if err != nil {
					logger.Errorf("Ошибка при вставке строки: %v", err)
					return nil, err
				}

				idd, err = res.LastInsertId()
				if err != nil {
					logger.Errorf("Ошибка при вставке строки: %v", err)
					return nil, err
				}
			}

			ids = append(ids, idd)
		}
	}

	return ids, nil
}

// ProcessOrder Функция для парсинга деталей заказа и добавления в базу данных.
func ProcessOrder(order string, db *sql.DB, userID int64) error {
	// Настраиваем логгер
	logger = loggers.SetupLogger()

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
		logger.Errorf("Error starting transaction: %v", err)
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
		logger.Errorf("Error inserting row: %v", err)
		return err
	}

	existingUserID := ids[0]
	existingFullNameID := ids[1]
	existingPhoneID := ids[2]
	existingAddressID := ids[3]

	// Вставляем новую запись в tg_orders_date
	var existingOrdersDateID int64

	tables = []string{"tg_user_orders_date"}
	columns = []string{"user_id", "phone_number_id", "full_name_id", "delivery_address_id", "date_time", "delivery_date", "total_cost"}
	values = []interface{}{existingUserID, existingPhoneID, existingFullNameID, existingAddressID, formattedDateTime, deliveryDate, totalCost}

	ids, err = getOrInsertValues(tx, tables, columns, values)
	if err != nil {
		// Обработать ошибку
		logger.Errorf("Error inserting row: %v", err)
		return err
	}

	existingOrdersDateID = ids[0]

	// Вставляем новые записи в tg_user_orders
	stmt, err := tx.Prepare(`INSERT INTO tg_user_orders (order_id, item_name, quantity, cost) VALUES (?, ?, ?, ?)`)
	if err != nil {
		logger.Errorf("Error inserting row: %v", err)
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
			logger.Errorf("Error inserting row: %v", err)
			return err
		}

		_, err = stmt.Exec(existingOrdersDateID, ids[0], quantity, cost)
		if err != nil {
			logger.Errorf("Error inserting row: %v", err)
			return err
		}
	}

	return nil
}

// processRows функция для обработки строк результата SQL-запроса.
func processRows(rows *sql.Rows) ([]structs.OrderDb, error) {
	// Настраиваем логгер
	logger = loggers.SetupLogger()

	var orders []structs.OrderDb

	for rows.Next() {
		var order structs.OrderDb
		err := rows.Scan(
			&order.UserID.UserID,
			&order.FullName,
			&order.DateTime,
			&order.TotalCost,
			&order.DeliveryDate,
			&order.PhoneNumber,
			&order.DeliveryAddress,
			&order.Purchase.Item,
			&order.Purchase.Quantity,
			&order.Purchase.Cost,
		)
		if err != nil {
			logger.Errorf("Error scanning rows: %v", err)
			return nil, err
		}
		orders = append(orders, order)
	}

	if err := rows.Err(); err != nil {
		logger.Errorf("Error after rows.Next(): %v", err)
		return nil, err
	}

	return orders, nil
}

// Общая часть запроса
const baseQuery = `
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
const limitedOrdersQuery = `
    WITH LimitedOrders AS (
        SELECT
            tg_user_orders_date.id AS order_id
        FROM
            tg_user_orders_date
            JOIN tg_user_Chat_ID ON tg_user_orders_date.user_id = tg_user_Chat_ID.id
        WHERE
            tg_user_Chat_ID.user_id = ?
        ORDER BY
            tg_user_orders_date.delivery_date DESC
        LIMIT ?
    )
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
        tg_user_orders_date.id IN (SELECT order_id FROM LimitedOrders)
    ORDER BY
        tg_user_orders_date.delivery_date DESC;
`

// GetOrdersByUserID Функция для выбора из базы данных.
func GetOrdersByUserID(db *sql.DB, userID int64, limit uint8) ([]structs.OrderDb, error) {
	// Настраиваем логгер
	logger = loggers.SetupLogger()

	// Выполняем запрос для получения заказов
	rows, err := db.Query(limitedOrdersQuery, userID, limit)
	if err != nil {
		logger.Printf("Error preparing query: %v", err)
		return nil, err
	}
	//defer rows.Close()

	return processRows(rows)
}

// GetOrdersWithFilters Функция для выбора данных из базы данных с фильтрацией и сортировкой.
func GetOrdersWithFilters(db *sql.DB, params structs.QueryParams) ([]structs.OrderDb, error) {
	// Настраиваем логгер
	logger = loggers.SetupLogger()

	query := baseQuery

	queryArgs := []interface{}{}
	if params.UserID.UserID != 0 {
		query += " WHERE tg_user_Chat_ID.user_id = ?"
		queryArgs = append(queryArgs, params.UserID)
	}

	if params.StartDate != "" {
		if len(queryArgs) > 0 {
			query += " AND tg_user_orders_date.date_time >= ?"
		} else {
			query += " WHERE tg_user_orders_date.date_time >= ?"
		}
		queryArgs = append(queryArgs, params.StartDate)
	}

	if params.EndDate != "" {
		if len(queryArgs) > 0 {
			query += " AND tg_user_orders_date.date_time <= ?"
		} else {
			query += " WHERE tg_user_orders_date.date_time <= ?"
		}
		queryArgs = append(queryArgs, params.EndDate)
	}

	if params.OrderBy != "" {
		query += " ORDER BY " + params.OrderBy
		if params.OrderDir != "" {
			query += " " + params.OrderDir
		}
	}

	rows, err := db.Query(query, queryArgs...)
	if err != nil {
		logger.Errorf("Error preparing query %v ", err)
		return nil, err
	}
	//defer rows.Close()

	return processRows(rows)
}
