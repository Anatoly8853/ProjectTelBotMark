package dataSources

import (
	`ProjectTelBotMark/bychat`
	"ProjectTelBotMark/loggers"
	"ProjectTelBotMark/structs"
	"database/sql"
	"fmt"
	"io"
	"os"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	_ "github.com/mattn/go-sqlite3"
)

// DB одк
var DB *sql.DB

var UserBlack = make(map[int64]int)

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

const baseQueryOrders = `
  SELECT
    tg_user_Chat_ID.user_id,
    tg_user_full_name.full_name,
    tg_user_phone_number.phone_number,
    tg_user_delivery_address.delivery_address
FROM
    tg_user_Chat_ID
        JOIN tg_user_orders_date ON tg_user_Chat_ID.id = tg_user_orders_date.user_id
        JOIN tg_user_phone_number ON tg_user_orders_date.phone_number_id = tg_user_phone_number.id
        JOIN tg_user_delivery_address ON tg_user_orders_date.delivery_address_id = tg_user_delivery_address.id
        JOIN tg_user_full_name ON tg_user_orders_date.full_name_id = tg_user_full_name.id
WHERE tg_user_Chat_ID.user_id = ?
ORDER BY
    tg_user_orders_date.delivery_date DESC
LIMIT 1
`

// InitDB - инициализация базы данных.
func InitDB(app *structs.Application) {

	// Открываем соединение с базой данных SQLite
	var err error
	DB, err = sql.Open("sqlite3", "./data.db")
	if err != nil {
		app.Log.Fatalf("Error opening database: %v", err)
	}

	_, err = DB.Exec(`
	CREATE TABLE IF NOT EXISTS tg_user_Chat_ID (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER UNIQUE,
    user_name TEXT,
    user_black INTEGER    	
	)		
	    `)
	if err != nil {
		app.Log.Fatalf("Error creating reminders table: %v", err)
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
		app.Log.Fatalf("Error creating reminders table: %v", err)
	}

	_, err = DB.Exec(`
	CREATE TABLE IF NOT EXISTS tg_user_order_id (
    id INTEGER PRIMARY KEY AUTOINCREMENT,    
    item_name TEXT     
	)
	`)
	if err != nil {
		app.Log.Fatalf("Error creating reminders table: %v", err)
	}

	_, err = DB.Exec(`
	CREATE TABLE IF NOT EXISTS tg_user_phone_number (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	phone_number TEXT UNIQUE
	)
	`)
	if err != nil {
		app.Log.Fatalf("Error creating reminders table: %v", err)
	}

	_, err = DB.Exec(`
	CREATE TABLE IF NOT EXISTS tg_user_full_name (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
    full_name TEXT UNIQUE
	)
	`)
	if err != nil {
		app.Log.Fatalf("Error creating reminders table: %v", err)
	}

	_, err = DB.Exec(`
	CREATE TABLE IF NOT EXISTS tg_user_delivery_address (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	delivery_address TEXT UNIQUE
	)
	`)
	if err != nil {
		app.Log.Fatalf("Error creating reminders table: %v", err)
	}

	_, err = DB.Exec(`
	CREATE TABLE IF NOT EXISTS tg_user_orders_date (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER,
    phone_number_id INTEGER,
    full_name_id INTEGER,
    delivery_address_id INTEGER,
    date_time TEXT,
    delivery_date DATE,
    total_cost INTEGER,
    FOREIGN KEY (user_id) REFERENCES tg_user_Chat_ID(id),
    FOREIGN KEY (phone_number_id) REFERENCES tg_user_phone_number(id),
    FOREIGN KEY (full_name_id) REFERENCES tg_user_full_name(id),
    FOREIGN KEY (delivery_address_id) REFERENCES tg_user_delivery_address(id)
	)	
	`)
	if err != nil {
		app.Log.Fatalf("Error creating reminders table: %v", err)
	}

}

// CloseDB - закрытие соединения с базой данных.
func CloseDB(app *structs.Application) {

	if DB != nil {
		err := DB.Close()
		if err != nil {
			app.Log.Errorf("Error closing database: %v", err)
			return
		}
	}
}

func UserID(app *structs.Application) []structs.User {

	row, err := DB.Query(`SELECT user_id FROM tg_user_Chat_ID`)
	if err != nil {
		app.Log.Errorf("Error getting user id: %v", err)
		return nil
	}

	var user []structs.User

	for row.Next() {
		u := structs.User{}
		err = row.Scan(&u.UserID)
		if err != nil {
			app.Log.Errorf("Error getting user id: %v", err)
			return nil
		}
		user = append(user, u)
	}
	return user
}

// getOrInsertValues Функция для получения или вставки значений в таблицы базы данных.
func getOrInsertValues(tx *sql.Tx, tables []string, columns []string, values []interface{}) ([]int64, error) {
	// Настраиваем логгер
	logger := loggers.SetupLogger()

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
func ProcessOrder(db *sql.DB, update tgbotapi.Update) error {
	// Настраиваем логгер
	logger := loggers.SetupLogger()

	userID := update.CallbackQuery.Message.Chat.ID
	userName := update.CallbackQuery.From.UserName

	logger.Infof("userName = %s", userName)

	// Регулярные выражения для извлечения информации
	var phoneNumber string
	var deliveryAddress string
	var deliveryDate string
	var fullName string
	var totalCost int

	// Извлечение основной информации о заказе
	//"Номер телефона: [ваш номер]\n" +
	if field, err := bychat.GetFieldByChatID(userID, "PhoneNumber"); err == nil {
		str, _ := field.(string)
		phoneNumber = str
	}

	//"Адрес доставки: [ваш адрес]\n" +
	if field, err := bychat.GetFieldByChatID(userID, "DeliveryAddress"); err == nil {
		str, _ := field.(string)
		deliveryAddress = str
	}

	//"Дата доставки: [желаемая дата]\n" +
	if field, err := bychat.GetFieldByChatID(userID, "DeliveryDate"); err == nil {
		str, _ := field.(string)
		deliveryDate = str
	}

	//"ФИО: [ваше имя и фамилия]\n\n" +
	if field, err := bychat.GetFieldByChatID(userID, "FullName"); err == nil {
		str, _ := field.(string)
		fullName = str
	}

	// Получение содержимого корзины
	purchases := structs.Baskets[userID]

	groupedPurchases := make(map[string]structs.Purchase)
	for _, p := range purchases {

		key := p.Item
		if existing, found := groupedPurchases[key]; found {

			existing.Quantity += p.Quantity
			existing.Cost += p.Cost
			groupedPurchases[key] = existing

		} else {

			groupedPurchases[key] = p

		}
	}

	for _, p := range groupedPurchases {

		itemTotalCost := p.Cost // Общая стоимость для этой группы товаров

		totalCost += itemTotalCost
	}

	// Парсинг строки времени в тип time.Time
	t, err := time.Parse("02.01.2006", deliveryDate)
	if err != nil {
		logger.Errorf("Ошибка парсинга строки времени: %v", err)
		return err
	}

	// Преобразуем дату в строку формата "YYYY-MM-DD" для вставки в базу данных
	formattedDeliveryDate := t.Format("2006-01-02")

	// Начало транзакции
	tx, err := db.Begin()
	if err != nil {
		logger.Errorf("Error starting transaction: %v", err)
		return err
	}

	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				logger.Errorf("tx.Rollback() failed: %v", rbErr)
			}
		} else {
			if cmErr := tx.Commit(); cmErr != nil {
				logger.Errorf("tx.Commit() failed: %v", cmErr)
			}
		}
	}()

	userBlack := 0

	err = UpsertUser(tx, userID, userName, userBlack)
	if err != nil {
		logger.Errorf("Error upserting user: %v", err)
	}

	// Форматирование текущей даты и времени в указанный формат
	formattedDateTime := time.Now().Format("15:04 02.01.2006")

	// Вставка новой записи в tg_user_Chat_ID
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
	values = []interface{}{existingUserID, existingPhoneID, existingFullNameID, existingAddressID, formattedDateTime, formattedDeliveryDate, totalCost}

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

	for _, p := range groupedPurchases {
		itemTotalCost := p.Cost // Общая стоимость для этой группы товаров
		itemName := p.Item
		quantity := p.Quantity
		cost := itemTotalCost

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
	logger := loggers.SetupLogger()

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

// GetOrdersByUserID Функция для выбора из базы данных.
func GetOrdersByUserID(userID int64, limit uint8) ([]structs.OrderDb, error) {
	// Настраиваем логгер
	logger := loggers.SetupLogger()

	// Выполняем запрос для получения заказов
	rows, err := DB.Query(limitedOrdersQuery, userID, limit)
	if err != nil {
		logger.Printf("Error preparing query: %v", err)
		return nil, err
	}

	return processRows(rows)
}

// GetOrdersWithFilters Функция для выбора данных из базы данных с фильтрацией и сортировкой.
func GetOrdersWithFilters(params structs.QueryParams) ([]structs.OrderDb, error) {
	// Настраиваем логгер
	logger := loggers.SetupLogger()

	query := baseQuery

	// Инициализация пустого среза с помощью make
	queryArgs := make([]interface{}, 0)

	if params.UserID != 0 {
		query += " WHERE tg_user_Chat_ID.user_id = ?"
		queryArgs = append(queryArgs, params.UserID)
	}

	if params.StartDate != "" {
		if len(queryArgs) > 0 {
			query += " AND tg_user_orders_date.delivery_date >= ?"
		} else {
			query += " WHERE tg_user_orders_date.delivery_date >= ?"
		}
		queryArgs = append(queryArgs, params.StartDate)
	}

	if params.EndDate != "" {
		if len(queryArgs) > 0 {
			query += " AND tg_user_orders_date.delivery_date <= ?"
		} else {
			query += " WHERE tg_user_orders_date.delivery_date <= ?"
		}
		queryArgs = append(queryArgs, params.EndDate)
	}

	if params.OrderBy != "" {
		query += " ORDER BY " + params.OrderBy
		if params.OrderDir != "" {
			query += " " + params.OrderDir
		}
	}

	rows, err := DB.Query(query, queryArgs...)
	if err != nil {
		logger.Errorf("Error preparing query %v ", err)
		return nil, err
	}

	return processRows(rows)
}

func UpsertUser(tx *sql.Tx, userID int64, userName string, userBlack int) error {
	// Настраиваем логгер
	logger := loggers.SetupLogger()

	// Вставляем запись, если её нет
	_, err := tx.Exec(`
		INSERT OR IGNORE INTO tg_user_Chat_ID (user_id, user_name, user_black)
		VALUES (?, ?, ?)`, userID, userName, userBlack)
	if err != nil {
		logger.Errorf("error inserting record: %v", err)
		return fmt.Errorf("error inserting record: %v", err)
	}

	// Обновляем user_name, если он отличается
	_, err = tx.Exec(`
		UPDATE tg_user_Chat_ID
		SET user_name = ?
		WHERE user_id = ?
		  AND user_name != ?`, userName, userID, userName)
	if err != nil {
		logger.Errorf("error updating user_name: %v", err)
		return fmt.Errorf("error updating user_name: %v", err)
	}

	// Обновляем user_black, если он отличается
	_, err = tx.Exec(`
		UPDATE tg_user_Chat_ID
		SET user_black = ?
		WHERE user_id = ?
		  AND user_black != ?`, userBlack, userID, userBlack)
	if err != nil {
		logger.Errorf("error updating user_black: %v", err)
		return fmt.Errorf("error updating user_black: %v", err)
	}

	return nil
}

func GetUsers() ([]structs.User, error) {
	// Настраиваем логгер
	logger := loggers.SetupLogger()
	rows, err := DB.Query("SELECT user_id, user_name, user_black FROM tg_user_Chat_ID")
	if err != nil {
		logger.Errorf("Error preparing query: %v", err)
		return nil, err
	}

	var users []structs.User
	for rows.Next() {
		var user structs.User
		err := rows.Scan(&user.UserID, &user.UserBlack.UserName, &user.UserBlack.UserBlack)
		if err != nil {
			logger.Errorf("Error scanning rows: %v", err)
			return nil, err
		}

		users = append(users, user)

		// Добавляем в мапу, если user_black не равен 0
		//if user.UserBlack.UserBlack != 0 {
		UserBlack[user.UserID] = user.UserBlack.UserBlack
		//}
	}

	if err = rows.Err(); err != nil {
		logger.Errorf("Error after iterating rows: %v", err)
		return nil, err
	}

	return users, nil
}

func UpdateUserBlack(userID, userBlack int) error {
	// Настраиваем логгер
	logger := loggers.SetupLogger()
	_, err := DB.Exec("UPDATE tg_user_Chat_ID SET user_black = ? WHERE user_id = ?", userBlack, userID)
	if err != nil {
		logger.Errorf("Error updating user_name: %v", err)
	}

	return err
}

// backupDatabase резервного копирования базы данных SQLite.
func backupDatabase() {
	// Настраиваем логгер
	logger := loggers.SetupLogger()
	// Путь к файлу текущей и новой резервной копии
	oldBackupPath := "./backup_old.db"
	newBackupPath := "./backup.db"

	// Создаем новую резервную копию
	inputFile, err := os.Open("./data.db")
	if err != nil {
		logger.Errorf("Ошибка не удается открыть базу данных: %v", err)
	}
	defer func(inputFile *os.File) {
		_ = inputFile.Close()
	}(inputFile)

	outputFile, err := os.Create(newBackupPath)
	if err != nil {
		logger.Errorf("Ошибка при создании нового файла резервной копии: %v", err)
	}
	_, err = io.Copy(outputFile, inputFile)
	if err != nil {
		logger.Errorf("Ошибка при копировании базы данных в файл резервной копии: %v", err)
	}
	_ = outputFile.Close() // Закрываем файл перед переименованием

	// Ждем немного, чтобы ОС завершила все операции с файлом
	time.Sleep(500 * time.Millisecond)

	// Удаляем старую резервную копию, если она существует
	if _, err := os.Stat(oldBackupPath); err == nil {
		err = os.Remove(oldBackupPath)
		if err != nil {
			logger.Errorf("Ошибка при удалении старого файла резервной копии: %v", err)
		}
	} else if !os.IsNotExist(err) {
		logger.Errorf("Error checking old backup file: %v", err)
	}

	// Переименовываем новую резервную копию в старую
	err = os.Rename(newBackupPath, oldBackupPath)
	if err != nil {
		logger.Fatalf("Error renaming new backup file to old: %v", err)
	}

	logger.Info("Backup completed successfully and old backup removed.")
}

// StartBackupRoutine запускает Go-рутину для ежедневного копирования базы данных SQLite.
func StartBackupRoutine() {
	go func() {
		for {
			now := time.Now()
			// Вычисляем время до следующей полуночи
			// Устанавливаем время следующего запуска на 8:00 утра текущего дня
			next := time.Date(now.Year(), now.Month(), now.Day(), 17, 10, 0, 0, now.Location())

			// Если текущее время уже после 8:00 утра, то устанавливаем следующий запуск на 8:00 утра следующего дня
			if now.After(next) {
				next = next.Add(24 * time.Hour)
			}

			// Создаем таймер до следующего запуска
			duration := time.Until(next)
			timer := time.NewTimer(duration)
			<-timer.C

			// Выполняем резервное копирование
			backupDatabase()

			// Останавливаем таймер и переустанавливаем его на следующие 24 часа
			//timer.Stop()
			//timer.Reset(24 * time.Hour)

		}
	}()
}

// GetByUserID Функция для выбора из базы данных.
func GetByUserID(userID int64) structs.OrderDb {

	var o structs.OrderDb
	// Выполняем запрос для получения заказов
	_ = DB.QueryRow(baseQueryOrders, userID).Scan(&o.UserID.UserID, &o.FullName, &o.PhoneNumber, &o.DeliveryAddress)

	return o
}
