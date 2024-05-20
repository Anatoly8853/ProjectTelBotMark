// main пакет.
package main

import (
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// EnableLogging Глобальная переменная для включения/отключения записи логов.
var EnableLogging = true

// router создаем роутер.
var router *gin.Engine

// GinRouter Gin Router функция запускает сервер.
func GinRouter() {
	gin.SetMode(gin.ReleaseMode)
	router = gin.Default()
	router.Static("/assets/", "front/")
	router.LoadHTMLGlob("html/templates/*")
	router.GET("/", HandlerIndex)
	err := router.Run(":8090")
	if err != nil {
		logError("GinRouter", err)
		return
	}
}

func main() {

	// Открываем соединение с базой данных SQLite
	var err error
	InitDB()
	defer CloseDB()

	// Загружаем токен бота из файла .env
	err = godotenv.Load()
	if err != nil {
		//log.Fatal("Error loading .env file")
		logError("Не удалось открыть файл .env:", err)
		panic(err)
	}

	bot, err := tgbotapi.NewBotAPI(os.Getenv("TELEGRAM_TOKEN"))
	if err != nil {
		logError("Неn токена TELEGRAM_TOKEN:", err)
		panic(err)
	}

	// 1. Чтение файла JSON
	jsonData, er := os.ReadFile("data.json")
	if er != nil {
		logError("Ошибка чтения файла JSON:", er)
		panic(er)
	}

	// 2. Распаковка данных
	err = json.Unmarshal(jsonData, &Categories)
	if err != nil {
		logError("Ошибка распаковки JSON:", err)
		return
	}

	bot.Debug = false //false //true   // выводим лог или нет

	//err = fmt.Errorf("все норм")
	//str := fmt.Sprintf("Authorized on account %s", bot.Self.UserName)
	//logError(str, err)

	go UpdateMessage(bot)

	GinRouter()

}

// UpdateMessage Основной обработчик всех данных бота.
func UpdateMessage(bot *tgbotapi.BotAPI) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 3

	updates := bot.GetUpdatesChan(u)

	for update := range updates {

		if update.CallbackQuery != nil {
			// Обработка CallbackQuery
			handleCallbackQueryData(bot, update)
			continue // Переход к следующему обновлению после обработки CallbackQuery
		}

		if update.Message != nil {
			// В этом месте обрабатываем все сообщения, включая команды
			if update.Message.IsCommand() {
				// Обработка команд пользователя
				HandleCommands(bot, update)
			} else {
				// Обработка текстовых сообщений (не команд)
				handleMessageText(bot, update)
			}
			continue
		}
	}
}

// HandleCommands Обработка команды /start и вывод основного меню.
func HandleCommands(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	//var err error
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Нет такой каманды")
	if update.Message.IsCommand() {
		// Извлекаем команду из сообщения
		command := update.Message.Command()
		// Extract the command from the Message.
		switch command {
		case "start":
			// Отправляем главное меню при старте бота или команде /start
			err := createCategoryMessage(bot, update, "Старт")
			if err != nil {
				logError("Ошибка создания сообщения с категорией update.Message.IsCommand() = ", err)
				return
			}
			// Другие команды...
		case "cabbage":
			//msg.Text = "Hi :)"
			//fmt.Println("cabbage              ------------------------------------------------")
		case "status":
		//msg.Text = "I'm ok."
		default:
			msg.ReplyToMessageID = update.Message.MessageID // Устанавливаем ID сообщения, на которое отвечаем
			if _, err := bot.Send(msg); err != nil {
				logError("Ошибка отправки сообщения:", err)
				return
			}
		}

	}
}

// handleMessageText Разбор callbackData для определения действия текстовых сообщений.
func handleMessageText(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Нет такой категории продуктов")

	if update.Message.Text != "" {

		// Обработчик нажатий на кнопки меню
		categoryName := update.Message.Text
		// Создаем сообщение с фото категории и встроенной клавиатурой
		_, pureText := SeparateEmojiAndText(categoryName)

		// Проверяем есть у нас такая категория если нет вызываем другое меню
		err := createCategoryMessage(bot, update, pureText)

		if err != nil {

			// Получаем и выводим поле DeliveryDate по chatID
			if field, err := getFieldByChatID(update.Message.Chat.ID, "FullName"); err == nil {

				if field == "ФИО:" {
					if pureText == "" || len(pureText) < 3 {
						//Удаляем предыдущие сообщения
						deletePreviousMessages(bot, update)
						str := "ФИО: не может быть пустым и имя не может быть короче 3 символов\n"
						FullName := "ФИО:"
						instructions := "Пожалуйста, введите " + FullName + "\n" +
							"Пример:\n" +
							"Иванов Иван Иванович\n"
						str += instructions
						//отправляем сообщение
						sendMessage(bot, update.Message.Chat.ID, str)
						return
					}
					err = addOrUpdateOrderField(update.Message.Chat.ID, "FullName", pureText)
					if err != nil {
						logError("Ошибка при добавлении или обновлении поля:", err)
						return
					}
					pureText = "Номер телефона:"
				}
			}

			// Получаем и выводим поле DeliveryDate по chatID
			if field, err := getFieldByChatID(update.Message.Chat.ID, "DeliveryDate"); err == nil {
				if field == "Дату доставки:" {
					formatted, str := formatDateString(pureText)
					if str != "" {
						//Удаляем предыдущие сообщения
						deletePreviousMessages(bot, update)
						//Получаем текущую дату и добавляем один день
						tomorrow := time.Now().AddDate(0, 0, 1)
						// Форматируем дату в строку
						tomorrowStr := tomorrow.Format("02.01.2006")
						DeliveryDate := "Дату доставки:"
						instructions := "\nПожалуйста, введите " + DeliveryDate + "\n" +
							"Пример:\n" +
							tomorrowStr + "\n"
						str += instructions
						//отправляем сообщение
						sendMessage(bot, update.Message.Chat.ID, str)
						return
					}

					err = addOrUpdateOrderField(update.Message.Chat.ID, "DeliveryDate", formatted)
					if err != nil {
						logError("Ошибка при добавлении или обновлении поля:", err)
						return
					}
					FullName := "ФИО:"
					instructions := "Пожалуйста, введите " + FullName + "\n" +
						"Пример:\n" +
						"Иванов Иван Иванович\n"

					err = addOrUpdateOrderField(update.Message.Chat.ID, "FullName", FullName)
					if err != nil {
						logError("Ошибка при добавлении или обновлении поля:", err)
					}

					//отправляем сообщение
					sendMessage(bot, update.Message.Chat.ID, instructions)
					pureText = "ok"
				}
			}

			// Получаем и выводим поле DeliveryAddress по chatID
			if field, err := getFieldByChatID(update.Message.Chat.ID, "DeliveryAddress"); err == nil {

				if field == "Адрес доставки:" {
					if pureText == "" {
						str := "адрес доставки не может быть пустым\n"
						DeliveryAddress := "Адрес доставки:"
						instructions := "Пожалуйста, введите " + DeliveryAddress + "\n" +
							"Пример:\n" +
							"г.Будущего ул. Пушкина, д. 12 кв 5\n"
						str += instructions
						//отправляем сообщение
						sendMessage(bot, update.Message.Chat.ID, str)
						return
					}
					err = addOrUpdateOrderField(update.Message.Chat.ID, "DeliveryAddress", pureText)
					if err != nil {
						logError("Ошибка при добавлении или обновлении поля:", err)
					}

					// Получаем текущую дату и добавляем один день
					tomorrow := time.Now().AddDate(0, 0, 1)
					// Форматируем дату в строку
					tomorrowStr := tomorrow.Format("02.01.2006")
					DeliveryDate := "Дату доставки:"
					instructions := "Пожалуйста, введите " + DeliveryDate + "\n" +
						"Пример:\n" +
						tomorrowStr + "\n"

					err = addOrUpdateOrderField(update.Message.Chat.ID, "DeliveryDate", DeliveryDate)
					if err != nil {
						logError("Ошибка при добавлении или обновлении поля:", err)
					}

					//отправляем сообщение
					sendMessage(bot, update.Message.Chat.ID, instructions)
					pureText = "ok"
				}
			}

			// Получаем и выводим поле DeliveryAddress по chatID
			if field, err := getFieldByChatID(update.Message.Chat.ID, "PhoneNumber"); err == nil {
				if field == "Номер телефона:" {
					formatted, str := formatPhoneNumber(pureText)
					if str != "" {
						// Формирование текста с просьбой ввести номер телефона
						PhoneNumber := "Номер телефона:"
						instructions := "\nПожалуйста, введите " + PhoneNumber + "\n" +
							"Пример:\n" +
							"+79345678901\n"
						str += instructions
						//отправляем сообщение
						sendMessage(bot, update.Message.Chat.ID, str)
						return
					}
					err = addOrUpdateOrderField(update.Message.Chat.ID, "PhoneNumber", formatted)
					if err != nil {
						logError("Ошибка при добавлении или обновлении поля: PhoneNumber ", err)
					}
					DeliveryAddress := "Адрес доставки:"
					instructions := "Пожалуйста, введите " + DeliveryAddress + "\n" +
						"Пример:\n" +
						"г.Будущего ул. Пушкина, д. 12 кв 5\n"

					err = addOrUpdateOrderField(update.Message.Chat.ID, "DeliveryAddress", DeliveryAddress)
					if err != nil {
						logError("Ошибка при добавлении или обновлении поля:", err)
					}

					//отправляем сообщение
					sendMessage(bot, update.Message.Chat.ID, instructions)
					pureText = "ok"
				}
			}

			switch pureText {
			// Проверяем, соответствует ли сообщение ожидаемому формату

			case "ok":
				break
			case "Номер телефона:":
				//deletePreviousMessages(bot, update)
				handleDeliveryDetailsMessage(bot, update)
			case "Мои заказы":
				//Выводим заказы пользователей
				MainOrdersByUserID(bot, update)
			case "Назад":
				// Возвращаемся в основное меню
				err = createCategoryMessage(bot, update, "Меню")
				if err != nil {
					log.Println("Ошибка создания сообщения с категорией update.Message.Text = ", err)
					return
				}
			// Другие сценарии...
			case "Корзина":
				ShowBasket(bot, update.Message.Chat.ID)
				messageID := update.Message.MessageID
				chatID := update.Message.Chat.ID
				deleteMsg := tgbotapi.NewDeleteMessage(chatID, messageID)
				if _, err := bot.Request(deleteMsg); err != nil {
					logError("Ошибка при удалении сообщения:", err)
				}
			default:
				msg.ReplyToMessageID = update.Message.MessageID // Устанавливаем ID сообщения, на которое отвечаем
				if _, err = bot.Send(msg); err != nil {
					log.Println("Ошибка отправки сообщения:", err)
					return
				}
			}
		}
		deletePreviousMessages(bot, update)
	}
}

// handleCallbackQueryData Разбор callbackData для определения действия.
func handleCallbackQueryData(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	if update.CallbackQuery != nil {
		// Разбор callbackData для определения действия
		callbackData := update.CallbackQuery.Data
		// Примеры callbackData: "add_1", "remove", "change_1_3" (изменить количество первого товара на 3)
		parts := strings.Split(callbackData, "_")

		action := parts[0]

		switch action {

		case "add":
			//deletePreviousMessages(bot, update.CallbackQuery.Message.Chat.ID)
			num1, err := strconv.Atoi(parts[2])
			if err != nil {
				logError("Ошибка преобразования числа 1:", err)
				return
			}

			num2, er := strconv.Atoi(parts[3])
			if er != nil {
				logError("Ошибка преобразования числа 2:", er)
				return
			}

			AddToBasket(bot, update, parts[1], num1, num2) // Добавление товара с указанием названия, количества и стоимости
		case "confirm":
			err := ProcessOrder(orderDetails, db, update.CallbackQuery.Message.Chat.ID)
			if err != nil {
				logError("Ошибка обработки заказа:", err)
				return
			} else {
				//fmt.Println("Заказ успешно обработан и добавлен в базу данных.")
				GroupMessagesChatID(bot, update)
				Baskets[update.CallbackQuery.Message.Chat.ID] = nil
			}

		case "send": // Получение содержимого корзины
			// Пример ввода данных заказа
			RequestDeliveryDetails(bot, update)

		case "delete":
			DeleteBasketContents(bot, update)
		}
	}
}

// createCategoryMessage Вывод изображения продуктов и кнопки фасовки продукта.
func createCategoryMessage(bot *tgbotapi.BotAPI, update tgbotapi.Update, categoryName string) error {
	chatID := update.Message.Chat.ID

	// Поиск категории в данных
	categoryData, found := Categories[categoryName]
	if !found {
		er := "категория не найдена в createCategoryMessage"
		return errors.New(er)
	}

	if categoryData.Photo == "" {
		keyboardReply, err := CreateCategoryKeyboardReply(categoryData.CategoryName)
		if err != nil {
			// Обработка ошибки
			logError("ошибка в CreateCategoryKeyboardReply", err)
			return err
		}

		// Создаем сообщение с инлайн-клавиатурой
		msg := tgbotapi.NewMessage(chatID, "Выбранная категория : "+categoryData.CategoryName)

		msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(keyboardReply...)

		// Отправляем сообщение с клавиатурой
		sentMsg, err := bot.Send(msg)
		if err != nil {
			logError("Ошибка при отправке сообщения:", err)
			return err
		} else {
			// Сохраняем идентификатор сообщения для последующего удаления
			messagesToDelete[chatID] = append(messagesToDelete[chatID], sentMsg.MessageID)
		}

	} else {
		// Создаем медиа-объект для изображения
		photo := tgbotapi.NewInputMediaPhoto(tgbotapi.FilePath(categoryData.Photo))

		// Создаем медиагруппу с изображением и URL
		mediaGroup := tgbotapi.NewMediaGroup(chatID, []interface{}{photo})

		// Отправляем медиагруппу
		if _, err := bot.SendMediaGroup(mediaGroup); err != nil {
			logError("Ошибка при отправке сообщения медиагруппу:", err)
			return err
		}

		// Создаем кнопки клавиатуры
		keyboard, err := CreateCategoryKeyboard(categoryData.CategoryName)
		if err != nil {
			// Обработка ошибки
			logError("Ошибка не удается создать кнопки клавиатуры", err)
			return err
		}

		// Создаем сообщение с инлайн-клавиатурой
		msg := tgbotapi.NewMessage(chatID, "Выберите нужное колличество продукта : "+categoryData.CategoryName)
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(keyboard...)

		// Отправляем сообщение с клавиатурой
		sentMsg, err := bot.Send(msg)
		if err != nil {
			logError("Ошибка при отправке сообщения:", err)
			return err
		} else {
			// Сохраняем идентификатор сообщения для последующего удаления
			messagesToDelete[chatID] = append(messagesToDelete[chatID], sentMsg.MessageID)
		}
	}
	deletePreviousMessages(bot, update)
	return nil
}

// SeparateEmojiAndText Паттерн для поиска эмотзи update.Message.Text.
func SeparateEmojiAndText(text string) (string, string) {
	// Паттерн для поиска эмотзи
	emojiPattern := regexp.MustCompile(`[\p{So}]`)
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
