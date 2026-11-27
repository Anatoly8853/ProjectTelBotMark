package structs

import "testing"

func TestCategoryCacheReturnsIndependentCopies(t *testing.T) {
	SetCategories(map[string]Category{
		"Старт": {
			CategoryName: "Старт",
			Buttons:      []ButtonData{{Text: "Категория", Callback: "cat_test"}},
			ButtonsM:     []ButtonData{{Text: "Корзина", Callback: "cat_cart"}},
		},
	})

	category, ok := GetCategory("Старт")
	if !ok {
		t.Fatal("категория должна присутствовать в кэше")
	}
	category.Buttons[0].Text = "Изменено"
	category.ButtonsM[0].Text = "Изменено"

	all := GetCategoriesSnapshot()
	all["Старт"] = Category{CategoryName: "Подмена"}

	stored, ok := GetCategory("Старт")
	if !ok {
		t.Fatal("категория должна оставаться в кэше")
	}
	if stored.CategoryName != "Старт" || stored.Buttons[0].Text != "Категория" || stored.ButtonsM[0].Text != "Корзина" {
		t.Fatalf("внешнее изменение повлияло на кэш: %#v", stored)
	}
}
