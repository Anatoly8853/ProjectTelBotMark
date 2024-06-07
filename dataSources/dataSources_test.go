package dataSources

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"testing"
)

// setupTestDB создает тестовую базу данных и заполняет ее необходимыми таблицами.
func setupTestDB() (*sql.DB, error) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS tg_user_Chat_ID (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER UNIQUE	
	)		
	`)
	if err != nil {
		return nil, err
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
		return nil, err
	}

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS tg_user_order_id (
		id INTEGER PRIMARY KEY AUTOINCREMENT,    
		item_name TEXT     
	)
	`)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS tg_user_phone_number (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		phone_number TEXT UNIQUE
	)
	`)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS tg_user_full_name (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		full_name TEXT UNIQUE
	)
	`)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`
	CREATE TABLE IF NOT EXISTS tg_user_delivery_address (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		delivery_address TEXT UNIQUE
	)
	`)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`
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
		return nil, err
	}

	return db, nil
}

// TestProcessOrder тестирует функцию ProcessOrder.
func TestProcessOrder(t *testing.T) {
	db, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to setup test database: %v", err)
	}
	defer db.Close()

	order := `ФИО: Иван Иванов
Номер телефона: +79991234567
Адрес доставки: Москва, ул. Ленина, д. 1
Дата доставки: 09.06.2024
Товар: Лобио ведёрко 0,9 кол-во: 1 цена: 150 руб.
Товар: Грибы шампиньоны ведёрко 0,9 кол-во: 1 цена: 350 руб.
Товар: Грибы шиитаке ведёрко 0,5 кол-во: 1 цена: 400 руб.
Товар: Морковь ведёрко 0,9 кол-во: 1 цена: 110 руб.
Товар: Баклажаны соломка 0,5 кол-во: 1 цена: 150 руб.
Общая стоимость: 1160 руб.`

	// Добавление пользователя в tg_user_Chat_ID
	_, err = db.Exec(`INSERT INTO tg_user_Chat_ID (user_id) VALUES (367922775)`)
	assert.NoError(t, err)

	// Добавление записей в tg_user_order_id для теста
	items := []string{
		"Лобио ведёрко 0,9",
		"Грибы шампиньоны ведёрко 0,9",
		"Грибы шиитаке ведёрко 0,5",
		"Морковь ведёрко 0,9",
		"Баклажаны соломка 0,5",
	}

	for _, item := range items {
		_, err = db.Exec(`INSERT INTO tg_user_order_id (item_name) VALUES (?)`, item)
		assert.NoError(t, err)
	}

	err = ProcessOrder(order, db, 367922775)
	assert.NoError(t, err)

	// Проверка общей стоимости
	var totalCost int
	row := db.QueryRow(`SELECT total_cost FROM tg_user_orders_date WHERE user_id = (SELECT id FROM tg_user_Chat_ID WHERE user_id = 367922775)`)
	err = row.Scan(&totalCost)
	assert.NoError(t, err)
	assert.Equal(t, 1160, totalCost)

	// Проверка вставки данных в tg_user_orders
	rows, err := db.Query(`
	SELECT i.item_name, o.quantity, o.cost
	FROM tg_user_orders o
	JOIN tg_user_order_id i ON o.item_name = i.id
	WHERE order_id = (SELECT id FROM tg_user_orders_date WHERE user_id = (SELECT id FROM tg_user_Chat_ID WHERE user_id = 367922775))
	`)
	assert.NoError(t, err)
	defer rows.Close()

	var actualItems []struct {
		Name     string
		Quantity int
		Cost     int
	}

	for rows.Next() {
		var item struct {
			Name     string
			Quantity int
			Cost     int
		}
		err := rows.Scan(&item.Name, &item.Quantity, &item.Cost)
		assert.NoError(t, err)
		actualItems = append(actualItems, item)
	}

	expectedItems := []struct {
		Name     string
		Quantity int
		Cost     int
	}{
		{Name: "Лобио ведёрко 0,9", Quantity: 1, Cost: 150},
		{Name: "Грибы шампиньоны ведёрко 0,9", Quantity: 1, Cost: 350},
		{Name: "Грибы шиитаке ведёрко 0,5", Quantity: 1, Cost: 400},
		{Name: "Морковь ведёрко 0,9", Quantity: 1, Cost: 110},
		{Name: "Баклажаны соломка 0,5", Quantity: 1, Cost: 150},
	}

	assert.Equal(t, expectedItems, actualItems)
}
