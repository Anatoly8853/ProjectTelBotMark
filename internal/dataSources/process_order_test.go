package dataSources

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"ProjectTelBotMark/internal/bychat"
	"ProjectTelBotMark/internal/structs"

	slogavp "github.com/Anatoly8853/slog-avp/v2"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func newProcessOrderTestRepository(t *testing.T) (*Repository, *sql.DB) {
	t.Helper()
	app, err := slogavp.New(slogavp.WithConsole(false), slogavp.WithLogDir(t.TempDir()))
	if err != nil {
		t.Fatalf("не удалось создать тестовый логгер: %v", err)
	}
	t.Cleanup(func() { app.Close() })

	dbPath := filepath.Join(t.TempDir(), "orders.db")
	db, err := sql.Open("sqlite3", fmt.Sprintf("%s?_busy_timeout=5000&_foreign_keys=on&_journal_mode=WAL&_txlock=immediate", dbPath))
	if err != nil {
		t.Fatalf("не удалось открыть тестовую SQLite: %v", err)
	}
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(5)

	schema := []string{
		`CREATE TABLE tg_user_Chat_ID (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER UNIQUE, user_name TEXT, user_black INTEGER DEFAULT 0)`,
		`CREATE TABLE tg_user_full_name (id INTEGER PRIMARY KEY AUTOINCREMENT, full_name TEXT UNIQUE)`,
		`CREATE TABLE tg_user_phone_number (id INTEGER PRIMARY KEY AUTOINCREMENT, phone_number TEXT UNIQUE)`,
		`CREATE TABLE tg_user_delivery_address (id INTEGER PRIMARY KEY AUTOINCREMENT, delivery_address TEXT UNIQUE)`,
		`CREATE TABLE tg_user_orders_date (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER, phone_number_id INTEGER, full_name_id INTEGER, delivery_address_id INTEGER, date_time TEXT, delivery_date DATE, total_cost INTEGER, comment TEXT NOT NULL DEFAULT '')`,
		`CREATE TABLE tg_user_order_id (id INTEGER PRIMARY KEY AUTOINCREMENT, item_name TEXT UNIQUE)`,
		`CREATE TABLE tg_user_orders (id INTEGER PRIMARY KEY AUTOINCREMENT, order_id INTEGER, item_name INTEGER, quantity INTEGER, cost INTEGER)`,
	}
	for _, statement := range schema {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("не удалось создать тестовую схему: %v", err)
		}
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewRepository(db, app), db
}

func prepareProcessOrderTestData(t *testing.T, userID int64) tgbotapi.Update {
	t.Helper()
	bychat.OrdersMu.Lock()
	bychat.Orders[userID] = structs.OrderDb{
		FullName:        "Тестовый Покупатель",
		PhoneNumber:     "+79990000000",
		DeliveryAddress: "Тестовый адрес, 1",
		DeliveryDate:    "26.11.2026",
		Comment:         "Тестовый комментарий",
	}
	bychat.OrdersMu.Unlock()
	t.Cleanup(func() {
		bychat.OrdersMu.Lock()
		delete(bychat.Orders, userID)
		bychat.OrdersMu.Unlock()
	})

	return tgbotapi.Update{CallbackQuery: &tgbotapi.CallbackQuery{
		From:    &tgbotapi.User{ID: userID, UserName: "test_user"},
		Message: &tgbotapi.Message{Chat: &tgbotapi.Chat{ID: userID}},
	}}
}

func TestProcessOrderPreservesUserBlackStatus(t *testing.T) {
	repo, db := newProcessOrderTestRepository(t)
	const userID int64 = 10003

	if _, err := db.Exec(
		`INSERT INTO tg_user_Chat_ID (user_id, user_name, user_black) VALUES (?, ?, ?)`,
		userID,
		"test_user",
		1,
	); err != nil {
		t.Fatalf("не удалось создать заблокированного пользователя: %v", err)
	}

	update := prepareProcessOrderTestData(t, userID)
	purchases := []structs.Purchase{{Item: "Черемша", Quantity: 1, Cost: 450}}

	if err := repo.ProcessOrder(update, purchases); err != nil {
		t.Fatalf("не удалось оформить заказ заблокированному пользователю: %v", err)
	}

	var userBlack int
	if err := db.QueryRow(
			`SELECT user_black FROM tg_user_Chat_ID WHERE user_id = ?`,
			userID,
		).Scan(&userBlack); err != nil {
		t.Fatalf("не удалось прочитать статус пользователя: %v", err)
	}

	if userBlack != 1 {
		t.Fatalf("оформление заказа изменило статус чёрного списка: ожидалось 1, получено %d", userBlack)
	}
}

func TestProcessOrderCreatesSeparateOrders(t *testing.T) {
	repo, db := newProcessOrderTestRepository(t)
	update := prepareProcessOrderTestData(t, 10001)
	purchases := []structs.Purchase{{Item: "Черемша", Quantity: 2, Cost: 450}}

	if err := repo.ProcessOrder(update, purchases); err != nil {
		t.Fatalf("первый заказ не сохранился: %v", err)
	}
	if err := repo.ProcessOrder(update, purchases); err != nil {
		t.Fatalf("второй заказ не сохранился: %v", err)
	}

	var orderCount, itemCount, totalCost int
	if err := db.QueryRow(`SELECT COUNT(*) FROM tg_user_orders_date`).Scan(&orderCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM tg_user_orders`).Scan(&itemCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT SUM(total_cost) FROM tg_user_orders_date`).Scan(&totalCost); err != nil {
		t.Fatal(err)
	}
	if orderCount != 2 || itemCount != 2 || totalCost != 1800 {
		t.Fatalf("неверный результат: заказов=%d, позиций=%d, сумма=%d", orderCount, itemCount, totalCost)
	}
	webOrders, err := repo.GetOrdersWithFilters(structs.QueryParams{
		OrderBy:  "tg_user_orders_date.delivery_date",
		OrderDir: "DESC",
	})
	if err != nil {
		t.Fatalf("не удалось прочитать заказ для веб-интерфейса: %v", err)
	}
	if len(webOrders) == 0 || webOrders[0].Comment != "Тестовый комментарий" {
		t.Fatalf("комментарий не вернулся из запроса заказов: %#v", webOrders)
	}

	lastPurchases, err := repo.GetLastOrderPurchases(10001)
	if err != nil {
		t.Fatalf("не удалось прочитать последний заказ для повтора: %v", err)
	}
	if len(lastPurchases) != 1 || lastPurchases[0].Item != "Черемша" || lastPurchases[0].Quantity != 2 || lastPurchases[0].Cost != 450 {
		t.Fatalf("неверные позиции последнего заказа: %#v", lastPurchases)
	}
}

func TestProcessOrderConcurrentOrders(t *testing.T) {
	repo, db := newProcessOrderTestRepository(t)
	const userID int64 = 10002
	update := prepareProcessOrderTestData(t, userID)
	purchases := []structs.Purchase{{Item: "Острый", Quantity: 1, Cost: 260}}

	const workers = 8
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- repo.ProcessOrder(update, purchases)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("ошибка параллельного оформления заказа: %v", err)
		}
	}

	var orderCount, itemCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM tg_user_orders_date`).Scan(&orderCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM tg_user_orders`).Scan(&itemCount); err != nil {
		t.Fatal(err)
	}
	if orderCount != workers || itemCount != workers {
		t.Fatalf("ожидалось %d отдельных заказов и позиций, получено заказов=%d, позиций=%d", workers, orderCount, itemCount)
	}
}

func TestInitDBMigratesLegacyDatabase(t *testing.T) {
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tempDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tempDir, "sqlite"), 0o755); err != nil {
		t.Fatal(err)
	}
	legacyPath := filepath.Join(tempDir, "sqlite", "data.db")
	legacyDB, err := sql.Open("sqlite3", legacyPath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = legacyDB.Exec(`CREATE TABLE tg_user_orders_date (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER,
		phone_number_id INTEGER,
		full_name_id INTEGER,
		delivery_address_id INTEGER,
		date_time TEXT,
		delivery_date DATE,
		total_cost INTEGER
	)`)
	if err != nil {
		t.Fatal(err)
	}
	if err := legacyDB.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(originalDir) })

	app, err := slogavp.New(slogavp.WithConsole(false), slogavp.WithLogDir(t.TempDir()))
	if err != nil {
		t.Fatalf("не удалось создать тестовый логгер: %v", err)
	}
	defer app.Close()

	db, err := InitDB(app)
	if err != nil {
		t.Fatalf("InitDB не выполнил миграцию старой базы: %v", err)
	}
	defer func() { _ = db.Close() }()

	rows, err := db.Query(`PRAGMA table_info(tg_user_orders_date)`)
	if err != nil {
		t.Fatal(err)
	}
	commentExists := false
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, columnType string
		var defaultValue interface{}
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatal(err)
		}
		if name == "comment" {
			commentExists = true
		}
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}
	if !commentExists {
		t.Fatal("после миграции отсутствует столбец comment")
	}

	var favoritesTableCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='tg_user_favorites'`).Scan(&favoritesTableCount); err != nil {
		t.Fatal(err)
	}
	if favoritesTableCount != 1 {
		t.Fatal("после инициализации отсутствует таблица tg_user_favorites")
	}
}

func TestBackupDatabaseCreatesConsistentCopy(t *testing.T) {
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tempDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tempDir, "sqlite"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(originalDir) })

	app, err := slogavp.New(slogavp.WithConsole(false), slogavp.WithLogDir(t.TempDir()))
	if err != nil {
		t.Fatalf("не удалось создать тестовый логгер: %v", err)
	}
	defer app.Close()

	db, err := sql.Open("sqlite3", "./sqlite/data.db?_busy_timeout=5000&_journal_mode=WAL")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec(`CREATE TABLE sample (id INTEGER PRIMARY KEY, value TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO sample(value) VALUES ('тест')`); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("backup_old.db", []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}

	repo := NewRepository(db, app)
	repo.backupDatabase()

	backup, err := sql.Open("sqlite3", "./backup_old.db")
	if err != nil {
		t.Fatalf("не удалось открыть резервную копию: %v", err)
	}
	defer func() { _ = backup.Close() }()
	var value string
	if err := backup.QueryRow(`SELECT value FROM sample WHERE id = 1`).Scan(&value); err != nil {
		t.Fatalf("в резервной копии нет исходной записи: %v", err)
	}
	if value != "тест" {
		t.Fatalf("неверное значение в резервной копии: %q", value)
	}
}
