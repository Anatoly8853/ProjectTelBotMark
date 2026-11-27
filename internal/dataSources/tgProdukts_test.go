package dataSources

import (
	"database/sql"
	"testing"

	"ProjectTelBotMark/internal/structs"
)

func TestPaginateCategoriesEmptyCatalog(t *testing.T) {
	got := paginateCategories(map[string]structs.Category{})

	start, ok := got["Старт"]
	if !ok {
		t.Fatal("для пустого каталога должна создаваться стартовая страница")
	}
	if start.CategoryName != "Старт" {
		t.Fatalf("имя стартовой страницы = %q, ожидалось %q", start.CategoryName, "Старт")
	}
	if len(start.Buttons) != 0 {
		t.Fatalf("у пустого каталога не должно быть товарных кнопок: %#v", start.Buttons)
	}
	if len(start.ButtonsM) == 0 {
		t.Fatal("на стартовой странице должны оставаться служебные кнопки")
	}
}

func TestSubcategoryBackButtonReturnsToParentCategory(t *testing.T) {
	buttons := getSubcategoryButtonsM("Капуста")
	if len(buttons) == 0 {
		t.Fatal("для подкатегории должны быть созданы кнопки навигации")
	}
	if buttons[0].Text != "🔙Назад🔙" {
		t.Fatalf("текст первой кнопки = %q, ожидалось %q", buttons[0].Text, "🔙Назад🔙")
	}
	if buttons[0].Callback != "cat_Капуста" {
		t.Fatalf("callback кнопки Назад = %q, ожидалось %q", buttons[0].Callback, "cat_Капуста")
	}
}


func TestGetCategoriesWithProductsSeparatesDirectProductsFromSubcategories(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("открытие тестовой БД: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	const schema = `
		CREATE TABLE categories (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			display_name TEXT NOT NULL
		);
		CREATE TABLE categories_2 (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			display_name TEXT NOT NULL,
			description TEXT
		);
		CREATE TABLE photo (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL
		);
		CREATE TABLE products_categories (
			id INTEGER PRIMARY KEY,
			category_id INTEGER NOT NULL,
			categories_2_id INTEGER,
			photo_id INTEGER NOT NULL,
			text TEXT NOT NULL,
			callback TEXT
		);
		INSERT INTO categories(id, name, display_name) VALUES
			(1, 'Морковь', 'Морковь'),
			(2, 'Овощи', 'Овощи');
		INSERT INTO categories_2(id, name, display_name, description) VALUES
			(1, 'Морковь', 'Морковь', ''),
			(2, 'Капуста', 'Капуста', '');
		INSERT INTO photo(id, name) VALUES
			(1, 'carrot.jpg'),
			(2, 'cabbage.jpg');
		INSERT INTO products_categories(id, category_id, categories_2_id, photo_id, text, callback) VALUES
			(1, 1, 1, 1, 'Морковь 1 кг', 'add_1'),
			(2, 2, 2, 2, 'Капуста 1 кг', 'add_2');
	`;
	if _, err = db.Exec(schema); err != nil {
		t.Fatalf("создание тестовых данных: %v", err)
	}

	repository := NewRepository(db, nil)
	categories, err := repository.GetCategoriesWithProducts()
	if err != nil {
		t.Fatalf("GetCategoriesWithProducts: %v", err)
	}

	carrot, ok := categories["Морковь"]
	if !ok {
		t.Fatal("самостоятельная категория «Морковь» не найдена")
	}
	if len(carrot.ButtonsM) == 0 || carrot.ButtonsM[0].Callback != "cat_Старт" {
		t.Fatalf("кнопка «Назад» самостоятельной категории должна вести на Старт: %+v", carrot.ButtonsM)
	}
	if len(carrot.Buttons) != 1 || carrot.Buttons[0].Callback != "add_1" {
		t.Fatalf("товары самостоятельной категории не загружены: %+v", carrot.Buttons)
	}
	if carrot.Photo != "carrot.jpg" {
		t.Fatalf("фото самостоятельной категории = %q, ожидалось %q", carrot.Photo, "carrot.jpg")
	}

	vegetables, ok := categories["Овощи"]
	if !ok {
		t.Fatal("основная категория «Овощи» не найдена")
	}
	if len(vegetables.Buttons) != 1 || vegetables.Buttons[0].Callback != "cat_Капуста" {
		t.Fatalf("подкатегория основной категории потеряна: %+v", vegetables.Buttons)
	}

	cabbage, ok := categories["Капуста"]
	if !ok {
		t.Fatal("подкатегория «Капуста» не найдена")
	}
	if len(cabbage.ButtonsM) == 0 || cabbage.ButtonsM[0].Callback != "cat_Овощи" {
		t.Fatalf("кнопка «Назад» подкатегории должна вести к родителю: %+v", cabbage.ButtonsM)
	}
}
