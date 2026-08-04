package main

import (
	"ProjectTelBotMark/config"
	"ProjectTelBotMark/internal/botTelegram"
	"ProjectTelBotMark/internal/dataSources"
	"ProjectTelBotMark/internal/reminders"
	"ProjectTelBotMark/internal/routers"
	"ProjectTelBotMark/internal/structs"

	slogavp "github.com/Anatoly8853/slog-avp/v2"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {
	// Настраиваем логгер
	app, err := slogavp.New(
		slogavp.WithConsole(false),
		slogavp.WithLogDir("var/log/myapp"),
	)
	if err != nil {
		// обработать ошибку
	}
	defer app.Close()
	cfg := config.LoadConfig(app)

	bot, err := tgbotapi.NewBotAPI(cfg.TGtoken)
	if err != nil {
		app.Log.Fatalf("Нет токена TELEGRAM_TOKEN: %v", err)
	}

	// Открываем соединение с базой данных SQLite.
	db, err := dataSources.InitDB(app)
	if err != nil {
		app.Log.Fatalf("Error initializing databases: %v", err)
	}

	repo := dataSources.NewRepository(db, app)
	app.Log.Info("Все норм")
	/*
		// 1. Чтение файла JSON
		jsonData, er := os.ReadFile("data.json")
		if er != nil {

			app.Log.Fatalf("Ошибка чтения файла JSON: %v", er)

		}

		// 2. Распаковка данных
		err = json.Unmarshal(jsonData, &structs.Categories)
		if err != nil {

			app.Log.Fatalf("Ошибка распаковки JSON: %v", err)

		}
	*/
	cat, err := repo.GetCategoriesWithProducts()
	if err != nil {
		app.Log.Error("GetCategoriesWithProducts", err)
		return
	}
	structs.Categories = cat

	defer repo.CloseDB()

	bot.Debug = false //false //true   // выводим лог или нет

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
	haydler := routers.NewGinRouter(repo, app, bot)
	haydler.GinRouter()
}
