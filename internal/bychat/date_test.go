package bychat

import (
	"testing"
	"time"

	slogavp "github.com/Anatoly8853/slog-avp/v2"
)

func TestFormatDateStringAcceptsDatesBeyondNextMonth(t *testing.T) {
	app, err := slogavp.New(slogavp.WithConsole(false), slogavp.WithLogDir(t.TempDir()))
	if err != nil {
		t.Fatalf("не удалось создать тестовый логгер: %v", err)
	}
	defer app.Close()

	future := time.Now().In(time.Local).AddDate(0, 3, 1)
	input := future.Format("02.01.2006")
	got, message := FormatDateString(app, input)
	if message != "" {
		t.Fatalf("дата за пределами следующего месяца отклонена: %s", message)
	}
	if got != input {
		t.Fatalf("получена дата %q, ожидалась %q", got, input)
	}
}

func TestFormatDateStringRejectsPastDate(t *testing.T) {
	app, err := slogavp.New(slogavp.WithConsole(false), slogavp.WithLogDir(t.TempDir()))
	if err != nil {
		t.Fatalf("не удалось создать тестовый логгер: %v", err)
	}
	defer app.Close()

	past := time.Now().In(time.Local).AddDate(0, 0, -1).Format("02.01.2006")
	if _, message := FormatDateString(app, past); message == "" {
		t.Fatal("прошедшая дата должна отклоняться")
	}
}
