package main

import (
	"ProjectTelBotMark/config"
	"ProjectTelBotMark/internal/dataSources"
	"ProjectTelBotMark/internal/reminders"
	"ProjectTelBotMark/internal/routers"
	"ProjectTelBotMark/internal/structs"
	"errors"
	slogavp "github.com/Anatoly8853/slog-avp/v2"
	"regexp"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const UploadsDir = "./internal/routers/html"

func main() {
	// Настраиваем логгер
	app := slogavp.SetupApplication()
	cfg := config.LoadConfig(app)

	//bot, err = tgbotapi.NewBotAPI(os.Getenv("TELEGRAM_TOKEN"))
	bot, err := tgbotapi.NewBotAPI(cfg.TGtoken)
	if err != nil {
		app.Log.Fatalf("Нет токена TELEGRAM_TOKEN: %v", err)
	}

	// Открываем соединение с базой данных SQLite.
	db, err := dataSources.InitDB(app)
	if err != nil {
		app.Log.Fatalf("Error initializing databases: %v", err)
	}

	repo := dataSources.NewRepository(db, app, bot)
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
		//app.Log.Error("GetCategoriesWithProducts", err)
	}
	structs.Categories = cat
	//app.Log.Info("GetCategoriesWithProducts", cat)
	defer repo.CloseDB()

	bot.Debug = false //false //true   // выводим лог или нет

	_, err = repo.GetUsers()
	if err != nil {
		app.Log.Errorf("Error getting users: %v", err)
		return
	}

	updatemessage := NewUpdateMessage(repo, app, bot)
	updatemessage.UpdateMessage(cfg)

	repo.StartBackupRoutine()

	// Запуск ежедневной проверки базы данных
	messageservice := reminders.NewMessageService(repo, app, bot)
	messageservice.StartDailyCheck()

	// Запускаем сервер
	haydler := routers.NewGinRouter(repo, app, bot)
	haydler.GinRouter()
}

// createCategoryMessage Вывод изображения продуктов и кнопки фасовки продукта.
func (m *MessageService) createCategoryMessage(update tgbotapi.Update, categoryName string) error {

	if update.Message.Chat.ID == 0 {
		m.app.Log.Errorf("нет данных для обработки update.Message.Chat.ID = %v ", update.Message.Chat.ID)
		er := "нет данных для обработки update.Message.Chat.ID"
		return errors.New(er)
	}

	chatID := update.Message.Chat.ID
	// Поиск категории в данных
	categoryData, found := structs.Categories[categoryName]
	if !found {
		er := "категория не найдена в createCategoryMessage"
		return errors.New(er)
	}

	m.photoMessagesDelete(chatID)
	m.keyboardMessagesDelete(chatID)

	if categoryData.Photo == "" {
		keyboardReply, err := CreateCategoryKeyboardReply(categoryData.CategoryName)
		if err != nil {
			// Обработка ошибки
			m.app.Log.Errorf("ошибка %v", err)
			return err
		}

		// Создаем сообщение с инлайн-клавиатурой
		msg := tgbotapi.NewMessage(chatID, "Выбранная категория: "+categoryData.CategoryName)
		//msg := tgbotapi.NewMessage(chatID, ".")
		msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(keyboardReply...)

		// Отправляем сообщение с клавиатурой
		sentMsg, err := m.bot.Send(msg)
		if err != nil {
			m.app.Log.Errorf("Ошибка при отправке сообщения: %v", err)
			return err
		} else {
			messagesToDelete[chatID] = append(messagesToDelete[chatID], sentMsg.MessageID)
		}
	} else {
		// Создаем медиа-объект для изображения
		photo := tgbotapi.NewInputMediaPhoto(tgbotapi.FilePath(UploadsDir + "/" + categoryData.Photo))

		// Создаем медиагруппу с изображением и URL
		mediaGroup := tgbotapi.NewMediaGroup(chatID, []interface{}{photo})

		// Отправляем медиагруппу
		sentMsgs, err := m.bot.SendMediaGroup(mediaGroup)
		if err != nil {
			m.app.Log.Errorf("Ошибка при отправке сообщения медиагруппу: %v", err)
			return err
		} else {
			// Сохраняем идентификатор сообщения для последующего удаления
			if len(sentMsgs) > 0 {
				photoMessagesToDelete[chatID] = sentMsgs[0].MessageID
			}
		}

		// Создаем кнопки клавиатуры
		keyboard, err := CreateCategoryKeyboard(categoryData.CategoryName)
		if err != nil {
			// Обработка ошибки
			m.app.Log.Errorf("Ошибка не удается создать кнопки клавиатуры %v", err)
			return err
		}

		// Создаем сообщение с инлайн-клавиатурой
		msg := tgbotapi.NewMessage(chatID, "Выберите нужное количество продукта: "+categoryData.CategoryMsg)
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(keyboard...)

		// Отправляем сообщение с клавиатурой
		sentMsg, err := m.bot.Send(msg)
		if err != nil {
			m.app.Log.Errorf("Ошибка при отправке сообщения: %v", err)
			return err
		} else {
			// Сохраняем идентификатор сообщения для последующего удаления
			messagesToDelete[chatID] = append(messagesToDelete[chatID], sentMsg.MessageID)
			keyboardMessagesToDelete[chatID] = sentMsg.MessageID // Сохраняем идентификатор сообщения с клавиатурой
		}
	}
	//deletePreviousMessages(bot, update.Message.Chat.ID, update.Message.MessageID)
	return nil
}

// SeparateEmojiAndText Паттерн для поиска эмотзи update.Message.Text.
func SeparateEmojiAndText(text string) (string, string) {
	// Паттерн для поиска эмотзи
	emojiPattern := regexp.MustCompile(`\p{So}`)
	// Ищем все эмотзи в тексте
	emojis := emojiPattern.FindAllString(text, -1)

	// Удаляем эмотзи из исходного текста
	cleanText := emojiPattern.ReplaceAllString(text, "")
	// Удаляем лишние пробелы
	cleanText = regexp.MustCompile(`\s+`).ReplaceAllString(cleanText, " ")

	return joinStrings(emojis), cleanText
}

// joinStrings Объединяем все строки из среза в одну строку из update.Message.Text.
func joinStrings(str []string) string {
	// Объединяем все строки из среза в одну строку
	result := ""
	for _, s := range str {
		result += s
	}
	return result
}
