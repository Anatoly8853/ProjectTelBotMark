package botTelegram

import (
	"ProjectTelBotMark/internal/structs"
	"testing"
)

func TestAddPurchaseToBasketMergesSameItemAndPrice(t *testing.T) {
	current := []structs.Purchase{{ID: 10, Item: "Черемша", Quantity: 1, Cost: 450}}
	updated, total, err := addPurchaseToBasket(current, "Черемша", 2, 450)
	if err != nil {
		t.Fatalf("не ожидалась ошибка: %v", err)
	}
	if len(updated) != 1 || updated[0].Quantity != 3 {
		t.Fatalf("ожидалась одна позиция с количеством 3, получено: %#v", updated)
	}
	if total != 1350 {
		t.Fatalf("итог = %d, ожидалось 1350", total)
	}
}

func TestAddPurchaseToBasketKeepsDifferentPricesSeparate(t *testing.T) {
	current := []structs.Purchase{{ID: 10, Item: "Черемша", Quantity: 1, Cost: 450}}
	updated, total, err := addPurchaseToBasket(current, "Черемша", 1, 500)
	if err != nil {
		t.Fatalf("не ожидалась ошибка: %v", err)
	}
	if len(updated) != 2 {
		t.Fatalf("ожидалось две позиции с разными ценами, получено %d", len(updated))
	}
	if total != 950 {
		t.Fatalf("итог = %d, ожидалось 950", total)
	}
}

func TestChangePurchaseQuantityRemovesItemAtZero(t *testing.T) {
	current := []structs.Purchase{{ID: 10, Item: "Черемша", Quantity: 1, Cost: 450}}
	updated, total, err := changePurchaseQuantity(current, 10, -1)
	if err != nil {
		t.Fatalf("не ожидалась ошибка: %v", err)
	}
	if len(updated) != 0 || total != 0 {
		t.Fatalf("позиция должна удалиться при количестве 0: %#v, сумма=%d", updated, total)
	}
}

func TestChangePurchaseQuantityIncrements(t *testing.T) {
	current := []structs.Purchase{{ID: 10, Item: "Черемша", Quantity: 1, Cost: 450}}
	updated, total, err := changePurchaseQuantity(current, 10, 1)
	if err != nil {
		t.Fatalf("не ожидалась ошибка: %v", err)
	}
	if len(updated) != 1 || updated[0].Quantity != 2 || total != 900 {
		t.Fatalf("неверный результат изменения количества: %#v, сумма=%d", updated, total)
	}
}
