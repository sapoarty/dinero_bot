package main

import (
	"database/sql"
	"encoding/json" // Для работы с JSON
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api" // Библиотека для работы с Telegram API
	_ "github.com/mattn/go-sqlite3"
	"io/ioutil"
	"log"
	"net/http" // Для создания HTTP запросов
	"strconv"  // Для преобразования строк в числа
	"strings"
)

type CurrencyRates struct {
	Rates map[string]float64 `json:"rates"` // Структура для хранения информации о курсах валют
}

type Account struct {
	Currency string
	Amount   float64
}

type GoldPrice struct {
	Price string `json:"price"`
}

type GoldApiResponse struct {
	Timestamp      int     `json:"timestamp"`
	Metal          string  `json:"metal"`
	Currency       string  `json:"currency"`
	Exchange       string  `json:"exchange"`
	Symbol         string  `json:"symbol"`
	PrevClosePrice float64 `json:"prev_close_price"`
	OpenPrice      float64 `json:"open_price"`
	LowPrice       float64 `json:"low_price"`
	HighPrice      float64 `json:"high_price"`
	OpenTime       int     `json:"open_time"`
	Price          float64 `json:"price"`
	Ch             float64 `json:"ch"`
	Chp            float64 `json:"chp"`
	Ask            float64 `json:"ask"`
	Bid            float64 `json:"bid"`
}

var accounts map[int64][]Account // Глобальная карта для хранения аккаунтов

func main() {
	fmt.Println("Бот запущен. Инициализация БД ...")
	db, err := InitDB("account.db")
	if err != nil {
		log.Fatal(err)
	}
	accounts = make(map[int64][]Account)                                             // Инициализация карты аккаунтов
	bot, err := tgbotapi.NewBotAPI(Token) // Создание нового бота
	if err != nil {
		log.Panic(err) // Если возникают ошибки, программа завершается
	}

	// Конфигурация бота
	bot.Debug = false
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	fmt.Println("Бот готов к работе, ждет команд.")
	updates, err := bot.GetUpdatesChan(u) // Получение обновлений от бота

	// Обработка обновлений
	for update := range updates {
		// Пропустить сообщения без команд
		if update.Message == nil {
			continue
		}

		// Определение типа команды
		switch update.Message.Command() {
		case "add":
			HandleAddAccCommand(bot, update, db)
		case "edit":
			HandleEditAccCommand(bot, update, db)
		case "getsum":
			HandleGetSumCommand(bot, update, db)
		case "addgold":
			HandleAddGoldCommand(bot, update, db)
		default:
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "I don't know that command")
			bot.Send(msg)
		}

	}
}

// Инициализация БД
func InitDB(dbName string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbName)
	if err != nil {
		return nil, err
	}

	// Создание таблицы аккаунтов, если ее еще не существует
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS accounts (id INTEGER PRIMARY KEY, chat_id INTEGER, currency TEXT, amount REAL)")
	if err != nil {
		return nil, err
	}

	// Создание таблицы с ценами на золото, если ее еще не существует
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS gold_prices (id INTEGER PRIMARY KEY, date_time DATETIME, oz_usd_price REAL)")
	if err != nil {
		return nil, err
	}
	return db, nil
}

// Обработка команды "add"
func HandleAddAccCommand(bot *tgbotapi.BotAPI, update tgbotapi.Update, db *sql.DB) {
	args := strings.Split(update.Message.CommandArguments(), " ")
	if len(args) < 2 {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Not enough parameters")
		bot.Send(msg) // Если недостаточно параметров, отправить сообщение об ошибке
		return
	}

	if args[0] == "GOLD" {
		HandleAddGoldCommand(bot, update, db)
		return
	}

	amount, err := strconv.ParseFloat(args[1], 64)
	if err != nil {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Invalid amount")
		bot.Send(msg) // Если параметр не является числом, отправить сообщение об ошибке
		return
	}

	// Используйте функцию AddTotalToAccount для добавления аккаунта в базу данных
	err = AddTotalToAccount(update.Message.Chat.ID, args[0], amount, db)
	if err != nil {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Failed to add account")
		bot.Send(msg) // Если произошла ошибка при добавлении аккаунта, отправить сообщение об ошибке
		return
	}

	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Account added")
	bot.Send(msg) // Отправить сообщение об успешном добавлении аккаунта
}

func HandleEditAccCommand(bot *tgbotapi.BotAPI, update tgbotapi.Update, db *sql.DB) {
	args := strings.Split(update.Message.CommandArguments(), " ")
	if len(args) < 2 {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Not enough parameters")
		bot.Send(msg)
		return
	}
	currency := args[0]
	newAmount, err := strconv.ParseFloat(args[1], 64)
	if err != nil {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Invalid amount")
		bot.Send(msg)
		return
	}

	chatID := update.Message.Chat.ID
	// Check if account with the given currency exists
	query := "SELECT COUNT(*) FROM accounts WHERE chat_id = ? AND currency = ?"
	row := db.QueryRow(query, chatID, currency)
	var accountCount int
	err = row.Scan(&accountCount)
	if err != nil {
		fmt.Println("Error checking account:", err)
		return
	}

	if accountCount > 0 {
		// Update existing account
		query = "UPDATE accounts SET amount = ? WHERE chat_id = ? AND currency = ?"
		_, err = db.Exec(query, newAmount, chatID, currency)
		if err != nil {
			fmt.Println("Error updating account:", err)
			return
		}

		msg := tgbotapi.NewMessage(chatID, "Account edited")
		bot.Send(msg)
	} else {
		msg := tgbotapi.NewMessage(chatID, "Account not found")
		bot.Send(msg)
	}
}

// Обработка команды "getsum"
func HandleGetSumCommand(bot *tgbotapi.BotAPI, update tgbotapi.Update, db *sql.DB) {
	chatID := update.Message.Chat.ID
	args := update.Message.CommandArguments()

	// Query the database to get the accounts for the chatID
	rows, err := db.Query("SELECT currency, amount FROM accounts WHERE chat_id=?", chatID)
	if err != nil {
		msg := tgbotapi.NewMessage(chatID, "Failed to get accounts")
		bot.Send(msg)
		return
	}
	defer rows.Close()

	accounts := make([]Account, 0)
	for rows.Next() {
		var acc Account
		err := rows.Scan(&acc.Currency, &acc.Amount)
		if err != nil {
			msg := tgbotapi.NewMessage(chatID, "Failed to read account data")
			bot.Send(msg)
			return
		}
		accounts = append(accounts, acc)
	}
	if err := rows.Err(); err != nil {
		msg := tgbotapi.NewMessage(chatID, "Failed to get account data")
		bot.Send(msg)
		return
	}

	var rates CurrencyRates
	// если аргументы для команды указаны, тогда получаем стоимость в указанной валюте
	if args != "" {
		res, err := http.Get("https://api.exchangerate-api.com/v4/latest/" + args)
		if err != nil {
			msg := tgbotapi.NewMessage(chatID, "Unsupported currency: "+args)
			bot.Send(msg)
			return
		}
		json.NewDecoder(res.Body).Decode(&rates)
	}

	msgText := ""
	var goldAmount float64
	var total float64
	for _, acc := range accounts {
		if acc.Currency == "GOLD" {
			goldAmount += acc.Amount
			continue
		}

		amount := acc.Amount
		if args != "" {
			// переведем стоимость валюты в необходимую
			rate, ok := rates.Rates[acc.Currency]
			if !ok {
				msg := tgbotapi.NewMessage(chatID, "Unsupported currency: "+acc.Currency)
				bot.Send(msg)
				return
			}
			amount = (acc.Amount / rate)
		}

		msgText += fmt.Sprintf("%s: %.2f %s\n", acc.Currency, amount, args)
		total += amount
	}

	// Подсчитываем золото
	if goldAmount > 0 {
		// Получаем стоимость в USD
		goldValue, err := GetGoldValue(db)
		if err != nil {
			msg := tgbotapi.NewMessage(chatID, "Failed to get gold price")
			bot.Send(msg)
			return
		}

		var amount float64
		if args != "" {
			// переведем в необходимую стоимость
			amount = goldAmount * goldValue / rates.Rates["USD"]
			msgText += fmt.Sprintf("GOLD: %.2f g. (%.2f %s)", goldAmount, amount, args) + "\n"
		} else {
			amount = goldAmount * goldValue
			msgText += fmt.Sprintf("GOLD: %.2f g. (%.2f$)", goldAmount, amount) + "\n"
		}
		total += amount
	}
        if args != "" {
                msgText += fmt.Sprintf("Total in %s: %.2f", args, total) + "\n"
        }
	msg := tgbotapi.NewMessage(chatID, msgText)
	bot.Send(msg)
}

func HandleAddGoldCommand(bot *tgbotapi.BotAPI, update tgbotapi.Update, db *sql.DB) {
	args := strings.Split(update.Message.CommandArguments(), " ")

	if len(args) < 1 {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Usage: amount_in_grams")
		bot.Send(msg)
		return
	}

	price, err := GetGoldValue(db)
	if err != nil {
		log.Printf("Failed to get gold price: %v", err)
		return
	}
	log.Printf("Price is: %v", price)

	log.Printf("Amount is %v", args[1])
	amount, err := strconv.ParseFloat(args[1], 64)
	if err != nil {
		log.Printf("Failed to parse gold amount: %v", err)
		return
	}

	totalValueInGrams := amount
	err = AddTotalToAccount(update.Message.Chat.ID, "GOLD", totalValueInGrams, db)
	if err != nil {
		log.Printf("Failed to add gold value to account: %v", err)
		return
	}

	totalValueInDollars := price * amount

	msg := tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("Added %.2f grams of gold. Total value: $%.2f", totalValueInGrams, totalValueInDollars))
	bot.Send(msg)
}

func AddTotalToAccount(chatID int64, currency string, amount float64, db *sql.DB) error {
	// Check if account with the given currency exists
	query := "SELECT amount FROM accounts WHERE chat_id = ? AND currency = ?"
	row := db.QueryRow(query, chatID, currency)
	var currentAmount float64
	err := row.Scan(&currentAmount)
	if err == nil {
		// Update existing account
		query = "UPDATE accounts SET amount = ? WHERE chat_id = ? AND currency = ?"
		_, err = db.Exec(query, currentAmount+amount, chatID, currency)
		if err != nil {
			return fmt.Errorf("Error updating account: %v", err)
		}
	} else if err == sql.ErrNoRows {
		// Add new account if the account with the given currency does not exist
		query = "INSERT INTO accounts (chat_id, currency, amount) VALUES (?, ?, ?)"
		_, err = db.Exec(query, chatID, currency, amount)
		if err != nil {
			return fmt.Errorf("Error adding account: %v", err)
		}
	} else {
		return fmt.Errorf("Error checking account: %v", err)
	}

	return nil
}

func GetGoldValue(db *sql.DB) (float64, error) {
	var grammInOz = 28.3495
	var coefToSell = 0.84
	req, err := http.NewRequest("GET", "https://www.goldapi.io/api/XAU/USD", nil)
	if err != nil {
		return 0, err
	}
	//req.Header.Set("x-access-token", "goldapi-qnorlp5jccye-io")
	req.Header.Set("x-access-token", "goldapi-4etusw4rlpecswtg-io")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	// Печать тела ответа
	//fmt.Println(string(body))

	var goldPrice GoldApiResponse
	err = json.Unmarshal(body, &goldPrice)
	if err != nil {
		return 0, err
	}
	if goldPrice.Price == 0 {
		fmt.Println("GOLD API error: %v. Trying to get from local history table", string(body))
		query := "select oz_usd_price from gold_prices order by date_time desc limit 1"
		row := db.QueryRow(query)
		err = row.Scan(&goldPrice.Price)
		if err != nil {
			fmt.Println("Error getting gold price from history table:", err)
		} else {
			fmt.Println("Last gold price have gotten from local history table - %v", goldPrice.Price)
		}
	} else {
		query := "INSERT INTO gold_prices (date_time, oz_usd_price) VALUES (datetime(), ?)"
		_, err = db.Exec(query, goldPrice.Price)
		if err != nil {
			fmt.Errorf("Error INSERT INTO gold_prices: %v", err)
		}
	}
	return goldPrice.Price / grammInOz * coefToSell, nil
}
