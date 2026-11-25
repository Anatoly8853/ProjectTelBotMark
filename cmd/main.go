package main

import (
	"ProjectTelBotMark/config"
	"ProjectTelBotMark/internal/botTelegram"
	"ProjectTelBotMark/internal/dataSources"
	"ProjectTelBotMark/internal/reminders"
	"ProjectTelBotMark/internal/routers"
	"ProjectTelBotMark/internal/structs"
	"fmt"
	"os"
	"strings"

	slogavp "github.com/Anatoly8853/slog-avp/v2"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {
	// Настраиваем логгер
	app, err := slogavp.New(
		slogavp.WithConsole(false),
		slogavp.WithLogDir("log"),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "не удалось настроить логгер: %v\n", err)
		os.Exit(1)
	}
	defer app.Close()

	cfg := config.LoadConfig(app)
	if strings.TrimSpace(cfg.AdminUsername) == "" || len(cfg.AdminPassword) < 12 {
		app.Log.Error("Не заданы ADMIN_USERNAME и ADMIN_PASSWORD (не менее 12 символов). Добавьте их в app.env перед запуском.")
		return
	}

	bot, err := tgbotapi.NewBotAPI(cfg.TGtoken)
	if err != nil {
		app.Fatalf("Нет токена TELEGRAM_TOKEN: %v", err)
	}

	db, err := dataSources.InitDB(app)
	if err != nil {
		app.Fatalf("Error initializing databases: %v", err)
	}

	repo := dataSources.NewRepository(db, app)

	cat, err := repo.GetCategoriesWithProducts()
	if err != nil {
		app.Log.Error("GetCategoriesWithProducts", err)
		return
	}
	structs.Categories = cat

	defer repo.CloseDB()

	bot.Debug = true //false //true   // выводим лог или нет

	_, err = repo.GetUsers()
	if err != nil {
		app.Log.Errorf("Error getting users: %v", err)
		return
	}

	updatemessage := botTelegram.NewUpdateMessage(repo, app, bot)
	updatemessage.UpdateMessage(cfg)

	repo.StartBackupRoutine()

	// Запуск ежедневной проверки базы данных
	messageservice := reminders.NewMessageService(repo, app, bot)
	messageservice.StartDailyCheck()

	// Запускаем сервер
	haydler := routers.NewGinRouter(repo, app, bot, cfg)
	haydler.GinRouter()
}
