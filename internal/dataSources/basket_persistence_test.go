package dataSources

import (
	"ProjectTelBotMark/internal/structs"
	"database/sql"
	"reflect"
	"testing"
)

func newBasketTestRepository(t *testing.T) *Repository {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("не удалось открыть тестовую SQLite: %v", err)
	}
	if _, err = db.Exec(`
		CREATE TABLE tg_user_baskets (
			user_id INTEGER NOT NULL,
			item_id INTEGER NOT NULL,
			item_name TEXT NOT NULL,
			quantity INTEGER NOT NULL CHECK(quantity > 0),
			cost INTEGER NOT NULL CHECK(cost > 0),
			position INTEGER NOT NULL,
			PRIMARY KEY (user_id, item_id)
		)
	`); err != nil {
		_ = db.Close()
		t.Fatalf("не удалось создать таблицу корзин: %v", err)
	}

	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("не удалось закрыть тестовую SQLite: %v", err)
		}
	})
	return &Repository{db: db}
}

func TestSaveAndRestoreBasket(t *testing.T) {
	repo := newBasketTestRepository(t)
	const userID int64 = 123456

	want := []structs.Purchase{
		{ID: 11, Item: "Черемша 1 кг", Quantity: 2, Cost: 450},
		{ID: 12, Item: "Острый 1 кг", Quantity: 1, Cost: 260},
	}
	if err := repo.SaveBasket(userID, want); err != nil {
		t.Fatalf("не удалось сохранить корзину: %v", err)
	}

	got, err := repo.GetAllBaskets()
	if err != nil {
		t.Fatalf("не удалось загрузить корзины: %v", err)
	}
	if !reflect.DeepEqual(got[userID], want) {
		t.Fatalf("восстановленная корзина не совпадает с сохранённой: получено %#v, ожидалось %#v", got[userID], want)
	}
}

func TestSaveEmptyBasketDeletesSavedItems(t *testing.T) {
	repo := newBasketTestRepository(t)
	const userID int64 = 123456

	purchases := []structs.Purchase{{ID: 11, Item: "Черемша 1 кг", Quantity: 1, Cost: 450}}
	if err := repo.SaveBasket(userID, purchases); err != nil {
		t.Fatalf("не удалось сохранить корзину: %v", err)
	}
	if err := repo.SaveBasket(userID, nil); err != nil {
		t.Fatalf("не удалось очистить сохранённую корзину: %v", err)
	}

	got, err := repo.GetAllBaskets()
	if err != nil {
		t.Fatalf("не удалось загрузить корзины: %v", err)
	}
	if len(got[userID]) != 0 {
		t.Fatalf("после очистки корзина должна быть пустой, получено %#v", got[userID])
	}
}

func TestDeleteBasket(t *testing.T) {
	repo := newBasketTestRepository(t)
	const userID int64 = 123456

	if err := repo.SaveBasket(userID, []structs.Purchase{{ID: 11, Item: "Черемша 1 кг", Quantity: 1, Cost: 450}}); err != nil {
		t.Fatalf("не удалось сохранить корзину: %v", err)
	}
	if err := repo.DeleteBasket(userID); err != nil {
		t.Fatalf("не удалось удалить корзину: %v", err)
	}
	got, err := repo.GetAllBaskets()
	if err != nil {
		t.Fatalf("не удалось загрузить корзины: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("после удаления не должно остаться корзин: %#v", got)
	}
}
