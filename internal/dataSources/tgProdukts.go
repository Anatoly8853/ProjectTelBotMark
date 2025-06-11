package dataSources

import (
	"ProjectTelBotMark/internal/structs"
	"context"
	"strings"
)

// GetCategoriesWithProducts основная функция выводит все продукты из всех таблиц.
func (r *Repository) GetCategoriesWithProducts() (map[string]structs.Category, error) {
	ctx := context.Background()
	categories := make(map[string]structs.Category)

	// 1. Получаем все основные категории
	mainCategories, err := r.getAllMainCategories(ctx)
	if err != nil {
		return nil, err
	}

	// 2. Обрабатываем каждую основную категорию
	for mainCatName, mainCat := range mainCategories {
		// Проверяем, есть ли у категории подкатегории
		hasSubcategories, err := r.hasSubcategories(ctx, mainCat.ID)
		if err != nil {
			return nil, err
		}

		if hasSubcategories {
			// Если есть подкатегории - получаем их
			subCats, err := r.getSubCategories(ctx, mainCat.ID)
			if err != nil {
				return nil, err
			}

			// Добавляем кнопки подкатегорий
			var buttons []structs.ButtonData
			for _, subCat := range subCats {
				buttons = append(buttons, structs.ButtonData{
					Text: subCat.CategoryName,
				})
			}

			mainCat.Buttons = buttons
			mainCat.ButtonsM = getDefaultButtonsM()
			categories[mainCatName] = mainCat

			// Добавляем подкатегории в общий список
			for _, subCat := range subCats {
				// Получаем продукты для подкатегории
				products, err := r.getProductsForSubCategory(ctx, mainCat.ID, subCat.ID)
				if err != nil {
					r.app.Log.Error("GetCategoriesWithProducts getProductsForSubCategory", err)
					return nil, err
				}
				for _, subCatBut := range products {
					categories[subCat.CategoryName] = structs.Category{
						CategoryName: subCat.CategoryName,
						CategoryMsg:  subCat.CategoryMsg,
						Photo:        products[0].Photo,
						Buttons:      subCatBut.Buttons,
						ButtonsM:     getDefaultButtonsM(),
					}
				}
			}
		}
	}

	// 3. Добавляем обязательную категорию "Старт"
	categories["Старт"] = createStartCategory(mainCategories)

	return categories, nil
}

// Вспомогательные функции
// getPhotoForCategory Получает фото для категории из первого продукта.
func getPhotoForCategory(products []structs.Category) string {
	if len(products) > 0 {
		return products[0].Photo
	}
	return ""
}

// getDefaultButtonsM Возвращает стандартные кнопки меню.
func getDefaultButtonsM() []structs.ButtonData {
	return []structs.ButtonData{
		{Text: "🔙Назад🔙"},
		{Text: "🔙Мои заказы🔙"},
		{Text: "🛒Корзина🛒"},
	}
}

// createStartCategory Создает стартовую категорию.
func createStartCategory(mainCategories map[string]structs.Category) structs.Category {
	startButtons := []structs.ButtonData{}
	for name := range mainCategories {
		if name != "Старт" {
			startButtons = append(startButtons, structs.ButtonData{
				Text: name,
			})
		}
	}

	return structs.Category{
		CategoryName: "Старт",
		Buttons:      startButtons,
		ButtonsM: []structs.ButtonData{
			{Text: "🔙Мои заказы🔙"},
			{Text: "🛒Корзина🛒"},
		},
	}
}

// getAllMainCategories Получает все основные категории.
func (r *Repository) getAllMainCategories(ctx context.Context) (map[string]structs.Category, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, name FROM categories")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]structs.Category)
	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}

		result[name] = structs.Category{
			ID:           id,
			CategoryName: name,
			//Description:  displayName,
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
		return false, err
	}
	return count > 0, nil
}

// getSubCategories Получает подкатегории для основной категории.
func (r *Repository) getSubCategories(ctx context.Context, mainCatID int) ([]structs.Category, error) {
	rows, err := r.db.QueryContext(ctx, `
        SELECT c2.id, c2.name
        FROM categories_2 c2
        JOIN products_categories pc ON pc.categories_2_id = c2.id
        WHERE pc.category_id = ?
        GROUP BY c2.id, c2.name
        ORDER BY c2.name`, mainCatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subCats []structs.Category
	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}

		subCats = append(subCats, structs.Category{
			ID:           id,
			CategoryName: name,
			//Description:  description,
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
		return nil, err
	}
	defer rows.Close()
	var categories []structs.Category
	var products []structs.ButtonData
	for rows.Next() {
		var text, callback, photoPath string
		if err := rows.Scan(&text, &callback, &photoPath); err != nil {
			return nil, err
		}

		//displayText := fmt.Sprintf("⚖%s \uD83D\uDCB5 %d рублей", weight, price)
		products = append(products, structs.ButtonData{
			Text:     text,
			Callback: callback,
			//Photo:    photoPath,
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
        SELECT p.text, p.callback, ph.name AS photo_path
        FROM products_categories p
        JOIN photo ph ON p.photo_id = ph.id
        WHERE p.category_id = ? 
        AND p.categories_2_id = ?`,
		mainCatID, subCatID)
	//mainCatID, mainCatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []structs.Category
	var products []structs.ButtonData
	for rows.Next() {
		var text, callback, photoPath string
		if err := rows.Scan(&text, &callback, &photoPath); err != nil {
			return nil, err
		}

		products = append(products, structs.ButtonData{
			Text:     text,
			Callback: callback,
		})

		categories = append(categories, structs.Category{
			Photo:   photoPath,
			Buttons: products,
		})
	}

	return categories, nil
}

// NormalizeWords NormalizeWord Первое слово как есть все остальные в нижнем регистре, удаляет повторы слов (регистр не учитывается).
func NormalizeWords(input string) string {
	words := strings.Fields(input)
	seen := make(map[string]bool)
	var result []string

	for i, word := range words {
		normalized := word
		lower := strings.ToLower(word)

		if seen[lower] {
			continue // пропустить дубликат
		}
		seen[lower] = true

		if i == 0 {
			result = append(result, normalized)
		} else {
			result = append(result, lower)
		}
	}
	return strings.Join(result, " ")
}
