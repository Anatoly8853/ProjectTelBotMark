package dataSources

import (
	"ProjectTelBotMark/internal/structs"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/text/cases"
	. "golang.org/x/text/language"
)

type JSONCategories map[string]structs.Category

// GetCategoriesWithProducts основная функция выводит все продукты из всех таблиц.
func (r *Repository) GetCategoriesWithProducts() (map[string]structs.Category, error) {
	ctx := context.Background()
	jsonResult := make(JSONCategories)

	mainCategories, err := r.getAllMainCategories(ctx)
	if err != nil {
		r.app.Log.Error("getAllMainCategories", err.Error())
		return nil, err
	}

	for mainCatName, mainCat := range mainCategories {
		hasSubcategories, err := r.hasSubcategories(ctx, mainCat.ID)
		if err != nil {
			r.app.Log.Error("hasSubcategories", err.Error())
			return nil, err
		}

		if hasSubcategories {
			subCats, err := r.getSubCategories(ctx, mainCat.ID)
			if err != nil {
				r.app.Log.Error("getSubCategories", err.Error())
				return nil, err
			}

			// Кнопки подкатегорий
			var buttons []structs.ButtonData
			for _, subCat := range subCats {
				buttons = append(buttons, structs.ButtonData{
					Text:     subCat.CategoryName,
					Callback: fmt.Sprintf("cat_%q", StripEmojis(subCat.CategoryName)),
				})
			}

			jsonResult[mainCatName] = structs.Category{
				CategoryName: mainCat.CategoryName,
				Buttons:      buttons,
				ButtonsM:     getDefaultButtonsM(),
			}

			// Подкатегории с продуктами
			for _, subCat := range subCats {
				products, err := r.getProductsForSubCategory(ctx, mainCat.ID, subCat.ID)
				if err != nil {
					r.app.Log.Error("getProductsForSubCategory", err)
					return nil, err
				}

				for _, product := range products {
					var productButtons []structs.ButtonData
					for _, b := range product.Buttons {
						productButtons = append(productButtons, b)
					}

					jsonResult[subCat.CategoryName] = structs.Category{
						CategoryName: subCat.CategoryName,
						CategoryMsg:  subCat.CategoryMsg,
						Photo:        product.Photo,
						Buttons:      productButtons,
						ButtonsM:     getDefaultButtonsM(),
					}
				}
			}
		}
	}

	// 🔄 Постраничная разбивка "Старт" + "Стр. 2", "Стр. 3", ...
	paginated := paginateCategories(mainCategories)
	for key, val := range paginated {
		jsonResult[key] = val
	}

	// 📦 JSON отладка — можно закомментировать
	_, err = json.MarshalIndent(jsonResult, "", "  ")
	if err != nil {
		return nil, err
	}

	// 🧩 Преобразуем в финальную структуру
	final := make(map[string]structs.Category)
	for key, val := range jsonResult {
		final[key] = structs.Category{
			CategoryName: val.CategoryName,
			CategoryMsg:  val.CategoryMsg,
			Photo:        val.Photo,
			Buttons:      val.Buttons,
			ButtonsM:     val.ButtonsM,
		}
	}

	return final, nil
}

// paginateCategories создаем пагинацию для основных категорий продуктов.
func paginateCategories(mainCategories map[string]structs.Category) map[string]structs.Category {
	const maxButtonsPerPage = 6

	var pages []structs.Category
	var currentButtons []structs.ButtonData

	// Сохраняем имена категорий-кнопок
	for _, cat := range mainCategories {
		currentButtons = append(currentButtons, structs.ButtonData{
			Text:     cat.CategoryName,
			Callback: fmt.Sprintf("cat_%s", StripEmojis(cat.CategoryName)),
		})
	}

	// Разбиваем на страницы по 6 штук
	for i := 0; i < len(currentButtons); i += maxButtonsPerPage {
		end := i + maxButtonsPerPage
		if end > len(currentButtons) {
			end = len(currentButtons)
		}
		page := structs.Category{
			CategoryName: fmt.Sprintf("Стр %d", len(pages)+1),
			Buttons:      currentButtons[i:end],
		}
		pages = append(pages, page)
	}

	// Устанавливаем ButtonsM
	for i := range pages {
		var buttonsM []structs.ButtonData

		// Кнопка "назад"
		if i > 0 {
			buttonsM = append(buttonsM, structs.ButtonData{
				Text:     fmt.Sprintf("🔙Стр %d🔙", i),
				Callback: fmt.Sprintf("cat_Стр %d", i),
			})
		}

		// Кнопка "🛒Корзина"
		buttonsM = append(buttonsM, structs.ButtonData{
			Text:     "🛒Корзина🛒",
			Callback: "cat_Корзина",
		})

		// Кнопка "Мои заказы"
		buttonsM = append(buttonsM, structs.ButtonData{
			Text:     "🔙Мои заказы🔙",
			Callback: "cat_Мои заказы",
		})

		// Кнопка "вперёд"
		if i < len(pages)-1 {
			buttonsM = append(buttonsM, structs.ButtonData{
				Text:     fmt.Sprintf("Стр %d", i+2),
				Callback: fmt.Sprintf("cat_Стр %d", i+2),
			})
		}

		pages[i].ButtonsM = buttonsM
	}

	// Собираем в карту
	result := make(map[string]structs.Category)

	// Категория "Старт" — первая страница
	result["Старт"] = pages[0]
	start := result["Старт"]
	start.CategoryName = "Старт"
	result["Старт"] = start

	// Остальные страницы
	for i := 1; i < len(pages); i++ {
		pageName := fmt.Sprintf("Стр %d", i+1)
		pages[i].CategoryName = pageName
		result[pageName] = pages[i]
	}

	return result
}

// getDefaultButtonsM Возвращает стандартные кнопки меню.
// GetDefaultButtonsM возвращает стандартные кнопки меню для внешних пакетов.
func (r *Repository) GetDefaultButtonsM() []structs.ButtonData {
	return getDefaultButtonsM()
}

func getDefaultButtonsM() []structs.ButtonData {
	return []structs.ButtonData{
		{Text: "🔙Назад🔙", Callback: "cat_Назад"},
		{Text: "🔙Мои заказы🔙", Callback: "cat_Мои заказы"},
		{Text: "🛒Корзина🛒", Callback: "cat_Корзина"},
	}
}

// getAllMainCategories Получает все основные категории.
func (r *Repository) getAllMainCategories(ctx context.Context) (map[string]structs.Category, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, name, display_name FROM categories")
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			r.app.Log.Error("getAllMainCategories", err)
			return
		}
	}(rows)

	result := make(map[string]structs.Category)
	for rows.Next() {
		var id int
		var name, displayName string
		if err := rows.Scan(&id, &name, &displayName); err != nil {
			r.app.Log.Error("getAllMainCategories", err)
			return nil, err
		}

		result[name] = structs.Category{
			ID:           id,
			CategoryName: displayName,
		}
	}
	return result, nil
}

// hasSubcategories Проверяет, есть ли подкатегории у основной категории.
func (r *Repository) hasSubcategories(ctx context.Context, mainCatID int) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `
        SELECT COUNT(*) 
        FROM products_categories 
        WHERE category_id = ? 
        AND categories_2_id IS NOT NULL`,
		mainCatID).Scan(&count)
	if err != nil {
		r.app.Log.Error("hasSubcategories", err.Error())
		return false, err
	}
	return count > 0, nil
}

// getSubCategories Получает подкатегории для основной категории.
func (r *Repository) getSubCategories(ctx context.Context, mainCatID int) ([]structs.Category, error) {
	rows, err := r.db.QueryContext(ctx, `
        SELECT c2.id, c2.name, c2.display_name
        FROM categories_2 c2
        JOIN products_categories pc ON pc.categories_2_id = c2.id
        WHERE pc.category_id = ?
        GROUP BY c2.id, c2.name
        ORDER BY c2.name`, mainCatID)
	if err != nil {
		r.app.Log.Error("getSubcategories", err.Error())
		return nil, err
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			r.app.Log.Error("getSubCategories", err)
			return
		}
	}(rows)

	var subCats []structs.Category
	for rows.Next() {
		var id int
		var name, displayName string
		if err := rows.Scan(&id, &name, &displayName); err != nil {
			r.app.Log.Error("getSubcategories", err.Error())
			return nil, err
		}

		subCats = append(subCats, structs.Category{
			ID:           id,
			CategoryName: displayName,
		})
	}

	return subCats, nil
}

// getProductsForMainCategory Получает продукты для основной категории (без подкатегорий).
func (r *Repository) getProductsForMainCategory(ctx context.Context, mainCatID int) ([]structs.Category, error) {
	rows, err := r.db.QueryContext(ctx, `
        SELECT p.weight_kg, p.price_rub, p.callback, ph.name AS photo_path
        FROM products_categories p
        JOIN photo ph ON p.photo_id = ph.id
        WHERE p.category_id = ? 
        AND p.categories_2_id IS NULL`,
		mainCatID)
	if err != nil {
		r.app.Log.Error("getProductsForMainCategory", err.Error())
		return nil, err
	}

	defer func(rows *sql.Rows) {
		err = rows.Close()
		if err != nil {
			r.app.Log.Error("getProductsForMainCategory", err)
			return
		}
	}(rows)

	var categories []structs.Category
	var products []structs.ButtonData
	for rows.Next() {
		var productID int
		var text, callback, photoPath string
		if err = rows.Scan(&productID, &text, &callback, &photoPath); err != nil {
			r.app.Log.Error("getProductsForMainCategory", err.Error())
			return nil, err
		}

		//displayText := fmt.Sprintf("⚖%s \uD83D\uDCB5 %d рублей", weight, price)
		products = append(products, structs.ButtonData{
			Text:             text,
			Callback:         callback, // существующий callback товара сохраняется без изменений
			FavoriteCallback: fmt.Sprintf("fav_toggle_%d", productID),
		})

		categories = append(categories, structs.Category{
			Photo:   photoPath,
			Buttons: products,
		})
	}
	return categories, nil
}

// getProductsForSubCategory Получает продукты для подкатегории.
func (r *Repository) getProductsForSubCategory(ctx context.Context, mainCatID, subCatID int) ([]structs.Category, error) {
	rows, err := r.db.QueryContext(ctx, `
        SELECT p.id, p.text, p.callback, ph.name AS photo_path
        FROM products_categories p
        JOIN photo ph ON p.photo_id = ph.id
        WHERE p.category_id = ? 
        AND p.categories_2_id = ?
        ORDER BY p.id ASC`,
		mainCatID, subCatID)

	if err != nil {
		r.app.Log.Error("getProductsForSubCategory", err.Error())
		return nil, err
	}

	defer func(rows *sql.Rows) {
		err = rows.Close()
		if err != nil {
			r.app.Log.Error("getProductsForSubCategory", err)
			return
		}
	}(rows)

	var categories []structs.Category
	var products []structs.ButtonData
	for rows.Next() {
		var productID int
		var text, callback, photoPath string
		if err = rows.Scan(&productID, &text, &callback, &photoPath); err != nil {
			r.app.Log.Error("getProductsForSubCategory", err.Error())
			return nil, err
		}

		products = append(products, structs.ButtonData{
			Text:             text,
			Callback:         callback,
			FavoriteCallback: fmt.Sprintf("fav_toggle_%d", productID),
		})

		categories = append(categories, structs.Category{
			Photo:   photoPath,
			Buttons: products,
		})
	}
	if err := rows.Err(); err != nil {
		r.app.Log.Errorf("getProductsForSubCategory: ошибка обхода строк: %v", err)
		return nil, err
	}

	return categories, nil
}

// NormalizeWords NormalizeWord Первое слово как есть все остальные в нижнем регистре, удаляет повторы слов (регистр не учитывается).
func NormalizeWords(input string) string {
	words := strings.Fields(input)
	if len(words) == 0 {
		return ""
	}

	// Определяем язык
	lang := detectLanguage(words[0])
	titleCaser := cases.Title(lang)

	// Обрабатываем первое слово
	first := strings.ToLower(words[0])
	first = titleCaser.String(first)
	result := []string{first}

	seen := make(map[string]bool)
	seen[strings.ToLower(first)] = true

	// Обработка остальных слов
	for _, word := range words[1:] {
		lower := strings.ToLower(word)
		if seen[lower] {
			continue
		}
		seen[lower] = true
		result = append(result, lower)
	}

	return strings.Join(result, " ")
}

// detectLanguage определяет язык по первому слову
func detectLanguage(word string) Tag {
	for _, r := range word {
		if unicode.Is(unicode.Cyrillic, r) {
			return Russian
		}
	}
	return English
}
