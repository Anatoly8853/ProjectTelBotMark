package botTelegram

import "testing"

func TestFormatProductTitleSameCategoryAndSubcategory(t *testing.T) {
	got := formatProductTitle("Морковка", "Морковка", "Морковка")
	if want := "Морковка"; got != want {
		t.Fatalf("formatProductTitle() = %q, want %q", got, want)
	}
}

func TestFormatProductTitleKeepsPackagingSuffix(t *testing.T) {
	got := formatProductTitle("Морковка", "Морковка", "Морковка 1 кг")
	if want := "Морковка 1 кг"; got != want {
		t.Fatalf("formatProductTitle() = %q, want %q", got, want)
	}
}

func TestFormatProductTitleDifferentCategoryAndSubcategory(t *testing.T) {
	got := formatProductTitle("Овощи", "Морковка", "Морковка 1 кг")
	if want := "Овощи морковка 1 кг"; got != want {
		t.Fatalf("formatProductTitle() = %q, want %q", got, want)
	}
}

func TestNormalizeProductTitleRemovesRepeatedFirstWord(t *testing.T) {
	got := normalizeProductTitle("Морковка морковка 1 кг")
	if want := "Морковка 1 кг"; got != want {
		t.Fatalf("normalizeProductTitle() = %q, want %q", got, want)
	}
}

func TestFormatProductTitleIgnoresEmojiWhenComparingCategory(t *testing.T) {
	got := formatProductTitle("Морковка", "🥕Морковка", "🥕Морковка 1 кг")
	if want := "Морковка 1 кг"; got != want {
		t.Fatalf("formatProductTitle() = %q, want %q", got, want)
	}
}

func TestFormatProductTitleWhenCallbackOmitsEmoji(t *testing.T) {
	got := formatProductTitle("Морковка", "🥕Морковка", "Морковка 1 кг")
	if want := "Морковка 1 кг"; got != want {
		t.Fatalf("formatProductTitle() = %q, want %q", got, want)
	}
}
