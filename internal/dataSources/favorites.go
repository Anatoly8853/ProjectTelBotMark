package dataSources

import (
	"ProjectTelBotMark/internal/structs"
	"context"
	"database/sql"
	"fmt"
)

// ToggleFavorite добавляет товар в избранное или удаляет его, если он уже там есть.
// Возвращает true, если товар добавлен, и false, если удалён.
func (r *Repository) ToggleFavorite(userID int64, productID int) (bool, error) {
	if userID <= 0 || productID <= 0 {
		err := fmt.Errorf("некорректные данные избранного: userID=%d, productID=%d", userID, productID)
		r.app.Log.Error("ToggleFavorite", err.Error())
		return false, err
	}

	tx, err := r.db.BeginTx(context.Background(), nil)
	if err != nil {
		r.app.Log.Errorf("ToggleFavorite: ошибка начала транзакции: %v", err)
		return false, err
	}
	committed := false
	defer func() {
		if !committed {
			if rollbackErr := tx.Rollback(); rollbackErr != nil && rollbackErr != sql.ErrTxDone {
				r.app.Log.Errorf("ToggleFavorite: ошибка отката транзакции: %v", rollbackErr)
			}
		}
	}()

	result, err := tx.ExecContext(context.Background(),
		`DELETE FROM tg_user_favorites WHERE user_id = ? AND product_id = ?`, userID, productID)
	if err != nil {
		r.app.Log.Errorf("ToggleFavorite: ошибка удаления избранного: userID=%d, productID=%d, ошибка=%v", userID, productID, err)
		return false, err
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		r.app.Log.Errorf("ToggleFavorite: не удалось определить результат удаления: %v", err)
		return false, err
	}
	if deleted > 0 {
		if err := tx.Commit(); err != nil {
			r.app.Log.Errorf("ToggleFavorite: ошибка фиксации удаления: %v", err)
			return false, err
		}
		committed = true
		r.app.Log.Infof("Избранное: товар удалён: userID=%d, productID=%d", userID, productID)
		return false, nil
	}

	if _, err := tx.ExecContext(context.Background(),
		`INSERT INTO tg_user_favorites (user_id, product_id) VALUES (?, ?)`, userID, productID); err != nil {
		r.app.Log.Errorf("ToggleFavorite: ошибка добавления товара: userID=%d, productID=%d, ошибка=%v", userID, productID, err)
		return false, err
	}
	if err := tx.Commit(); err != nil {
		r.app.Log.Errorf("ToggleFavorite: ошибка фиксации добавления: %v", err)
		return false, err
	}
	committed = true
	r.app.Log.Infof("Избранное: товар добавлен: userID=%d, productID=%d", userID, productID)
	return true, nil
}

// GetFavoriteProducts возвращает избранные фасовки пользователя в порядке добавления.
func (r *Repository) GetFavoriteProducts(userID int64) ([]structs.Product, error) {
	if userID <= 0 {
		err := fmt.Errorf("некорректный userID: %d", userID)
		r.app.Log.Error("GetFavoriteProducts", err.Error())
		return nil, err
	}

	rows, err := r.db.QueryContext(context.Background(), `
		SELECT pc.id, COALESCE(c.name, ''), COALESCE(c2.display_name, ''), COALESCE(c2.description, ''),
		       pc.weight_kg, pc.price_rub, pc.text, pc.callback, COALESCE(ph.name, '')
		FROM tg_user_favorites f
		JOIN products_categories pc ON pc.id = f.product_id
		LEFT JOIN categories c ON c.id = pc.category_id
		LEFT JOIN categories_2 c2 ON c2.id = pc.categories_2_id
		LEFT JOIN photo ph ON ph.id = pc.photo_id
		WHERE f.user_id = ?
		ORDER BY f.created_at DESC, pc.id DESC
		LIMIT 40`, userID)
	if err != nil {
		r.app.Log.Errorf("GetFavoriteProducts: ошибка запроса: userID=%d, ошибка=%v", userID, err)
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			r.app.Log.Errorf("GetFavoriteProducts: ошибка закрытия SQL rows: %v", closeErr)
		}
	}()

	products := make([]structs.Product, 0)
	for rows.Next() {
		var product structs.Product
		if err := rows.Scan(&product.ID, &product.Category, &product.Title, &product.Description,
			&product.Packaging, &product.Price, &product.Text, &product.Callback, &product.Image); err != nil {
			r.app.Log.Errorf("GetFavoriteProducts: ошибка чтения товара: %v", err)
			return nil, err
		}
		// Старые записи callback в SQLite могли содержать цену и название.
		// Для избранного всегда формируем безопасный callback по ID строки.
		product.Callback = fmt.Sprintf("add_%d", product.ID)
		products = append(products, product)
	}
	if err := rows.Err(); err != nil {
		r.app.Log.Errorf("GetFavoriteProducts: ошибка обхода SQL rows: %v", err)
		return nil, err
	}
	return products, nil
}

// GetProductForBasket возвращает название, категорию, фасовку и цену товара по его ID.
func (r *Repository) GetProductForBasket(productID int) (structs.Product, error) {
	if productID <= 0 {
		return structs.Product{}, fmt.Errorf("некорректный ID товара: %d", productID)
	}

	var product structs.Product
	err := r.db.QueryRowContext(context.Background(), `
		SELECT COALESCE(c.name, ''), COALESCE(c2.display_name, ''),
		       COALESCE(pc.weight_kg, ''), pc.price_rub
		FROM products_categories pc
		LEFT JOIN categories c ON c.id = pc.category_id
		LEFT JOIN categories_2 c2 ON c2.id = pc.categories_2_id
		WHERE pc.id = ?`, productID,
	).Scan(&product.Category, &product.Title, &product.Packaging, &product.Price)
	if err != nil {
		if err == sql.ErrNoRows {
			return structs.Product{}, fmt.Errorf("товар с ID %d не найден", productID)
		}
		return structs.Product{}, fmt.Errorf("не удалось получить товар с ID %d: %w", productID, err)
	}
	product.ID = productID
	return product, nil
}

// GetProductCategoriesByCallback возвращает родительскую категорию и подкатегорию товара по неизменённому callback из каталога.
func (r *Repository) GetProductCategoriesByCallback(callback string) (string, string, error) {
	if callback == "" {
		err := fmt.Errorf("пустой callback товара")
		r.app.Log.Error("GetProductCategoriesByCallback", err.Error())
		return "", "", err
	}

	var category, subcategory string
	err := r.db.QueryRowContext(context.Background(), `
		SELECT COALESCE(c.name, ''), COALESCE(c2.display_name, '')
		FROM products_categories pc
		LEFT JOIN categories c ON c.id = pc.category_id
		LEFT JOIN categories_2 c2 ON c2.id = pc.categories_2_id
		WHERE pc.callback = ?
		LIMIT 1`, callback).Scan(&category, &subcategory)
	if err != nil {
		r.app.Log.Errorf("GetProductCategoriesByCallback: не удалось найти категории товара по callback=%q: %v", callback, err)
		return "", "", err
	}
	return category, subcategory, nil
}

// GetLastOrderPurchases возвращает позиции последнего заказа пользователя.
func (r *Repository) GetLastOrderPurchases(userID int64) ([]structs.Purchase, error) {
	if userID <= 0 {
		err := fmt.Errorf("некорректный userID: %d", userID)
		r.app.Log.Error("GetLastOrderPurchases", err.Error())
		return nil, err
	}

	rows, err := r.db.QueryContext(context.Background(), `
		SELECT oi.item_name, o.quantity, o.cost
		FROM tg_user_orders_date od
		JOIN tg_user_Chat_ID u ON u.id = od.user_id
		JOIN tg_user_orders o ON o.order_id = od.id
		JOIN tg_user_order_id oi ON oi.id = o.item_name
		WHERE u.user_id = ?
		  AND od.id = (
			SELECT od2.id
			FROM tg_user_orders_date od2
			JOIN tg_user_Chat_ID u2 ON u2.id = od2.user_id
			WHERE u2.user_id = ?
			ORDER BY od2.id DESC LIMIT 1
		  )
		ORDER BY o.id ASC`, userID, userID)
	if err != nil {
		r.app.Log.Errorf("GetLastOrderPurchases: ошибка запроса: userID=%d, ошибка=%v", userID, err)
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			r.app.Log.Errorf("GetLastOrderPurchases: ошибка закрытия SQL rows: %v", closeErr)
		}
	}()

	purchases := make([]structs.Purchase, 0)
	for rows.Next() {
		var purchase structs.Purchase
		if err := rows.Scan(&purchase.Item, &purchase.Quantity, &purchase.Cost); err != nil {
			r.app.Log.Errorf("GetLastOrderPurchases: ошибка чтения позиции: %v", err)
			return nil, err
		}
		purchases = append(purchases, purchase)
	}
	if err := rows.Err(); err != nil {
		r.app.Log.Errorf("GetLastOrderPurchases: ошибка обхода SQL rows: %v", err)
		return nil, err
	}
	return purchases, nil
}
