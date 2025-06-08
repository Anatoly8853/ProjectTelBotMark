package dataSources

import (
	"ProjectTelBotMark/internal/structs"
	"context"
	"fmt"
)

// GetProductsByCategory получаем
func (r *Repository) GetProductsByCategory(category string) (map[string]structs.Category, error) {
	ctx := context.Background()
	categories := make(map[string]structs.Category)

	rows, err := r.db.QueryContext(ctx, `
        SELECT 
            p.id, 
            p.title, 
            p.description, 
            p.price, 
            p.image, 
            p.packaging, 
            p.packaging_cal,
            pc.category AS category_name 
        FROM products p
        JOIN products_category pc ON p.category_id = pc.id
        WHERE pc.category = ?
        ORDER BY p.title`, category)
	if err != nil {
		return nil, fmt.Errorf("failed to query products: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var p structs.Product
		if err := rows.Scan(
			&p.ID,
			&p.Title,
			&p.Description,
			&p.Price,
			&p.Image,
			&p.Packaging,
			&p.PackagingCal,
			&p.Category,
		); err != nil {
			r.app.Log.Errorf("failed to scan product: %w", err)
			return nil, fmt.Errorf("failed to scan product: %w", err)
		}

		r.app.Log.Debugf("product: %#v", p)

		// Создаем или обновляем категорию с нужной структурой
		categories[p.Category] = structs.Category{
			CategoryName: p.Title,                    // Как вы указали
			CategoryMsg:  p.Category + " " + p.Title, // Как вы указали
			Photo:        p.Image,                    // Как вы указали
			Buttons: []structs.ButtonData{ // Как вы указали
				{Text: p.PackagingCal},
			},
		}
	}

	if err := rows.Err(); err != nil {
		r.app.Log.Errorf("rows iteration error: %w", err)
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	r.app.Log.Infof("Successfully loaded category: %s with %d products", category, len(categories))
	return categories, nil
}

// GetCategoriesWithProducts возвращает категории с товарами, готовые для отображения в Telegram
func (r *Repository) GetCategoriesWithProducts() (map[string]structs.Category, error) {
	ctx := context.Background()
	categories := make(map[string]structs.Category)

	// 1. Сначала получаем только категории (без продуктов)
	catRows, err := r.db.QueryContext(ctx, "SELECT category FROM products_category ORDER BY category")
	if err != nil {
		return nil, fmt.Errorf("failed to query categories: %w", err)
	}
	defer catRows.Close()

	for catRows.Next() {
		var category string
		if err := catRows.Scan(&category); err != nil {
			return nil, fmt.Errorf("failed to scan category: %w", err)
		}

		// Создаем пустую категорию
		categories[category] = structs.Category{
			CategoryName: category,
			CategoryMsg:  "",
			Photo:        "",
			Buttons:      []structs.ButtonData{},
			ButtonsM:     []structs.ButtonData{},
		}
	}

	// 2. Теперь получаем все продукты и заполняем категории
	productRows, err := r.db.QueryContext(ctx, `
        SELECT 
            p.title,
            p.image,
            p.packaging_cal,
            pc.category
        FROM products p
        JOIN products_category pc ON p.category_id = pc.id
        ORDER BY pc.category, p.title`)
	if err != nil {
		return nil, fmt.Errorf("failed to query products: %w", err)
	}
	defer productRows.Close()

	for productRows.Next() {
		var title, image, packagingCal, category string
		if err := productRows.Scan(&title, &image, &packagingCal, &category); err != nil {
			return nil, fmt.Errorf("failed to scan product: %w", err)
		}

		// Если категория существует, обновляем ее
		if cat, exists := categories[category]; exists {
			cat.CategoryName = title
			cat.CategoryMsg = category + " " + title
			cat.Photo = image
			cat.Buttons = append(cat.Buttons, structs.ButtonData{
				Text: packagingCal,
			})
			categories[category] = cat
		}
	}

	// 3. Добавляем обязательную категорию "Старт"
	var startButtons []structs.ButtonData
	for categoryName := range categories {
		if categoryName != "Старт" {
			startButtons = append(startButtons, structs.ButtonData{
				Text: categoryName,
			})
		}
	}

	categories["Старт"] = structs.Category{
		CategoryName: "Старт",
		Buttons:      startButtons,
	}

	return categories, nil
}
