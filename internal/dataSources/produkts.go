package dataSources

import (
	"ProjectTelBotMark/internal/structs"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// HandlerProducts Основная функция загружает все данные.
func (r *Repository) HandlerProducts(category string, sort string) (structs.PageData, error) {
	query := `
		SELECT 
			pc.id,
			c.display_name AS category,
			c2.name AS title,
			c2.description,
			ph.name AS image,
			pc.weight_kg AS packaging,
			pc.price_rub AS price
		FROM products_categories pc
		LEFT JOIN categories c ON pc.category_id = c.id
		LEFT JOIN categories_2 c2 ON pc.categories_2_id = c2.id
		LEFT JOIN photo ph ON pc.photo_id = ph.id
	`

	args := []any{}

	// Фильтрация по категории
	if category != "" {
		query += " WHERE c.display_name = ?"
		args = append(args, category)
	}

	// Сортировка
	switch sort {
	case "price_asc":
		query += " ORDER BY pc.price_rub ASC"
	case "price_desc":
		query += " ORDER BY pc.price_rub DESC"
	default:
		query += " ORDER BY pc.id DESC"
	}

	// Выполняем запрос
	rows, err := r.db.Query(query, args...)
	if err != nil {
		r.app.Log.Error("Ошибка при запросе продуктов: " + err.Error())
		return structs.PageData{}, err
	}
	defer rows.Close()

	// Читаем строки
	var products []structs.Product
	for rows.Next() {
		var p structs.Product
		err = rows.Scan(&p.ID, &p.Category, &p.Title, &p.Description, &p.Image, &p.Packaging, &p.Price)
		if err != nil {
			r.app.Log.Error("Ошибка при сканировании продукта: " + err.Error())
			return structs.PageData{}, err
		}
		products = append(products, p)
	}

	// Получаем список категорий (если есть метод)
	categories, err := r.Categories()
	if err != nil {
		r.app.Log.Error("Ошибка при получении категорий: " + err.Error())
		return structs.PageData{}, err
	}

	return structs.PageData{
		Products:   products,
		Categories: categories,
	}, nil
}

// HandlerProductsPage возвращает продукты и информацию о пагинации.
func (r *Repository) HandlerProductsPage(page structs.PageProduct) ([]structs.Product, structs.PageProduct, error) {
	ctx := context.Background()

	query := `
		SELECT 
			pc.id,
			c2.name AS title,
			c2.description,
			pc.price_rub AS price,
			c.display_name AS category,
			ph.name AS image,
			pc.weight_kg AS packaging
		FROM products_categories pc
		LEFT JOIN categories c ON c.id = pc.category_id
		LEFT JOIN categories_2 c2 ON c2.id = pc.categories_2_id
		LEFT JOIN photo ph ON ph.id = pc.photo_id
		WHERE 1=1
		AND (c2.name LIKE ? OR ? = '')
		AND (c.display_name = ? OR ? = '')
	`

	// Сортировка
	switch page.Sort {
	case "title":
		query += " ORDER BY c2.name ASC"
	case "title_desc":
		query += " ORDER BY c2.name DESC"
	case "price":
		query += " ORDER BY pc.price_rub ASC"
	case "price_desc":
		query += " ORDER BY pc.price_rub DESC"
	default:
		query += " ORDER BY pc.id DESC"
	}

	query += " LIMIT ? OFFSET ?"

	// Подготовка запроса
	stmt, err := r.db.PrepareContext(ctx, query)
	if err != nil {
		return nil, page, fmt.Errorf("failed to prepare products query: %w", err)
	}
	defer func(stmt *sql.Stmt) {
		err := stmt.Close()
		if err != nil {
			return
		}
	}(stmt)

	rows, err := stmt.QueryContext(ctx,
		"%"+page.Search+"%",
		page.Search,
		page.Category,
		page.Category,
		page.Limit,
		(page.Page-1)*page.Limit,
	)
	if err != nil {
		return nil, page, fmt.Errorf("failed to query products: %w", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			return
		}
	}(rows)

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
		); err != nil {
			return nil, page, fmt.Errorf("failed to scan product: %w", err)
		}
		products = append(products, p)
	}
	if err = rows.Err(); err != nil {
		return nil, page, fmt.Errorf("rows iteration error: %w", err)
	}

	// Считаем общее количество
	countQuery := `
		SELECT COUNT(*)
		FROM products_categories pc
		LEFT JOIN categories c ON c.id = pc.category_id
		LEFT JOIN categories_2 c2 ON c2.id = pc.categories_2_id
		WHERE 1=1
		AND (c2.name LIKE ? OR ? = '')
		AND (c.display_name = ? OR ? = '')
	`
	countStmt, err := r.db.PrepareContext(ctx, countQuery)
	if err != nil {
		return nil, page, fmt.Errorf("failed to prepare count query: %w", err)
	}
	defer func(countStmt *sql.Stmt) {
		err := countStmt.Close()
		if err != nil {
			return
		}
	}(countStmt)

	err = countStmt.QueryRowContext(ctx,
		"%"+page.Search+"%",
		page.Search,
		page.Category,
		page.Category,
	).Scan(&page.Total)
	if err != nil {
		return nil, page, fmt.Errorf("failed to count products: %w", err)
	}

	page.TotalPages = (page.Total + page.Limit - 1) / page.Limit

	return products, page, nil
}

// AddProduct добавляем продукты.
func (r *Repository) AddProduct(product structs.Product) error {
	if strings.TrimSpace(product.Title) == "" {
		return errors.New("product title cannot be empty")
	}

	ctx := context.Background()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		r.app.Log.Errorf("Failed to begin transaction: %v", err)
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			err := tx.Rollback()
			if err != nil {
				return
			}
		}
	}()

	// 1. Категория (верхний уровень)
	var categoryID int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM categories WHERE display_name = ?`, product.Category).Scan(&categoryID)
	if err == sql.ErrNoRows {

		res, err := tx.ExecContext(ctx, `INSERT INTO categories(name, display_name) VALUES (?, ?)`, stripEmojis(product.Category), product.Category)
		if err != nil {
			return fmt.Errorf("failed to insert category: %w", err)
		}
		categoryID, err = res.LastInsertId()
		if err != nil {
			return fmt.Errorf("failed to get category ID: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to query category: %w", err)
	}

	// 2. Подкатегория (продукт)
	var subcategoryID int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM categories_2 WHERE display_name = ?`, product.Title).Scan(&subcategoryID)
	if err == sql.ErrNoRows {
		res, err := tx.ExecContext(ctx, `INSERT INTO categories_2(name, display_name, description) VALUES (?, ?, ?)`,
			stripEmojis(product.Title), product.Title, NormalizeWords(stripEmojis(product.Category)+stripEmojis(product.Title)))
		if err != nil {
			return fmt.Errorf("failed to insert subcategory: %w", err)
		}
		subcategoryID, err = res.LastInsertId()
		if err != nil {
			return fmt.Errorf("failed to get subcategory ID: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to query subcategory: %w", err)
	}

	// 3. Фото
	var photoID int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM photo WHERE name = ?`, product.Image).Scan(&photoID)
	if err == sql.ErrNoRows {
		res, err := tx.ExecContext(ctx, `INSERT INTO photo(name) VALUES (?)`, product.Image)
		if err != nil {
			return fmt.Errorf("failed to insert photo: %w", err)
		}
		photoID, err = res.LastInsertId()
		if err != nil {
			return fmt.Errorf("failed to get photo ID: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to query photo: %w", err)
	}

	// 4. Продукт-фасовка
	_, err = tx.ExecContext(ctx, `
		INSERT INTO products_categories
			(category_id, categories_2_id, photo_id, weight_kg, price_rub, text, callback)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		categoryID, subcategoryID, photoID,
		product.Packaging, product.Price,
		product.Text, product.Callback,
	)
	if err != nil {
		return fmt.Errorf("failed to insert product category: %w", err)
	}

	// Commit
	if err = tx.Commit(); err != nil {
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

	stmt, err := r.db.PrepareContext(ctx, `
		SELECT 
			pc.id,
			c.display_name AS category,
			c2.display_name AS title,
			c2.description,
			ph.name AS image,
			pc.weight_kg,
			pc.price_rub,
			pc.text,
			pc.callback
		FROM products_categories pc
		LEFT JOIN categories c ON pc.category_id = c.id
		LEFT JOIN categories_2 c2 ON pc.categories_2_id = c2.id
		LEFT JOIN photo ph ON pc.photo_id = ph.id
		WHERE pc.id = ?
	`)
	if err != nil {
		return structs.Product{}, fmt.Errorf("failed to prepare query: %w", err)
	}
	defer func(stmt *sql.Stmt) {
		err := stmt.Close()
		if err != nil {
			r.app.Log.Errorf("Failed to close product select: %v", err)
			return
		}
	}(stmt)

	var p structs.Product
	err = stmt.QueryRowContext(ctx, productID).Scan(
		&p.ID,
		&p.Category,
		&p.Title,
		&p.Description,
		&p.Image,
		&p.Packaging,
		&p.Price,
		&p.Text,
		&p.Callback,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return structs.Product{}, fmt.Errorf("product with ID %d not found", productID)
		}
		return structs.Product{}, fmt.Errorf("failed to scan product: %w", err)
	}
	return p, nil
}

func (r *Repository) UpdateProduct(p structs.Product) (err error) {
	if p.ID <= 0 {
		return errors.New("invalid product ID")
	}
	if strings.TrimSpace(p.Title) == "" {
		return errors.New("product title cannot be empty")
	}

	ctx := context.Background()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			err := tx.Rollback()
			if err != nil {
				r.app.Log.Errorf("Failed to close product select: %v", err)
				return
			}
		}
	}()

	// --- 1. Категория (верхняя)
	var newCatID int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM categories WHERE display_name = ?`, p.Category).Scan(&newCatID)
	if err == sql.ErrNoRows {
		res, err := tx.ExecContext(ctx, `INSERT INTO categories(name, display_name) VALUES (?, ?)`, stripEmojis(p.Category), p.Category)
		if err != nil {
			return fmt.Errorf("failed to insert category: %w", err)
		}
		newCatID, err = res.LastInsertId()
		if err != nil {
			return fmt.Errorf("failed to get category ID: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to query category: %w", err)
	}

	// --- 2. Подкатегория (товар)
	var subID int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM categories_2 WHERE display_name = ?`, p.Title).Scan(&subID)
	if err == sql.ErrNoRows {
		res, err := tx.ExecContext(ctx, `INSERT INTO categories_2(name, display_name, description) VALUES (?, ?, ?)`,
			stripEmojis(p.Title), p.Title, NormalizeWords(stripEmojis(p.Category)+stripEmojis(p.Title)))
		if err != nil {
			return fmt.Errorf("failed to insert subcategory: %w", err)
		}
		subID, err = res.LastInsertId()
		if err != nil {
			return fmt.Errorf("failed to get subcategory ID: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to query subcategory: %w", err)
	}

	// --- 3. Фото
	var oldPhotoID int64
	err = tx.QueryRowContext(ctx, `SELECT photo_id FROM products_categories WHERE id = ?`, p.ID).Scan(&oldPhotoID)
	if err != nil {
		return fmt.Errorf("failed to get old photo ID: %w", err)
	}

	var newPhotoID int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM photo WHERE id = ?`, oldPhotoID).Scan(&newPhotoID)
	if err == sql.ErrNoRows {
		res, err := tx.ExecContext(ctx, `INSERT INTO photo(name) VALUES (?)`, p.Image)
		if err != nil {
			return fmt.Errorf("failed to insert photo: %w", err)
		}
		newPhotoID, err = res.LastInsertId()
		if err != nil {
			return fmt.Errorf("failed to get new photo ID: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to query photo: %w", err)
	}

	// --- удаляем старое фото, если оно больше не используется
	if oldPhotoID != newPhotoID {
		var count int
		err = tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM products_categories WHERE photo_id = ?`,
			oldPhotoID).Scan(&count)
		if err != nil {
			return fmt.Errorf("failed to check old photo usage: %w", err)
		}
		if count == 0 {
			_, err = tx.ExecContext(ctx,
				`DELETE FROM photo WHERE id = ?`,
				oldPhotoID)
			if err != nil {
				return fmt.Errorf("failed to delete unused photo: %w", err)
			}
			r.app.Log.Infof("Deleted unused photo with ID: %d", oldPhotoID)
		}
	}

	// --- 4. Получаем старую категорию для удаления (если поменялась)
	var oldCatID int64
	err = tx.QueryRowContext(ctx, `SELECT category_id FROM products_categories WHERE id = ?`, p.ID).Scan(&oldCatID)
	if err != nil {
		return fmt.Errorf("failed to get old category: %w", err)
	}

	// --- 5. Обновляем продукт
	res, err := tx.ExecContext(ctx, `
		UPDATE products_categories
		SET category_id = ?, categories_2_id = ?, photo_id = ?, weight_kg = ?, price_rub = ?, text = ?, callback = ?
		WHERE id = ?`,
		newCatID, subID, newPhotoID, p.Packaging, p.Price, p.Text, p.Callback, p.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update product: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("product with ID %d not found", p.ID)
	}

	// --- 6. Удаляем старую категорию, если не используется
	if oldCatID != newCatID {
		var count int
		err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM products_categories WHERE category_id = ?`, oldCatID).Scan(&count)
		if err != nil {
			return fmt.Errorf("failed to check category usage: %w", err)
		}
		if count == 0 {
			_, err = tx.ExecContext(ctx, `DELETE FROM categories WHERE id = ?`, oldCatID)
			if err != nil {
				return fmt.Errorf("failed to delete unused category: %w", err)
			}
			r.app.Log.Infof("Deleted unused category with ID: %d", oldCatID)
		}
	}

	// --- 7. Commit
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (r *Repository) DeleteProduct(productID int) error {
	if productID <= 0 {
		return errors.New("invalid product ID")
	}

	ctx := context.Background()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			err := tx.Rollback()
			if err != nil {
				r.app.Log.Error(err)
				return
			}
		}
	}()

	var categoryID, category2ID, photoID int64
	err = tx.QueryRowContext(ctx, `
		SELECT category_id, categories_2_id, photo_id
		FROM products_categories
		WHERE id = ?
	`, productID).Scan(&categoryID, &category2ID, &photoID)

	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("product with ID %d not found", productID)
		}
		return fmt.Errorf("failed to fetch product metadata: %w", err)
	}

	// Удаляем сам продукт
	res, err := tx.ExecContext(ctx, `
		DELETE FROM products_categories
		WHERE id = ?
	`, productID)
	if err != nil {
		return fmt.Errorf("failed to delete product: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("product with ID %d not found", productID)
	}

	// Проверяем, используется ли категория
	var count int
	err = tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM products_categories WHERE category_id = ?
	`, categoryID).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check category usage: %w", err)
	}
	if count == 0 {
		_, err = tx.ExecContext(ctx, `DELETE FROM categories WHERE id = ?`, categoryID)
		if err != nil {
			return fmt.Errorf("failed to delete unused category: %w", err)
		}
		r.app.Log.Infof("Deleted unused category ID: %d", categoryID)
	}

	// Проверяем, используется ли подкатегория
	err = tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM products_categories WHERE categories_2_id = ?
	`, category2ID).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check subcategory usage: %w", err)
	}
	if count == 0 {
		_, err = tx.ExecContext(ctx, `DELETE FROM categories_2 WHERE id = ?`, category2ID)
		if err != nil {
			return fmt.Errorf("failed to delete unused subcategory: %w", err)
		}
		r.app.Log.Infof("Deleted unused subcategory ID: %d", category2ID)
	}

	// Проверяем, используется ли фото
	err = tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM products_categories WHERE photo_id = ?
	`, photoID).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check photo usage: %w", err)
	}
	if count == 0 {
		_, err = tx.ExecContext(ctx, `DELETE FROM photo WHERE id = ?`, photoID)
		if err != nil {
			return fmt.Errorf("failed to delete unused photo: %w", err)
		}
		r.app.Log.Infof("Deleted unused photo ID: %d", photoID)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (r *Repository) ExportExcel() ([]interface{}, error) {
	var result []interface{}

	rows, err := r.db.Query(`
		SELECT 
			c2.name AS title,
			pc.price_rub AS price,
			c2.description,
			c.display_name AS category,
			pc.weight_kg AS packaging
		FROM products_categories pc
		LEFT JOIN categories_2 c2 ON pc.categories_2_id = c2.id
		LEFT JOIN categories c ON pc.category_id = c.id
		ORDER BY c.display_name, c2.name
	`)
	if err != nil {
		r.app.Log.Error("Ошибка БД: " + err.Error())
		return nil, err
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil {
			r.app.Log.Error("Ошибка закрытия rows: " + cerr.Error())
		}
	}()

	rowNum := 1
	for rows.Next() {
		var p structs.Product
		if err := rows.Scan(&p.Title, &p.Price, &p.Description, &p.Category, &p.Packaging); err != nil {
			r.app.Log.Error("Ошибка при сканировании строки: " + err.Error())
			continue
		}

		entry := []interface{}{
			rowNum,
			p.Title,
			p.Price,
			p.Description,
			p.Category,
			p.Packaging,
		}
		result = append(result, entry)
		rowNum++
	}

	if err := rows.Err(); err != nil {
		r.app.Log.Error("Ошибка при итерации строк: " + err.Error())
		return nil, err
	}

	return result, nil
}

func (r *Repository) Categories() ([]string, error) {
	// Получаем список всех категорий из таблицы категорий
	categoryRows, err := r.db.Query("SELECT name FROM categories ORDER BY name")
	if err != nil {
		r.app.Log.Error("Ошибка при получении категорий: " + err.Error())
		return nil, err
	}
	defer func(categoryRows *sql.Rows) {
		err := categoryRows.Close()
		if err != nil {
			r.app.Log.Error("<UNK> <UNK> <UNK> <UNK>: " + err.Error())
			return
		}
	}(categoryRows)

	var categories []string
	for categoryRows.Next() {
		var cat string
		err = categoryRows.Scan(&cat)
		if err != nil {
			return nil, err
		}
		categories = append(categories, cat)
	}
	return categories, nil
}

func stripEmojis(text string) string {
	// Удаляет всё, кроме букв, цифр и пробелов (включая кириллицу)
	re := regexp.MustCompile(`[^\p{L}\p{N} ]+`)
	return strings.TrimSpace(re.ReplaceAllString(text, ""))
}
