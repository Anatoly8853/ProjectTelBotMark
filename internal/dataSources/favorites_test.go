package dataSources

import (
	"testing"
)

func TestToggleFavoriteAddsAndRemovesProduct(t *testing.T) {
	repo, db := newProcessOrderTestRepository(t)
	statements := []string{
		`CREATE TABLE categories_2 (id INTEGER PRIMARY KEY, name TEXT, description TEXT)`,
		`CREATE TABLE photo (id INTEGER PRIMARY KEY, name TEXT)`,
		`CREATE TABLE products_categories (id INTEGER PRIMARY KEY, categories_2_id INTEGER, photo_id INTEGER, weight_kg TEXT, price_rub INTEGER, text TEXT, callback TEXT)`,
		`CREATE TABLE tg_user_favorites (user_id INTEGER NOT NULL, product_id INTEGER NOT NULL, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, PRIMARY KEY(user_id, product_id), FOREIGN KEY(product_id) REFERENCES products_categories(id) ON DELETE CASCADE)`,
		`INSERT INTO categories_2(id, name, description) VALUES (1, 'Черемша', 'Тестовый товар')`,
		`INSERT INTO photo(id, name) VALUES (1, 'uploads/test.jpg')`,
		`INSERT INTO products_categories(id, categories_2_id, photo_id, weight_kg, price_rub, text, callback) VALUES (11, 1, 1, '1 кг', 450, 'Черемша 1 кг', 'add_Черемша 1 кг_1_450')`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("не удалось подготовить тест избранного: %v", err)
		}
	}

	added, err := repo.ToggleFavorite(10003, 11)
	if err != nil {
		t.Fatalf("не удалось добавить товар в избранное: %v", err)
	}
	if !added {
		t.Fatal("ожидалось добавление товара в избранное")
	}

	products, err := repo.GetFavoriteProducts(10003)
	if err != nil {
		t.Fatalf("не удалось получить избранное: %v", err)
	}
	if len(products) != 1 || products[0].ID != 11 || products[0].Callback != "add_Черемша 1 кг_1_450" {
		t.Fatalf("неверные данные избранного: %#v", products)
	}

	added, err = repo.ToggleFavorite(10003, 11)
	if err != nil {
		t.Fatalf("не удалось удалить товар из избранного: %v", err)
	}
	if added {
		t.Fatal("повторное нажатие должно удалить товар из избранного")
	}
	products, err = repo.GetFavoriteProducts(10003)
	if err != nil {
		t.Fatal(err)
	}
	if len(products) != 0 {
		t.Fatalf("после удаления избранное должно быть пустым: %#v", products)
	}
}
