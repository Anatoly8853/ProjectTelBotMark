// Package dateutil содержит единые функции разбора дат, сохранённых в SQLite
// и получаемых из пользовательского интерфейса.
package dateutil

import (
	"fmt"
	"time"
)

var layouts = []string{
	"2006-01-02",
	"02.01.2006",
	time.RFC3339,
	"2006-01-02 15:04:05",
	"2006-01-02 15:04",
}

// Parse принимает дату SQLite (YYYY-MM-DD), дату интерфейса (DD.MM.YYYY)
// и RFC3339. Для дат без времени используется локальная временная зона.
func Parse(value string) (time.Time, error) {
	for _, layout := range layouts {
		var parsed time.Time
		var err error
		if layout == time.RFC3339 {
			parsed, err = time.Parse(layout, value)
		} else {
			parsed, err = time.ParseInLocation(layout, value, time.Local)
		}
		if err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("неподдерживаемый формат даты %q", value)
}

// Format отображает дату в формате DD.MM.YYYY.
func Format(value string) (string, error) {
	parsed, err := Parse(value)
	if err != nil {
		return "", err
	}
	return parsed.Format("02.01.2006"), nil
}
