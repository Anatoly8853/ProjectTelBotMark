package dataSources

import (
	"testing"

	"ProjectTelBotMark/internal/structs"
)

func TestPaginateCategoriesEmptyCatalog(t *testing.T) {
	got := paginateCategories(map[string]structs.Category{})

	start, ok := got["Старт"]
	if !ok {
		t.Fatal("для пустого каталога должна создаваться стартовая страница")
	}
	if start.CategoryName != "Старт" {
		t.Fatalf("имя стартовой страницы = %q, ожидалось %q", start.CategoryName, "Старт")
	}
	if len(start.Buttons) != 0 {
		t.Fatalf("у пустого каталога не должно быть товарных кнопок: %#v", start.Buttons)
	}
	if len(start.ButtonsM) == 0 {
		t.Fatal("на стартовой странице должны оставаться служебные кнопки")
	}
}
