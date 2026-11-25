package dateutil

import "testing"

func TestFormatSupportedDateLayouts(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "SQLite", input: "2026-11-25", want: "25.11.2026"},
		{name: "interface", input: "25.11.2026", want: "25.11.2026"},
		{name: "RFC3339", input: "2026-11-25T10:30:00Z", want: "25.11.2026"},
		{name: "SQLite datetime", input: "2026-11-25 10:30:00", want: "25.11.2026"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Format(tt.input)
			if err != nil {
				t.Fatalf("Format(%q) вернул ошибку: %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("Format(%q) = %q, ожидалось %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFormatRejectsInvalidDate(t *testing.T) {
	if _, err := Format("31.02.2026"); err == nil {
		t.Fatal("ожидалась ошибка для несуществующей даты")
	}
}
