package dataSources

import (
	"ProjectTelBotMark/internal/structs"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

func (r *Repository) HandlerProducts(category string, sort string) (structs.PageData, error) {
	// Основной запрос для получения продуктов
	query := `
        SELECT p.id, p.title, p.description, p.price, 
               pc.category as category, p.image, p.packaging 
        FROM products p
        LEFT JOIN products_category pc ON p.category_id = pc.id`

	args := []any{}

	// Фильтрация по категории (если указана)
	if category != "" {
		query += " WHERE pc.category = ?"
		args = append(args, category)
	}

	// Сортировка
	if sort == "price_asc" {
		query += " ORDER BY p.price ASC"
	} else if sort == "price_desc" {
		query += " ORDER BY p.price DESC"
	} else {
		query += " ORDER BY p.id DESC"
	}

	// Получаем продукты
	rows, err := r.db.Query(query, args...)
	if err != nil {
		r.app.Log.Error("Ошибка при запросе продуктов: " + err.Error())
		return structs.PageData{}, err
	}
	defer rows.Close()

	var products []structs.Product
	for rows.Next() {
		var p structs.Product
		err = rows.Scan(&p.ID, &p.Title, &p.Description, &p.Price,
			&p.Category, &p.Image, &p.Packaging)
		if err != nil {
			r.app.Log.Error("Ошибка при сканировании продукта: " + err.Error())
			return structs.PageData{}, err
		}
		products = append(products, p)
	}
	categories, err := r.Categories()

	data := structs.PageData{
		Products:   products,
		Categories: categories,
	}

	return data, nil
}

// HandlerProductsPage возвращает продукты и информацию о пагинации.
func (r *Repository) HandlerProductsPage(page structs.PageProduct) ([]structs.Product, structs.PageProduct, error) {
	ctx := context.Background()
	// Подготовка основного запроса
	productsQuery := `
		SELECT p.id, p.title, p.description, p.price, 
			   pc.category as category, p.image, p.packaging, p.packaging_cal 
		FROM products p
		LEFT JOIN products_category pc ON p.category_id = pc.id
		WHERE 1=1
		AND (p.title LIKE ? OR ? = '')
		AND (pc.category = ? OR ? = '')
		ORDER BY 
			CASE WHEN ? = 'title' THEN p.title END ASC,
			CASE WHEN ? = 'title_desc' THEN p.title END DESC,
			CASE WHEN ? = 'price' THEN p.price END ASC,
			CASE WHEN ? = 'price_desc' THEN p.price END DESC,
			p.id DESC
		LIMIT ? OFFSET ?`

	// Подготавливаем statement для продуктов
	productsStmt, err := r.db.PrepareContext(ctx, productsQuery)
	if err != nil {
		return nil, page, fmt.Errorf("failed to prepare products query: %w", err)
	}
	defer productsStmt.Close()

	// Выполняем запрос продуктов
	rows, err := productsStmt.QueryContext(ctx,
		"%"+page.Search+"%",      // LIKE параметр
		page.Search,              // Проверка на пустой поиск
		page.Category,            // Фильтр категории
		page.Category,            // Проверка на пустую категорию
		page.Sort,                // Сортировка по title
		page.Sort,                // Сортировка по title_desc
		page.Sort,                // Сортировка по price
		page.Sort,                // Сортировка по price_desc
		page.Limit,               // LIMIT
		(page.Page-1)*page.Limit, // OFFSET
	)
	if err != nil {
		return nil, page, fmt.Errorf("failed to query products: %w", err)
	}
	defer rows.Close()

	// Сканируем результаты
	var products []structs.Product
	for rows.Next() {
		var p structs.Product
		if err := rows.Scan(
			&p.ID,
			&p.Title,
			&p.Description,
			&p.Price,
			&p.Category,
			&p.Image,
			&p.Packaging,
			&p.PackagingCal,
		); err != nil {
			return nil, page, fmt.Errorf("failed to scan product: %w", err)
		}
		products = append(products, p)
	}
	if err = rows.Err(); err != nil {
		return nil, page, fmt.Errorf("rows iteration error: %w", err)
	}

	// Подготовка запроса для подсчета общего количества
	countQuery := `
		SELECT COUNT(*)
		FROM products p
		LEFT JOIN products_category pc ON p.category_id = pc.id
		WHERE 1=1
		AND (p.title LIKE ? OR ? = '')
		AND (pc.category = ? OR ? = '')`

	// Подготавливаем statement для подсчета
	countStmt, err := r.db.PrepareContext(ctx, countQuery)
	if err != nil {
		return nil, page, fmt.Errorf("failed to prepare count query: %w", err)
	}
	defer countStmt.Close()

	// Выполняем запрос подсчета
	err = countStmt.QueryRowContext(ctx,
		"%"+page.Search+"%",
		page.Search,
		page.Category,
		page.Category,
	).Scan(&page.Total)
	if err != nil {
		return nil, page, fmt.Errorf("failed to count products: %w", err)
	}

	// Рассчитываем общее количество страниц
	page.TotalPages = (page.Total + page.Limit - 1) / page.Limit

	return products, page, nil
}

// AddProduct добавляем продукты.
func (r *Repository) AddProduct(product structs.Product) error {
	if strings.TrimSpace(product.Title) == "" {
		return errors.New("product title cannot be empty")
	}

	ctx := context.Background()
	// Начинаем транзакцию
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		r.app.Log.Errorf("Failed to begin transaction: %v", err)
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// 1. Проверяем и создаем категорию при необходимости
	var categoryID int64
	err = tx.QueryRowContext(ctx,
		`SELECT id FROM products_category WHERE category = ?`,
		product.Category).Scan(&categoryID)

	if err != nil {
		if err == sql.ErrNoRows {
			// Категории нет - создаем новую
			res, err := tx.ExecContext(ctx,
				`INSERT INTO products_category(category) VALUES(?)`,
				product.Category)
			if err != nil {
				r.app.Log.Errorf("Failed to create new category: %v", err)
				return fmt.Errorf("failed to create new category: %w", err)
			}

			categoryID, err = res.LastInsertId()
			if err != nil {
				r.app.Log.Errorf("Failed to get new category ID: %v", err)
				return fmt.Errorf("failed to get new category ID: %w", err)
			}
		} else {
			r.app.Log.Errorf("Failed to query category: %v", err)
			return fmt.Errorf("failed to query category: %w", err)
		}
	}

	// 2. Добавляем продукт
	stmt, err := tx.PrepareContext(ctx, `
        INSERT INTO products(
            title, 
            description, 
            price, 
            category_id, 
            image, 
            packaging, 
            packaging_cal
        ) VALUES(?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		r.app.Log.Errorf("Failed to prepare product insert: %v", err)
		return fmt.Errorf("failed to prepare product insert: %w", err)
	}
	defer stmt.Close()

	res, err := stmt.ExecContext(ctx,
		product.Title,
		product.Description,
		product.Price,
		categoryID,
		product.Image,
		product.Packaging,
		product.PackagingCal,
	)
	if err != nil {
		r.app.Log.Errorf("Failed to insert product: %v", err)
		return fmt.Errorf("failed to insert product: %w", err)
	}

	// Получаем ID созданного продукта
	_, err = res.LastInsertId()
	if err != nil {
		r.app.Log.Errorf("Failed to get product ID: %v", err)
		return fmt.Errorf("failed to get product ID: %w", err)
	}

	// Фиксируем транзакцию
	if err = tx.Commit(); err != nil {
		r.app.Log.Errorf("Failed to commit transaction: %v", err)
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// EditProduct редактируем продукты.
func (r *Repository) EditProduct(productID int) (structs.Product, error) {
	ctx := context.Background()

	if productID <= 0 {
		r.app.Log.Error("invalid product ID")
		return structs.Product{}, errors.New("invalid product ID")
	}

	// Подготавливаем запрос с JOIN для получения названия категории
	stmt, err := r.db.PrepareContext(ctx, `
        SELECT 
            p.id, 
            p.title, 
            p.description, 
            p.price, 
            pc.category as category, 
            p.image, 
            p.packaging,
            p.packaging_cal
        FROM products p
        LEFT JOIN products_category pc ON p.category_id = pc.id
        WHERE p.id = ?`)
	if err != nil {
		r.app.Log.Errorf("Failed to prepare product select: %v", err)
		return structs.Product{}, fmt.Errorf("failed to prepare query: %w", err)
	}
	defer stmt.Close()

	// Выполняем запрос
	row := stmt.QueryRowContext(ctx, productID)

	// Сканируем результат
	var p structs.Product
	err = row.Scan(
		&p.ID,
		&p.Title,
		&p.Description,
		&p.Price,
		&p.Category,
		&p.Image,
		&p.Packaging,
		&p.PackagingCal,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			r.app.Log.Warnf("Product not found with ID: %d", productID)
			return structs.Product{}, fmt.Errorf("product with ID %d not found", productID)
		}
		r.app.Log.Errorf("Failed to scan product: %v", err)
		return structs.Product{}, fmt.Errorf("failed to scan product: %w", err)
	}

	return p, nil
}

func (r *Repository) UpdateProduct(p structs.Product) error {
	// Валидация входных данных
	if p.ID <= 0 {
		return errors.New("invalid product ID")
	}
	if strings.TrimSpace(p.Title) == "" {
		return errors.New("product title cannot be empty")
	}

	ctx := context.Background()

	// Начинаем транзакцию
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		r.app.Log.Errorf("Failed to begin transaction: %v", err)
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// 1. Получаем текущую категорию продукта
	var oldCategoryID int64
	err = tx.QueryRowContext(ctx,
		"SELECT category_id FROM products WHERE id = ?",
		p.ID).Scan(&oldCategoryID)
	if err != nil {
		r.app.Log.Errorf("Failed to get current product category: %v", err)
		return fmt.Errorf("failed to get current product category: %w", err)
	}

	// 2. Проверяем и создаем новую категорию при необходимости
	var newCategoryID int64
	err = tx.QueryRowContext(ctx,
		"SELECT id FROM products_category WHERE category = ?",
		p.Category).Scan(&newCategoryID)

	if err != nil {
		if err == sql.ErrNoRows {
			// Категории нет - создаем новую
			res, err := tx.ExecContext(ctx,
				"INSERT INTO products_category(category) VALUES(?)",
				p.Category)
			if err != nil {
				r.app.Log.Errorf("Failed to create new category: %v", err)
				return fmt.Errorf("failed to create new category: %w", err)
			}

			newCategoryID, err = res.LastInsertId()
			if err != nil {
				r.app.Log.Errorf("Failed to get new category ID: %v", err)
				return fmt.Errorf("failed to get new category ID: %w", err)
			}
		} else {
			r.app.Log.Errorf("Failed to query category: %v", err)
			return fmt.Errorf("failed to query category: %w", err)
		}
	}

	// 3. Обновляем продукт
	stmt, err := tx.PrepareContext(ctx, `
        UPDATE products SET 
            title = ?, 
            description = ?, 
            price = ?, 
            category_id = ?, 
            image = ?, 
            packaging = ?, 
            packaging_cal = ? 
        WHERE id = ?`)
	if err != nil {
		r.app.Log.Errorf("Failed to prepare update statement: %v", err)
		return fmt.Errorf("failed to prepare update statement: %w", err)
	}
	defer stmt.Close()

	res, err := stmt.ExecContext(ctx,
		p.Title,
		p.Description,
		p.Price,
		newCategoryID,
		p.Image,
		p.Packaging,
		p.PackagingCal,
		p.ID,
	)
	if err != nil {
		r.app.Log.Errorf("Failed to update product: %v", err)
		return fmt.Errorf("failed to update product: %w", err)
	}

	// Проверяем, была ли обновлена хотя бы одна строка
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		r.app.Log.Errorf("Failed to check rows affected: %v", err)
		return fmt.Errorf("failed to check rows affected: %w", err)
	}

	if rowsAffected == 0 {
		r.app.Log.Warnf("No rows affected when updating product ID: %d", p.ID)
		return fmt.Errorf("product with ID %d not found", p.ID)
	}

	// 4. Проверяем, была ли изменена категория
	if oldCategoryID != newCategoryID {
		// 5. Проверяем, используется ли старая категория другими продуктами
		var categoryInUse int
		err = tx.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM products WHERE category_id = ?",
			oldCategoryID).Scan(&categoryInUse)

		if err != nil {
			r.app.Log.Errorf("Failed to check category usage: %v", err)
			return fmt.Errorf("failed to check category usage: %w", err)
		}

		// 6. Если категория больше не используется, удаляем ее
		if categoryInUse == 0 {
			_, err = tx.ExecContext(ctx,
				"DELETE FROM products_category WHERE id = ?",
				oldCategoryID)
			if err != nil {
				r.app.Log.Errorf("Failed to delete unused category: %v", err)
				return fmt.Errorf("failed to delete unused category: %w", err)
			}
			r.app.Log.Infof("Deleted unused category with ID: %d", oldCategoryID)
		}
	}

	// Фиксируем транзакцию
	if err = tx.Commit(); err != nil {
		r.app.Log.Errorf("Failed to commit transaction: %v", err)
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (r *Repository) DeleteProduct(productID int) error {
	if productID <= 0 {
		r.app.Log.Error("invalid product ID")
		return errors.New("invalid product ID")
	}

	ctx := context.Background()
	// Начинаем транзакцию
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		r.app.Log.Errorf("Failed to begin transaction: %v", err)
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// 1. Сначала получаем category_id удаляемого продукта
	var categoryID int
	err = tx.QueryRowContext(ctx,
		"SELECT category_id FROM products WHERE id = ?",
		productID).Scan(&categoryID)

	if err != nil {
		if err == sql.ErrNoRows {
			r.app.Log.Warnf("Product with ID %d not found", productID)
			return fmt.Errorf("product with ID %d not found", productID)
		}
		r.app.Log.Errorf("Failed to get product category: %v", err)
		return fmt.Errorf("failed to get product category: %w", err)
	}

	// 2. Удаляем продукт
	res, err := tx.ExecContext(ctx,
		"DELETE FROM products WHERE id = ?",
		productID)
	if err != nil {
		r.app.Log.Errorf("Failed to delete product: %v", err)
		return fmt.Errorf("failed to delete product: %w", err)
	}

	// Проверяем, был ли удален продукт
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		r.app.Log.Errorf("Failed to check rows affected: %v", err)
		return fmt.Errorf("failed to check rows affected: %w", err)
	}

	if rowsAffected == 0 {
		r.app.Log.Warnf("No product deleted with ID: %d", productID)
		return fmt.Errorf("product with ID %d not found", productID)
	}

	// 3. Проверяем, используется ли еще эта категория другими продуктами
	var categoryInUse int
	err = tx.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM products WHERE category_id = ?",
		categoryID).Scan(&categoryInUse)

	if err != nil {
		r.app.Log.Errorf("Failed to check category usage: %v", err)
		return fmt.Errorf("failed to check category usage: %w", err)
	}

	// 4. Если категория больше не используется, удаляем ее
	if categoryInUse == 0 {
		_, err = tx.ExecContext(ctx,
			"DELETE FROM products_category WHERE id = ?",
			categoryID)
		if err != nil {
			r.app.Log.Errorf("Failed to delete unused category: %v", err)
			return fmt.Errorf("failed to delete unused category: %w", err)
		}
		r.app.Log.Infof("Deleted unused category with ID: %d", categoryID)
	}

	// Фиксируем транзакцию
	if err = tx.Commit(); err != nil {
		r.app.Log.Errorf("Failed to commit transaction: %v", err)
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (r *Repository) ExportExcel() ([]interface{}, error) {
	var data []interface{}
	// 3. Данные из БД
	rows, err := r.db.Query("SELECT title, price, description, category_id, packaging FROM products")
	if err != nil {
		r.app.Log.Error("Ошибка БД: " + err.Error())
		return nil, err
	}
	defer func(rows *sql.Rows) {
		err = rows.Close()
		if err != nil {
			r.app.Log.Error(err.Error())
		}
	}(rows)

	for rowNum := 2; rows.Next(); rowNum++ {
		var p structs.Product

		if err = rows.Scan(&p.Title, &p.Price, &p.Description, &p.Category, &p.Packaging); err != nil {
			continue
		}

		data = []interface{}{rowNum - 1, p.Title, p.Price, p.Description, p.Category}
	}
	return data, nil
}

func (r *Repository) Categories() ([]string, error) {
	// Получаем список всех категорий из таблицы категорий
	categoryRows, err := r.db.Query("SELECT category FROM products_category ORDER BY category")
	if err != nil {
		r.app.Log.Error("Ошибка при получении категорий: " + err.Error())
		return nil, err
	}
	defer categoryRows.Close()

	var categories []string
	for categoryRows.Next() {
		var cat string
		err = categoryRows.Scan(&cat)
		if err != nil {
			r.app.Log.Error("Ошибка при сканировании категории: " + err.Error())
			return nil, err
		}
		categories = append(categories, cat)
	}
	return categories, nil
}
