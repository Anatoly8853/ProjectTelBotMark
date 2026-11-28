package dataSources

import (
	"ProjectTelBotMark/internal/bychat"
	"ProjectTelBotMark/internal/structs"
	"context"

	slogavp "github.com/Anatoly8853/slog-avp/v2"

	"database/sql"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	_ "github.com/mattn/go-sqlite3"
)

type Repository struct {
	db  *sql.DB
	app *slogavp.Application
}

func NewRepository(db *sql.DB, app *slogavp.Application) *Repository {
	return &Repository{db, app}
}

var (
	UserBlack   = make(map[int64]int)
	UserBlackMu sync.RWMutex
)

// GetUserBlack безопасно возвращает статус пользователя из общего чёрного списка.
func GetUserBlack(userID int64) int {
	UserBlackMu.RLock()
	status := UserBlack[userID]
	UserBlackMu.RUnlock()
	return status
}

// SetUserBlack обновляет статус пользователя в общей карте.
func SetUserBlack(userID int64, status int) {
	UserBlackMu.Lock()
	UserBlack[userID] = status
	UserBlackMu.Unlock()
}

// Общая часть запроса
const baseQuery = `
	SELECT
		tg_user_Chat_ID.user_id,
		tg_user_orders_date.id,
		tg_user_full_name.full_name,
		tg_user_orders_date.date_time,
		tg_user_orders_date.total_cost,
		tg_user_orders_date.delivery_date,
		tg_user_phone_number.phone_number,
		tg_user_delivery_address.delivery_address,
		COALESCE(tg_user_orders_date.comment, '') AS comment,
		COALESCE(tg_user_orders_date.payment_method, '') AS payment_method,
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
            tg_user_orders_date.delivery_date DESC, tg_user_orders_date.id DESC
        LIMIT ?
    )
    SELECT
        tg_user_Chat_ID.user_id,
        tg_user_orders_date.id,
        tg_user_full_name.full_name,
        tg_user_orders_date.date_time,
        tg_user_orders_date.total_cost,
        tg_user_orders_date.delivery_date,
        tg_user_phone_number.phone_number,
        tg_user_delivery_address.delivery_address,
        COALESCE(tg_user_orders_date.comment, '') AS comment,
        COALESCE(tg_user_orders_date.payment_method, '') AS payment_method,
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
        tg_user_orders_date.delivery_date DESC, tg_user_orders_date.id DESC, tg_user_orders.id ASC;
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
func InitDB(app *slogavp.Application) (db *sql.DB, err error) {
	// Открываем соединение с базой данных SQLite
	ctx := context.Background()

	db, err = sql.Open("sqlite3", "./sqlite/data.db?_busy_timeout=5000&_foreign_keys=on&_journal_mode=WAL&_txlock=immediate")
	if err != nil {
		app.Log.Errorf("InitDB: ошибка открытия базы данных: %v", err)
		return nil, err
	}
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(5)

	_, err = db.ExecContext(ctx, `
	CREATE TABLE IF NOT EXISTS tg_user_Chat_ID (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER UNIQUE,
    user_name TEXT,
    user_black INTEGER DEFAULT 0 	
	)		
	    `)
	if err != nil {
		app.Log.Fatalf("Ошибка создания таблицы tg_user_Chat_ID: %v", err)
		return nil, err
	}

	_, err = db.ExecContext(ctx, `
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
		app.Log.Fatalf("Error creating reminders table tg_user_orders: %v", err)
		return nil, err
	}

	_, err = db.ExecContext(ctx, `
	CREATE TABLE IF NOT EXISTS tg_user_order_id (
    id INTEGER PRIMARY KEY AUTOINCREMENT,    
    item_name TEXT     
	)
	`)
	if err != nil {
		app.Log.Fatalf("Error creating reminders table tg_user_order_id: %v", err)
		return nil, err
	}

	_, err = db.ExecContext(ctx, `
	CREATE TABLE IF NOT EXISTS tg_user_phone_number (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	phone_number TEXT UNIQUE
	)
	`)
	if err != nil {
		app.Log.Fatalf("Error creating reminders table tg_user_phone_number: %v", err)
		return nil, err
	}

	_, err = db.ExecContext(ctx, `
	CREATE TABLE IF NOT EXISTS tg_user_full_name (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
    full_name TEXT UNIQUE
	)
	`)
	if err != nil {
		app.Log.Fatalf("Error creating reminders table tg_user_full_name: %v", err)
		return nil, err
	}

	_, err = db.ExecContext(ctx, `
	CREATE TABLE IF NOT EXISTS tg_user_delivery_address (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	delivery_address TEXT UNIQUE
	)
	`)
	if err != nil {
		app.Log.Fatalf("Error creating reminders table tg_user_delivery_address: %v", err)
		return nil, err
	}

	_, err = db.ExecContext(ctx, `
	CREATE TABLE IF NOT EXISTS tg_user_orders_date (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER,
    phone_number_id INTEGER,
    full_name_id INTEGER,
    delivery_address_id INTEGER,
    date_time TEXT,
    delivery_date DATE,
    total_cost INTEGER,
    comment TEXT NOT NULL DEFAULT '',
    payment_method TEXT NOT NULL DEFAULT '',
    FOREIGN KEY (user_id) REFERENCES tg_user_Chat_ID(id),
    FOREIGN KEY (phone_number_id) REFERENCES tg_user_phone_number(id),
    FOREIGN KEY (full_name_id) REFERENCES tg_user_full_name(id),
    FOREIGN KEY (delivery_address_id) REFERENCES tg_user_delivery_address(id)
	)	
	`)
	if err != nil {
		app.Log.Fatalf("Error creating reminders table tg_user_orders_date: %v", err)
		return nil, err
	}

	// Добавляем поле комментария в уже существующие базы данных.
	rows, err := db.QueryContext(ctx, `PRAGMA table_info(tg_user_orders_date)`)
	if err != nil {
		app.Log.Errorf("Ошибка проверки структуры tg_user_orders_date: %v", err)
		return nil, err
	}
	commentColumnExists := false
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, columnType string
		var defaultValue any
		if err = rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			_ = rows.Close()
			app.Log.Errorf("Ошибка чтения структуры tg_user_orders_date: %v", err)
			return nil, err
		}
		if name == "comment" {
			commentColumnExists = true
		}
	}
	if err = rows.Err(); err != nil {
		_ = rows.Close()
		app.Log.Errorf("Ошибка обхода структуры tg_user_orders_date: %v", err)
		return nil, err
	}
	if err = rows.Close(); err != nil {
		app.Log.Errorf("Ошибка закрытия проверки структуры tg_user_orders_date: %v", err)
		return nil, err
	}
	if !commentColumnExists {
		if _, err = db.ExecContext(ctx, `ALTER TABLE tg_user_orders_date ADD COLUMN comment TEXT NOT NULL DEFAULT ''`); err != nil {
			app.Log.Errorf("Ошибка добавления столбца comment в tg_user_orders_date: %v", err)
			return nil, err
		}
		app.Log.Info("В базу данных добавлен столбец комментария заказа")
	}

	// Добавляем поле способа оплаты в уже существующие базы данных.
	paymentRows, err := db.QueryContext(ctx, `PRAGMA table_info(tg_user_orders_date)`)
	if err != nil {
		app.Log.Errorf("Ошибка проверки поля payment_method в tg_user_orders_date: %v", err)
		return nil, err
	}
	paymentMethodColumnExists := false
	for paymentRows.Next() {
		var cid, notNull, primaryKey int
		var name, columnType string
		var defaultValue any
		if err = paymentRows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			_ = paymentRows.Close()
			app.Log.Errorf("Ошибка чтения структуры payment_method в tg_user_orders_date: %v", err)
			return nil, err
		}
		if name == "payment_method" {
			paymentMethodColumnExists = true
		}
	}
	if err = paymentRows.Err(); err != nil {
		_ = paymentRows.Close()
		app.Log.Errorf("Ошибка обхода структуры payment_method в tg_user_orders_date: %v", err)
		return nil, err
	}
	if err = paymentRows.Close(); err != nil {
		app.Log.Errorf("Ошибка закрытия проверки payment_method в tg_user_orders_date: %v", err)
		return nil, err
	}
	if !paymentMethodColumnExists {
		if _, err = db.ExecContext(ctx, `ALTER TABLE tg_user_orders_date ADD COLUMN payment_method TEXT NOT NULL DEFAULT ''`); err != nil {
			app.Log.Errorf("Ошибка добавления столбца payment_method в tg_user_orders_date: %v", err)
			return nil, err
		}
		app.Log.Info("В базу данных добавлен столбец способа оплаты заказа")
	}

	_, err = db.ExecContext(ctx, `
	CREATE TABLE IF NOT EXISTS categories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    display_name TEXT       -- например, 🥬Капуста🥬   
    )
	`)
	if err != nil {
		app.Log.Fatalf("Error creating reminders table categories: %v", err)
		return nil, err
	}

	_, err = db.ExecContext(ctx, `
	CREATE TABLE IF NOT EXISTS categories_2 (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL UNIQUE,
	display_name TEXT NOT NULL DEFAULT '',
	description TEXT NOT NULL
    )
	`)
	if err != nil {
		app.Log.Errorf("InitDB: ошибка создания таблицы categories_2: %v", err)
		return nil, err
	}

	// Совместимость с существующими базами: добавляем display_name,
	// если таблица categories_2 была создана старой версией приложения.
	subcategoryColumns, err := db.QueryContext(ctx, `PRAGMA table_info(categories_2)`)
	if err != nil {
		app.Log.Errorf("InitDB: ошибка проверки структуры categories_2: %v", err)
		return nil, err
	}
	displayNameExists := false
	for subcategoryColumns.Next() {
		var cid, notNull, primaryKey int
		var name, columnType string
		var defaultValue any
		if err = subcategoryColumns.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			_ = subcategoryColumns.Close()
			app.Log.Errorf("InitDB: ошибка чтения структуры categories_2: %v", err)
			return nil, err
		}
		if name == "display_name" {
			displayNameExists = true
		}
	}
	if err = subcategoryColumns.Err(); err != nil {
		_ = subcategoryColumns.Close()
		app.Log.Errorf("InitDB: ошибка обхода структуры categories_2: %v", err)
		return nil, err
	}
	if err = subcategoryColumns.Close(); err != nil {
		app.Log.Errorf("InitDB: ошибка закрытия проверки categories_2: %v", err)
		return nil, err
	}
	if !displayNameExists {
		if _, err = db.ExecContext(ctx, `ALTER TABLE categories_2 ADD COLUMN display_name TEXT NOT NULL DEFAULT ''`); err != nil {
			app.Log.Errorf("InitDB: ошибка добавления display_name в categories_2: %v", err)
			return nil, err
		}
		app.Log.Info("InitDB: в categories_2 добавлен столбец display_name")
	}
	if _, err = db.ExecContext(ctx, `UPDATE categories_2 SET display_name = name WHERE display_name = ''`); err != nil {
		app.Log.Errorf("InitDB: ошибка заполнения display_name в categories_2: %v", err)
		return nil, err
	}

	_, err = db.ExecContext(ctx, `
	CREATE TABLE IF NOT EXISTS photo (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT
    )
	`)
	if err != nil {
		app.Log.Fatalf("Error creating reminders table packagings: %v", err)
		return nil, err
	}

	_, err = db.ExecContext(ctx, `
	CREATE TABLE IF NOT EXISTS products_categories (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	category_id INTEGER NOT NULL,
	categories_2_id INTEGER NOT NULL,
	photo_id INTEGER NOT NULL,
	weight_kg TEXT NOT NULL,
	price_rub INTEGER NOT NULL,
	text TEXT NOT NULL,
	callback TEXT NOT NULL,
	FOREIGN KEY (category_id) REFERENCES categories(id),
	FOREIGN KEY (categories_2_id) REFERENCES categories_2(id),
	FOREIGN KEY (photo_id) REFERENCES photo(id)	
    )
	`)
	if err != nil {
		app.Log.Fatalf("Error creating reminders table packagings: %v", err)
		return nil, err
	}

	_, err = db.ExecContext(ctx, `
	CREATE TABLE IF NOT EXISTS tg_user_favorites (
		user_id INTEGER NOT NULL,
		product_id INTEGER NOT NULL,
		created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (user_id, product_id),
		FOREIGN KEY (product_id) REFERENCES products_categories(id) ON DELETE CASCADE
	)
	`)
	if err != nil {
		app.Log.Errorf("InitDB: ошибка создания таблицы избранного: %v", err)
		return nil, err
	}

	return db, nil
}

// CloseDB - закрытие соединения с базой данных.
func (r *Repository) CloseDB() {

	if r.db != nil {
		err := r.db.Close()
		if err != nil {
			r.app.Log.Errorf("Error closing database: %v", err)
			return
		}
	}
}

func (r *Repository) UserID() (users []structs.User, err error) {
	ctx := context.Background()
	rows, err := r.db.QueryContext(ctx, `SELECT user_id FROM tg_user_Chat_ID`)
	if err != nil {
		r.app.Log.Errorf("UserID: ошибка получения списка пользователей: %v", err)
		return nil, err
	}

	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			r.app.Log.Errorf("UserID: ошибка закрытия SQL rows: %v", closeErr)
			if err == nil {
				err = closeErr
			}
		}
	}()

	users = make([]structs.User, 0)
	for rows.Next() {
		var user structs.User
		if err = rows.Scan(&user.UserID); err != nil {
			r.app.Log.Errorf("UserID: ошибка чтения ID пользователя: %v", err)
			return nil, err
		}
		users = append(users, user)
	}

	if err = rows.Err(); err != nil {
		r.app.Log.Errorf("UserID: ошибка обхода SQL rows: %v", err)
		return nil, err
	}

	return users, nil
}

// getOrInsertValues Функция для получения или вставки значений в таблицы базы данных.
func (r *Repository) getOrInsertValues(tx *sql.Tx, tables []string, columns []string, values []any) ([]int64, error) {
	ctx := context.Background()
	var ids []int64

	// Проверяем, вставляем ли данные в одну таблицу.
	if len(tables) == 1 {
		table := tables[0]
		var idd int64

		var selectQuery, insertQuery, valuesPlaceholders strings.Builder
		selectQuery.WriteString("SELECT id FROM ")
		selectQuery.WriteString(table)
		selectQuery.WriteString(" WHERE ")
		insertQuery.WriteString("INSERT INTO ")
		insertQuery.WriteString(table)
		insertQuery.WriteString(" (")
		valuesPlaceholders.WriteString("VALUES (")

		for i, column := range columns {
			if i > 0 {
				selectQuery.WriteString(" AND ")
				insertQuery.WriteString(", ")
				valuesPlaceholders.WriteString(", ")
			}
			selectQuery.WriteString(column)
			selectQuery.WriteString(" = ?")
			insertQuery.WriteString(column)
			valuesPlaceholders.WriteString("?")
		}

		insertQuery.WriteString(") ")
		insertQuery.WriteString(valuesPlaceholders.String())
		insertQuery.WriteString(")")

		// Проверяем, существует ли строка.
		err := tx.QueryRowContext(ctx, selectQuery.String(), values...).Scan(&idd)
		if err != nil && err != sql.ErrNoRows {
			r.app.Log.Errorf("Ошибка при выборе строки: %v", err)
			return nil, err
		}

		if idd == 0 {
			// Вставляем новую строку, если она не существует.
			res, err := tx.ExecContext(ctx, insertQuery.String(), values...)
			if err != nil {
				r.app.Log.Errorf("Ошибка при вставке строки: %v", err)
				return nil, err
			}
			idd, err = res.LastInsertId()
			if err != nil {
				r.app.Log.Errorf("Ошибка при получении ID вставленной строки: %v", err)
				return nil, err
			}
		}

		ids = append(ids, idd)
	} else {
		// Вставка данных в несколько таблиц с одной колонкой в каждой.
		for i, table := range tables {
			column := columns[i]
			value := values[i]

			var idd int64
			query := fmt.Sprintf("SELECT id FROM %s WHERE %s = ?", table, column)
			err := tx.QueryRowContext(ctx, query, value).Scan(&idd)
			if err != nil && err != sql.ErrNoRows {
				r.app.Log.Errorf("Ошибка при выборе строки: %v", err)
				return nil, err
			}

			if idd == 0 {
				insertQuery := fmt.Sprintf("INSERT INTO %s (%s) VALUES (?)", table, column)
				res, err := tx.ExecContext(ctx, insertQuery, value)
				if err != nil {
					r.app.Log.Errorf("Ошибка при вставке строки: %v", err)
					return nil, err
				}
				idd, err = res.LastInsertId()
				if err != nil {
					r.app.Log.Errorf("Ошибка при получении ID вставленной строки: %v", err)
					return nil, err
				}
			}
			ids = append(ids, idd)
		}
	}

	return ids, nil
}

// ProcessOrder сохраняет заказ пользователя в базе данных.
func (r *Repository) ProcessOrder(
	update tgbotapi.Update,
	purchases []structs.Purchase,
) error {
	if update.CallbackQuery == nil {
		err := fmt.Errorf("отсутствует CallbackQuery")
		r.app.Log.Error("ProcessOrder", err.Error())
		return err
	}

	if update.CallbackQuery.Message == nil ||
		update.CallbackQuery.Message.Chat == nil {
		err := fmt.Errorf("отсутствует сообщение или Chat")
		r.app.Log.Error("ProcessOrder", err.Error())
		return err
	}

	if update.CallbackQuery.From == nil {
		err := fmt.Errorf("отсутствуют данные пользователя в CallbackQuery.From")
		r.app.Log.Error("ProcessOrder", err.Error())
		return err
	}

	userID := update.CallbackQuery.Message.Chat.ID
	userName := update.CallbackQuery.From.UserName

	if userID == 0 {
		err := fmt.Errorf("некорректный userID: %d", userID)
		r.app.Log.Error("ProcessOrder", err.Error())
		return err
	}

	if len(purchases) == 0 {
		err := fmt.Errorf("корзина пользователя %d пуста", userID)
		r.app.Log.Error("ProcessOrder", err.Error())
		return err
	}

	r.app.Log.Infof(
		"Начало оформления заказа: userID=%d, userName=%q, позиций=%d",
		userID,
		userName,
		len(purchases),
	)

	ctx := context.Background()

	// Проверяем содержимое корзины и рассчитываем общую стоимость тем же способом,
	// что и при отображении корзины в Telegram.
	totalCost, err := structs.CalculatePurchasesTotal(purchases)
	if err != nil {
		r.app.Log.Errorf("ProcessOrder: некорректная корзина: userID=%d, ошибка=%v", userID, err)
		return err
	}

	// Получаем данные пользователя.
	fields := []string{
		"PhoneNumber",
		"DeliveryAddress",
		"DeliveryDate",
		"FullName",
		"PaymentMethod",
	}

	values := make(map[string]string, len(fields)+1)
	values["Comment"] = ""

	for _, field := range fields {
		value, err := bychat.GetFieldByChatID(r.app, userID, field)
		if err != nil {
			r.app.Log.Errorf(
				"ProcessOrder: ошибка получения поля: userID=%d, поле=%s, ошибка=%v",
				userID,
				field,
				err,
			)
			return err
		}

		strValue, ok := value.(string)
		if !ok {
			err := fmt.Errorf(
				"поле имеет неверный тип: userID=%d, поле=%s, тип=%T",
				userID,
				field,
				value,
			)
			r.app.Log.Error("ProcessOrder", err.Error())
			return err
		}

		if strings.TrimSpace(strValue) == "" {
			err := fmt.Errorf(
				"пустое обязательное поле: userID=%d, поле=%s",
				userID,
				field,
			)
			r.app.Log.Error("ProcessOrder", err.Error())
			return err
		}

		values[field] = strValue
	}

	if values["PaymentMethod"] != structs.PaymentMethodCash && values["PaymentMethod"] != structs.PaymentMethodCard {
		err := fmt.Errorf("неизвестный способ оплаты: userID=%d, способ=%q", userID, values["PaymentMethod"])
		r.app.Log.Error("ProcessOrder", err.Error())
		return err
	}

	// Комментарий необязателен, но сохраняется вместе с заказом.
	commentValue, commentErr := bychat.GetFieldByChatID(r.app, userID, "Comment")
	if commentErr != nil {
		r.app.Log.Warnf("ProcessOrder: комментарий не найден в данных пользователя, сохраняем пустое значение: userID=%d, ошибка=%v", userID, commentErr)
	} else if commentText, ok := commentValue.(string); ok {
		values["Comment"] = strings.TrimSpace(commentText)
	} else {
		r.app.Log.Errorf("ProcessOrder: комментарий имеет неверный тип: userID=%d, тип=%T", userID, commentValue)
	}
	r.app.Log.Infof(
		"ProcessOrder: комментарий получен для сохранения: userID=%d, заполнен=%t, длина=%d",
		userID,
		values["Comment"] != "",
		len([]rune(values["Comment"])),
	)

	// Проверяем дату доставки.
	t, err := time.Parse("02.01.2006", values["DeliveryDate"])
	if err != nil {
		r.app.Log.Errorf(
			"ProcessOrder: ошибка разбора даты доставки: userID=%d, дата=%q, ошибка=%v",
			userID,
			values["DeliveryDate"],
			err,
		)
		return err
	}

	formattedDeliveryDate := t.Format("2006-01-02")
	formattedDateTime := time.Now().Format("15:04 02.01.2006")

	// Начинаем транзакцию.
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		r.app.Log.Errorf(
			"ProcessOrder: ошибка начала транзакции: userID=%d, ошибка=%v",
			userID,
			err,
		)
		return err
	}

	committed := false

	defer func() {
		if !committed {
			if rbErr := tx.Rollback(); rbErr != nil {
				r.app.Log.Errorf(
					"ProcessOrder: ошибка отката транзакции: userID=%d, ошибка=%v",
					userID,
					rbErr,
				)
			}
		}
	}()

	// Сохраняем пользователя.
	userBlack := 0

	if err = r.UpsertUser(tx, userID, userName, userBlack); err != nil {
		r.app.Log.Errorf(
			"ProcessOrder: ошибка сохранения пользователя: userID=%d, ошибка=%v",
			userID,
			err,
		)
		return err
	}

	// Сохраняем связанные данные пользователя.
	tables := []string{
		"tg_user_Chat_ID",
		"tg_user_full_name",
		"tg_user_phone_number",
		"tg_user_delivery_address",
	}

	columns := []string{
		"user_id",
		"full_name",
		"phone_number",
		"delivery_address",
	}

	valueList := []any{
		userID,
		values["FullName"],
		values["PhoneNumber"],
		values["DeliveryAddress"],
	}

	ids, err := r.getOrInsertValues(tx, tables, columns, valueList)
	if err != nil {
		r.app.Log.Errorf(
			"ProcessOrder: ошибка сохранения данных пользователя: userID=%d, ошибка=%v",
			userID,
			err,
		)
		return err
	}

	if len(ids) != 4 {
		err := fmt.Errorf(
			"неверное количество ID после сохранения данных пользователя: userID=%d, получено=%d, ожидалось=4",
			userID,
			len(ids),
		)
		r.app.Log.Error("ProcessOrder", err.Error())
		return err
	}

	existingUserID := ids[0]
	existingFullNameID := ids[1]
	existingPhoneID := ids[2]
	existingAddressID := ids[3]

	// Создаём запись заказа.
	tables = []string{"tg_user_orders_date"}

	columns = []string{
		"user_id",
		"phone_number_id",
		"full_name_id",
		"delivery_address_id",
		"date_time",
		"delivery_date",
		"total_cost",
		"comment",
		"payment_method",
	}

	valueList = []any{
		existingUserID,
		existingPhoneID,
		existingFullNameID,
		existingAddressID,
		formattedDateTime,
		formattedDeliveryDate,
		totalCost,
		values["Comment"],
		values["PaymentMethod"],
	}

	insertOrderQuery := `INSERT INTO tg_user_orders_date
		(user_id, phone_number_id, full_name_id, delivery_address_id, date_time, delivery_date, total_cost, comment, payment_method)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	result, err := tx.ExecContext(ctx, insertOrderQuery, valueList...)
	if err != nil {
		r.app.Log.Errorf("ProcessOrder: ошибка создания отдельной записи заказа: userID=%d, ошибка=%v", userID, err)
		return err
	}
	existingOrdersDateID, err := result.LastInsertId()
	if err != nil {
		r.app.Log.Errorf("ProcessOrder: не удалось получить ID нового заказа: userID=%d, ошибка=%v", userID, err)
		return err
	}

	// Подготавливаем добавление товаров в заказ.
	stmt, err := tx.PrepareContext(
		ctx,
		`INSERT INTO tg_user_orders
		(order_id, item_name, quantity, cost)
		VALUES (?, ?, ?, ?)`,
	)
	if err != nil {
		r.app.Log.Errorf(
			"ProcessOrder: ошибка подготовки запроса товаров: userID=%d, ошибка=%v",
			userID,
			err,
		)
		return err
	}

	// Гарантируем закрытие подготовленного запроса и до Commit.
	stmtClosed := false
	defer func() {
		if !stmtClosed {
			if closeErr := stmt.Close(); closeErr != nil {
				r.app.Log.Errorf(
					"ProcessOrder: ошибка закрытия SQL-запроса: userID=%d, ошибка=%v",
					userID,
					closeErr,
				)
			}
		}
	}()

	// Сохраняем каждую позицию корзины.
	for _, p := range purchases {
		tables = []string{"tg_user_order_id"}
		columns = []string{"item_name"}
		valueList = []any{p.Item}

		ids, err = r.getOrInsertValues(tx, tables, columns, valueList)
		if err != nil {
			r.app.Log.Errorf(
				"ProcessOrder: ошибка получения ID товара: userID=%d, товар=%q, ошибка=%v",
				userID,
				p.Item,
				err,
			)
			return err
		}

		if len(ids) != 1 {
			err := fmt.Errorf(
				"неверное количество ID товара: userID=%d, товар=%q, получено=%d",
				userID,
				p.Item,
				len(ids),
			)
			r.app.Log.Error("ProcessOrder", err.Error())
			return err
		}

		// Cost — цена одной единицы товара.
		// Quantity — количество единиц.
		_, err = stmt.ExecContext(
			ctx,
			existingOrdersDateID,
			ids[0],
			p.Quantity,
			p.Cost,
		)
		if err != nil {
			r.app.Log.Errorf(
				"ProcessOrder: ошибка сохранения позиции заказа: userID=%d, товар=%q, количество=%d, цена=%d, ошибка=%v",
				userID,
				p.Item,
				p.Quantity,
				p.Cost,
				err,
			)
			return err
		}
	}

	if err = stmt.Close(); err != nil {
		r.app.Log.Errorf("ProcessOrder: ошибка закрытия SQL-запроса: userID=%d, ошибка=%v", userID, err)
		return err
	}
	stmtClosed = true

	// Завершаем транзакцию.
	if err = tx.Commit(); err != nil {
		r.app.Log.Errorf(
			"ProcessOrder: ошибка фиксации транзакции: userID=%d, ошибка=%v",
			userID,
			err,
		)
		return err
	}

	committed = true

	r.app.Log.Infof(
		"Заказ успешно сохранён: userID=%d, orderID=%d, позиций=%d, сумма=%d руб.",
		userID,
		existingOrdersDateID,
		len(purchases),
		totalCost,
	)

	return nil
}

// processRows функция для обработки строк результата SQL-запроса.
func (r *Repository) processRows(rows *sql.Rows) ([]structs.OrderDb, error) {
	if rows == nil {
		err := fmt.Errorf("processRows: получен nil вместо SQL rows")
		r.app.Log.Error(err.Error())
		return nil, err
	}

	var orders []structs.OrderDb

	for rows.Next() {
		var order structs.OrderDb
		err := rows.Scan(
			&order.UserID.UserID,
			&order.OrderID,
			&order.FullName,
			&order.DateTime,
			&order.TotalCost,
			&order.DeliveryDate,
			&order.PhoneNumber,
			&order.DeliveryAddress,
			&order.Comment,
			&order.PaymentMethod,
			&order.Purchase.Item,
			&order.Purchase.Quantity,
			&order.Purchase.Cost,
		)
		if err != nil {
			r.app.Log.Errorf("Error scanning rows: %v", err)
			return nil, err
		}
		orders = append(orders, order)
	}

	if err := rows.Err(); err != nil {
		r.app.Log.Errorf("Error after rows.Next(): %v", err)
		return nil, err
	}

	return orders, nil
}

// GetOrdersByUserID Функция для выбора из базы данных.
func (r *Repository) GetOrdersByUserID(userID int64, limit uint8) ([]structs.OrderDb, error) {
	ctx := context.Background()
	// Выполняем запрос для получения заказов
	rows, err := r.db.QueryContext(ctx, limitedOrdersQuery, userID, limit)
	if err != nil {
		r.app.Log.Errorf("GetOrdersByUserID: ошибка запроса заказов userID=%d: %v", userID, err)
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			r.app.Log.Errorf("GetOrdersByUserID: ошибка закрытия SQL rows: %v", closeErr)
		}
	}()

	return r.processRows(rows)
}

// GetOrdersWithFilters Функция для выбора данных из базы данных с фильтрацией и сортировкой.
func (r *Repository) GetOrdersWithFilters(params structs.QueryParams) ([]structs.OrderDb, error) {
	ctx := context.Background()
	query := baseQuery

	// Инициализация пустого среза с помощью make
	queryArgs := make([]any, 0)

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
		query += ", tg_user_orders_date.id DESC, tg_user_orders.id ASC"
	}

	rows, err := r.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		r.app.Log.Errorf("GetOrdersWithFilters: ошибка запроса заказов: %v", err)
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			r.app.Log.Errorf("GetOrdersWithFilters: ошибка закрытия SQL rows: %v", closeErr)
		}
	}()

	return r.processRows(rows)
}

func (r *Repository) UpsertUser(tx *sql.Tx, userID int64, userName string, userBlack int) error {
	ctx := context.Background()
	// Вставляем запись, если её нет
	_, err := tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO tg_user_Chat_ID (user_id, user_name, user_black)
		VALUES (?, ?, ?)`, userID, userName, userBlack)
	if err != nil {
		r.app.Log.Errorf("error inserting record: %v", err)
		return fmt.Errorf("error inserting record: %w", err)
	}

	// Обновляем user_name, если он отличается
	_, err = tx.ExecContext(ctx, `
		UPDATE tg_user_Chat_ID
		SET user_name = ?
		WHERE user_id = ?
		  AND user_name != ?`, userName, userID, userName)
	if err != nil {
		r.app.Log.Errorf("error updating user_name: %v", err)
		return fmt.Errorf("error updating user_name: %w", err)
	}

	return nil
}

func (r *Repository) GetUsers() ([]structs.User, error) {
	ctx := context.Background()
	rows, err := r.db.QueryContext(ctx, "SELECT user_id, user_name, user_black FROM tg_user_Chat_ID")
	if err != nil {
		r.app.Log.Errorf("GetUsers: ошибка выполнения запроса: %v", err)
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			r.app.Log.Errorf("GetUsers: ошибка закрытия SQL rows: %v", closeErr)
		}
	}()

	var users []structs.User
	for rows.Next() {
		var user structs.User
		err = rows.Scan(&user.UserID, &user.UserBlack.UserName, &user.UserBlack.UserBlack)
		if err != nil {
			r.app.Log.Errorf("GetUsers: ошибка чтения пользователя: %v", err)
			return nil, err
		}

		users = append(users, user)
		SetUserBlack(user.UserID, user.UserBlack.UserBlack)
	}

	if err = rows.Err(); err != nil {
		r.app.Log.Errorf("Error after iterating rows: %v", err)
		return nil, err
	}

	return users, nil
}

func (r *Repository) UpdateUserBlack(userID, userBlack int) error {
	ctx := context.Background()
	_, err := r.db.ExecContext(ctx, "UPDATE tg_user_Chat_ID SET user_black = ? WHERE user_id = ?", userBlack, userID)
	if err != nil {
		r.app.Log.Errorf("Error updating user_name: %v", err)
	}

	if err == nil {
		SetUserBlack(int64(userID), userBlack)
	}
	return err
}

// backupDatabase создаёт согласованную резервную копию SQLite через VACUUM INTO.
// В отличие от копирования открытого файла, этот способ учитывает активный журнал SQLite.
func (r *Repository) backupDatabase() {
	const sourceDBPath = "./sqlite/data.db"
	const backupPath = "./backup_old.db"

	if r == nil {
		return
	}
	if r.db == nil {
		if r.app != nil {
			r.app.Log.Error("backupDatabase: соединение с базой данных не инициализировано")
		}
		return
	}
	if r.app == nil {
		return
	}

	tmpFile, err := os.CreateTemp(".", "backup-*.db")
	if err != nil {
		r.app.Log.Errorf("backupDatabase: не удалось создать временный файл: %v", err)
		return
	}
	tmpPath := tmpFile.Name()
	if err := tmpFile.Close(); err != nil {
		r.app.Log.Errorf("backupDatabase: ошибка закрытия временного файла: %v", err)
		_ = os.Remove(tmpPath)
		return
	}
	if err := os.Remove(tmpPath); err != nil {
		r.app.Log.Errorf("backupDatabase: не удалось подготовить временный путь: %v", err)
		return
	}
	defer func() {
		if err := os.Remove(tmpPath); err != nil && !os.IsNotExist(err) {
			r.app.Log.Errorf("backupDatabase: не удалось удалить временный файл %q: %v", tmpPath, err)
		}
	}()

	if _, err := os.Stat(sourceDBPath); err != nil {
		r.app.Log.Errorf("backupDatabase: файл базы данных %q недоступен: %v", sourceDBPath, err)
		return
	}

	if _, err := r.db.Exec(`VACUUM INTO ?`, tmpPath); err != nil {
		r.app.Log.Errorf("backupDatabase: ошибка создания согласованной копии SQLite: %v", err)
		return
	}

	if info, err := os.Stat(tmpPath); err != nil || info.Size() == 0 {
		r.app.Log.Errorf("backupDatabase: резервная копия не создана или пуста: path=%q, ошибка=%v", tmpPath, err)
		return
	}

	if err := os.Rename(tmpPath, backupPath); err != nil {
		// На Windows нельзя переименовать файл поверх существующего.
		previousPath := backupPath + ".previous"
		_ = os.Remove(previousPath)
		if _, statErr := os.Stat(backupPath); statErr == nil {
			if moveErr := os.Rename(backupPath, previousPath); moveErr != nil {
				r.app.Log.Errorf("backupDatabase: не удалось временно сохранить старую копию: %v", moveErr)
				return
			}
		}
		if retryErr := os.Rename(tmpPath, backupPath); retryErr != nil {
			if _, statErr := os.Stat(previousPath); statErr == nil {
				if restoreErr := os.Rename(previousPath, backupPath); restoreErr != nil {
					r.app.Log.Errorf("backupDatabase: не удалось восстановить предыдущую копию: %v", restoreErr)
				}
			}
			r.app.Log.Errorf("backupDatabase: не удалось сохранить резервную копию: %v", retryErr)
			return
		}
		if removeErr := os.Remove(previousPath); removeErr != nil && !os.IsNotExist(removeErr) {
			r.app.Log.Warnf("backupDatabase: не удалось удалить предыдущую копию: %v", removeErr)
		}
	}
	r.app.Log.Infof("backupDatabase: резервная копия успешно создана: %s", backupPath)
}

// StartBackupRoutine запускает Go-рутину для ежедневного копирования базы данных SQLite.
func (r *Repository) StartBackupRoutine() {
	go func() {
		for {
			now := time.Now()
			// Вычисляем время до следующего запуска резервного копирования.
			// Резервная копия создаётся ежедневно в 04:10 по локальному времени.
			next := time.Date(now.Year(), now.Month(), now.Day(), 4, 10, 0, 0, now.Location())

			// Если 04:10 уже прошло, переносим запуск на следующий день.
			if now.After(next) {
				next = next.Add(24 * time.Hour)
			}

			// Создаем таймер до следующего запуска
			duration := time.Until(next)
			timer := time.NewTimer(duration)
			<-timer.C

			// Выполняем резервное копирование
			r.backupDatabase()

			// Останавливаем таймер и переустанавливаем его на следующие 24 часа
			//timer.Stop()
			//timer.Reset(24 * time.Hour)

		}
	}()
}

// GetByUserID возвращает последние сохранённые данные заказа пользователя.
func (r *Repository) GetByUserID(userID int64) (structs.OrderDb, error) {
	ctx := context.Background()
	var order structs.OrderDb

	err := r.db.QueryRowContext(ctx, baseQueryOrders, userID).Scan(
		&order.UserID.UserID,
		&order.FullName,
		&order.PhoneNumber,
		&order.DeliveryAddress,
	)
	if err == sql.ErrNoRows {
		return order, nil
	}
	if err != nil {
		r.app.Log.Errorf("GetByUserID: ошибка получения данных заказа: userID=%d, ошибка=%v", userID, err)
		return structs.OrderDb{}, err
	}

	return order, nil
}
