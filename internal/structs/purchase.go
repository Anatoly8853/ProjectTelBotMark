package structs

import (
	"fmt"
	"strings"
)

// CalculatePurchasesTotal проверяет позиции и рассчитывает общую стоимость корзины.
// Cost — цена одной единицы, Quantity — количество единиц.
func CalculatePurchasesTotal(purchases []Purchase) (int, error) {
	if len(purchases) == 0 {
		return 0, fmt.Errorf("корзина пуста")
	}

	maxInt := int(^uint(0) >> 1)
	total := 0
	for i, purchase := range purchases {
		if strings.TrimSpace(purchase.Item) == "" || purchase.Quantity <= 0 || purchase.Cost <= 0 {
			return 0, fmt.Errorf("некорректная позиция %d: товар=%q, количество=%d, цена=%d", i, purchase.Item, purchase.Quantity, purchase.Cost)
		}
		if purchase.Cost > maxInt/purchase.Quantity {
			return 0, fmt.Errorf("переполнение стоимости позиции %d: товар=%q", i, purchase.Item)
		}
		lineTotal := purchase.Cost * purchase.Quantity
		if total > maxInt-lineTotal {
			return 0, fmt.Errorf("переполнение общей стоимости корзины")
		}
		total += lineTotal
	}
	return total, nil
}
