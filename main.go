// main пакет.
package main

import (
	"ProjectTelBotMark/dataSources"
	"ProjectTelBotMark/loggers"
	"ProjectTelBotMark/reminders"
	`ProjectTelBotMark/routers`
	"ProjectTelBotMark/structs"
	"encoding/json"
	"errors"
	"os"
	"regexp"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
)

func SetupApplication() *structs.Application {
	// Настройка логгера перед его инициализацией
	loggers.SetLogConsole(false) // Логи будут записываться в файл
	loggers.SetIsDebugMode(true)
	loggers.SetIsInfoMode(true)
	loggers.SetIsWarnMode(true)
	// Настраиваем логгер
	logger := loggers.SetupLogger()
	// Создаем экземпляр Application с настроенным логгером
	app = &structs.Application{Log: logger}
	return app
}

//var logger *slog.Logger
var app *structs.Application

var bot *tgbotapi.BotAPI

func main() {

	// Настраиваем логгер
	app = SetupApplication()

	// Открываем соединение с базой данных SQLite.
	var err error

	dataSources.InitDB(app)
	defer dataSources.CloseDB(app)

	// Загружаем токен бота из файла .env
	err = godotenv.Load()
	if err != nil {

		app.Log.Fatalf("Не удалось открыть файл .env: %v", err)

	}

	bot, err = tgbotapi.NewBotAPI(os.Getenv("TELEGRAM_TOKEN"))
	if err != nil {

		app.Log.Fatalf("Нет токена TELEGRAM_TOKEN: %v", err)

	}

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

	bot.Debug = false //false //true   // выводим лог или нет

	_, err = dataSources.GetUsers()
	if err != nil {
		app.Log.Errorf("Error getting users: %v", err)
		return
	}

	UpdateMessage()

	dataSources.StartBackupRoutine()

	routers.LoadCategories(app)

	// Запуск ежедневной проверки базы данных
	reminders.StartDailyCheck(bot)

	// Запускаем сервер
	routers.GinRouter(app, bot)

}

// createCategoryMessage Вывод изображения продуктов и кнопки фасовки продукта.
func createCategoryMessage(update tgbotapi.Update, categoryName string) error {
	if update.Message.Chat.ID == 0 {
		app.Log.Errorf("нет данных для обработки update.Message.Chat.ID = %v ", update.Message.Chat.ID)
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

	photoMessagesDelete(chatID)

	keyboardMessagesDelete(chatID)

	if categoryData.Photo == "" {
		keyboardReply, err := CreateCategoryKeyboardReply(categoryData.CategoryName)
		if err != nil {
			// Обработка ошибки
			app.Log.Errorf("ошибка %v", err)
			return err
		}

		// Создаем сообщение с инлайн-клавиатурой
		msg := tgbotapi.NewMessage(chatID, "Выбранная категория: "+categoryData.CategoryName)
		//msg := tgbotapi.NewMessage(chatID, ".")
		msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(keyboardReply...)

		// Отправляем сообщение с клавиатурой
		sentMsg, err := bot.Send(msg)
		if err != nil {
			app.Log.Errorf("Ошибка при отправке сообщения: %v", err)
			return err
		} else {
			messagesToDelete[chatID] = append(messagesToDelete[chatID], sentMsg.MessageID)
		}
	} else {
		// Создаем медиа-объект для изображения
		photo := tgbotapi.NewInputMediaPhoto(tgbotapi.FilePath(categoryData.Photo))

		// Создаем медиагруппу с изображением и URL
		mediaGroup := tgbotapi.NewMediaGroup(chatID, []interface{}{photo})

		// Отправляем медиагруппу
		sentMsgs, err := bot.SendMediaGroup(mediaGroup)
		if err != nil {
			app.Log.Errorf("Ошибка при отправке сообщения медиагруппу: %v", err)
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
			app.Log.Errorf("Ошибка не удается создать кнопки клавиатуры %v", err)
			return err
		}

		// Создаем сообщение с инлайн-клавиатурой
		msg := tgbotapi.NewMessage(chatID, "Выберите нужное количество продукта: "+categoryData.CategoryMsg)
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(keyboard...)

		// Отправляем сообщение с клавиатурой
		sentMsg, err := bot.Send(msg)
		if err != nil {
			app.Log.Errorf("Ошибка при отправке сообщения: %v", err)
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

	return JoinStrings(emojis), cleanText
}

// JoinStrings Объединяем все строки из среза в одну строку из update.Message.Text.
func JoinStrings(str []string) string {
	// Объединяем все строки из среза в одну строку
	result := ""
	for _, s := range str {
		result += s
	}
	return result
}
