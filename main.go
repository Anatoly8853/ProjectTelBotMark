// main пакет.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
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
		fmt.Println(err)
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
		log.Fatal("Error loading .env file")
	}

	bot, err := tgbotapi.NewBotAPI(os.Getenv("TELEGRAM_TOKEN"))
	if err != nil {
		log.Panic(err)
	}

	// 1. Чтение файла JSON
	jsonData, er := os.ReadFile("data.json")
	if er != nil {
		fmt.Println("Ошибка чтения файла JSON:", er)
		return
	}

	// 2. Распаковка данных
	err = json.Unmarshal(jsonData, &Categories)
	if err != nil {
		fmt.Println("Ошибка распаковки JSON:", err)
		return
	}

	bot.Debug = false //false //true   // выводим лог или нет

	//log.Printf("Authorized on account %s", bot.Self.UserName)

	go UpdateMessage(bot)

	GinRouter()

}

// UpdateMessage Основной обработчик всех данных бота.
func UpdateMessage(bot *tgbotapi.BotAPI) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 3

	updates := bot.GetUpdatesChan(u)

	//var menuState MenuState = MainMenu

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

		// Здесь можно добавить обработку других типов обновлений, если потребуется

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
				log.Println("Ошибка создания сообщения с категорией update.Message.IsCommand() = ", err)
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
				log.Println("Ошибка отправки сообщения:", err)
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
		_, pureText := separateEmojiAndText(categoryName)

		// Проверяем есть у нас такая категория если нет вызываем другое меню
		err := createCategoryMessage(bot, update, pureText)

		if err != nil {
			//log.Println("Ошибка создания сообщения с категорией:", err)
			// Отправляем сообщение
			if strings.HasPrefix(pureText, "Номер телефона:") {
				pureText = "Номер телефона:"
			}
			switch pureText {
			// Проверяем, соответствует ли сообщение ожидаемому формату

			case "Номер телефона:":
				handleDeliveryDetailsMessage(bot, update)
				//deletePreviousMessages(bot, update)
			case "Мои заказы":
				userID := update.Message.Chat.ID // Ваш user_id
				orders, err := GetOrdersByUserID(db, userID)
				if err != nil {
					fmt.Println("Error getting orders:", err)
					return
				}
				if len(orders) > 0 {
					var msgText string
					var currentDeliveryDate string // Объявляем переменную для отслеживания текущей даты доставки
					var currentDateTime string

					for i, order := range orders {
						// Форматирование даты заказа
						// Парсинг строки времени в тип time.Time
						t, err := time.Parse("15:04 02.01.2006", order.DateTime)
						if err != nil {
							fmt.Println("Ошибка парсинга строки времени:", err)
							return
						}

						// Форматирование времени в нужный формат
						formattedDate := t.Format("02.01.2006")

						// Группировка по дате доставки
						if order.DeliveryDate != currentDeliveryDate || order.DateTime != currentDateTime {
							msgText += fmt.Sprintf("Дата заказа: %v\n", formattedDate)
							msgText += fmt.Sprintf("Дата доставки: %v\n", order.DeliveryDate)
							msgText += fmt.Sprintf("Заказчик: %v\n", order.FullName)
							msgText += fmt.Sprintf("Телефон для связи: %v\n", order.PhoneNumber)
							msgText += fmt.Sprintf("Адрес доставки: %v\n", order.DeliveryAddress)
							msgText += "Список заказанных товаров :\n"
							currentDeliveryDate = order.DeliveryDate
							currentDateTime = order.DateTime
						}

						msgText += fmt.Sprintf("%v кол-во: %v цена: %v руб.\n", order.ItemName, order.Quantity, order.Cost)

						if i == len(orders)-1 || (i+1 < len(orders) && orders[i+1].DateTime != currentDateTime) {
							msgText += fmt.Sprintf("Общая стоимость:  %v ркб.\n", order.TotalCost)
							msgText += "------------------------------------------------------------\n"

						}
					}

					msgs := tgbotapi.NewMessage(update.Message.Chat.ID, msgText)
					if _, err = bot.Send(msgs); err != nil {
						log.Println("Ошибка отправки сообщения:", err)
					}
				} else {
					msgs := tgbotapi.NewMessage(update.Message.Chat.ID, "Вы ничего у нас не заказывали")
					if _, err = bot.Send(msgs); err != nil {
						log.Println("Ошибка отправки сообщения:", err)
					}
				}

			case "Назад":
				// Возвращаемся в основное меню
				err = createCategoryMessage(bot, update, "Меню")
				if err != nil {
					log.Println("Ошибка создания сообщения с категорией update.Message.Text = ", err)
				}
			// Другие сценарии...
			case "Корзина":
				showBasket(bot, update.Message.Chat.ID, 2)
				messageID := update.Message.MessageID
				chatID := update.Message.Chat.ID
				deleteMsg := tgbotapi.NewDeleteMessage(chatID, messageID)
				if _, err := bot.Request(deleteMsg); err != nil {
					log.Printf("Ошибка при удалении сообщения: %v", err)
					// Не используйте log.Panic, чтобы избежать прерывания работы бота из-за одной ошибки
				}
			default:
				msg.ReplyToMessageID = update.Message.MessageID // Устанавливаем ID сообщения, на которое отвечаем
				if _, err = bot.Send(msg); err != nil {
					log.Println("Ошибка отправки сообщения:", err)
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
				fmt.Println("Ошибка преобразования числа 1:", err)
				return
			}

			num2, er := strconv.Atoi(parts[3])
			if er != nil {
				fmt.Println("Ошибка преобразования числа 2:", er)
				return
			}

			addToBasket(bot, update, parts[1], num1, num2) // Добавление товара с указанием названия, количества и стоимости
		case "confirm":
			err := ProcessOrder(orderDetails, db, update.CallbackQuery.Message.Chat.ID)
			if err != nil {
				fmt.Println("Ошибка обработки заказа -------------------------------------------------------:", err)
				return
			} else {
				//fmt.Println("Заказ успешно обработан и добавлен в базу данных.")
				groupMessagesChatID(bot, update)
				baskets[update.CallbackQuery.Message.Chat.ID] = nil
			}

		case "send": // Получение содержимого корзины
			// Пример ввода данных заказа
			requestDeliveryDetails(bot, update)

		case "delete":
			deleteBasketContents(bot, update)
		}
	}
}

// createCategoryMessage Вывод изображения продуктов и кнопки фасовки продукта.
func createCategoryMessage(bot *tgbotapi.BotAPI, update tgbotapi.Update, categoryName string) error {
	chatID := update.Message.Chat.ID

	// Поиск категории в данных
	categoryData, found := Categories[categoryName]
	if !found {
		return errors.New("категория не найдена в createCategoryMessage")
	}

	if categoryData.Photo == "" {
		keyboardReply, err := CreateCategoryKeyboardReply(categoryData.CategoryName)
		if err != nil {
			// Обработка ошибки
			return fmt.Errorf("ошибка в CreateCategoryKeyboardReply %s:", err)
		}

		// Создаем сообщение с инлайн-клавиатурой
		msg := tgbotapi.NewMessage(chatID, "Выбранная категория : "+categoryData.CategoryName)

		msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(keyboardReply...)

		// Отправляем сообщение с клавиатурой
		sentMsg, err := bot.Send(msg)
		if err != nil {
			log.Printf("Ошибка при отправке сообщения: %v", err)
			return err
		} else {
			// Сохраняем идентификатор сообщения для последующего удаления
			messagesToDelete[chatID] = append(messagesToDelete[chatID], sentMsg.MessageID)
		}
	} else {
		// Создаем медиа-объект для изображения
		photo := tgbotapi.NewInputMediaPhoto(tgbotapi.FilePath(categoryData.Photo))
		//msgPhoto := tgbotapi.NewMessage(update.Message.Chat.ID, "")

		// Создаем медиагруппу с изображением и URL
		mediaGroup := tgbotapi.NewMediaGroup(chatID, []interface{}{photo})

		// Отправляем медиагруппу
		if _, err := bot.SendMediaGroup(mediaGroup); err != nil {
			log.Println("Error sending media group:", err)
			return err
		}

		// Создаем кнопки клавиатуры
		keyboard, err := CreateCategoryKeyboard(categoryData.CategoryName)
		if err != nil {
			// Обработка ошибки
			return err
		}

		// Создаем сообщение с инлайн-клавиатурой
		msg := tgbotapi.NewMessage(chatID, "Выберите нужное колличество продукта : "+categoryData.CategoryName)
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(keyboard...)

		// Отправляем сообщение с клавиатурой
		sentMsg, err := bot.Send(msg)
		if err != nil {
			log.Printf("Ошибка при отправке сообщения: %v", err)
			return err
		} else {
			// Сохраняем идентификатор сообщения для последующего удаления
			messagesToDelete[chatID] = append(messagesToDelete[chatID], sentMsg.MessageID)
		}
	}
	deletePreviousMessages(bot, update)
	return nil

}

// separateEmojiAndText Паттерн для поиска эмотзи update.Message.Text.
func separateEmojiAndText(text string) (string, string) {
	// Паттерн для поиска эмотзи
	emojiPattern := regexp.MustCompile(`[\p{So}]`)
	// Ищем все эмотзи в тексте
	emojis := emojiPattern.FindAllString(text, -1)

	// Удаляем эмотзи из исходного текста
	cleanText := emojiPattern.ReplaceAllString(text, "")
	// Удаляем лишние пробелы
	cleanText = regexp.MustCompile(`\s+`).ReplaceAllString(cleanText, " ")

	return joinStrings(emojis), cleanText
}

// joinStrings Объединяем все строки из среза в одну строку из update.Message.Text.
func joinStrings(strs []string) string {
	// Объединяем все строки из среза в одну строку
	result := ""
	for _, str := range strs {
		result += str
	}
	return result
}
