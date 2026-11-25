package structs

import "testing"

func TestCalculatePurchasesTotal(t *testing.T) {
	purchases := []Purchase{
		{Item: "Черемша", Quantity: 2, Cost: 450},
		{Item: "Острый", Quantity: 1, Cost: 260},
	}
	got, err := CalculatePurchasesTotal(purchases)
	if err != nil {
		t.Fatalf("не ожидалась ошибка: %v", err)
	}
	if got != 1160 {
		t.Fatalf("итог = %d, ожидалось 1160", got)
	}
}

func TestCalculatePurchasesTotalRejectsInvalidPosition(t *testing.T) {
	_, err := CalculatePurchasesTotal([]Purchase{{Item: "", Quantity: 1, Cost: 100}})
	if err == nil {
		t.Fatal("ожидалась ошибка для пустого названия товара")
	}
}

func TestCalculatePurchasesTotalDetectsOverflow(t *testing.T) {
	maxInt := int(^uint(0) >> 1)
	_, err := CalculatePurchasesTotal([]Purchase{{Item: "Товар", Quantity: 2, Cost: maxInt}})
	if err == nil {
		t.Fatal("ожидалась ошибка переполнения")
	}
}
