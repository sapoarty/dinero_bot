package main

import (
	"database/sql"
	"encoding/json" // Для работы с JSON
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api" // Библиотека для работы с Telegram API
	_ "github.com/mattn/go-sqlite3"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
)

type CurrencyRates struct {
	Rates map[string]float64 `json:"rates"`
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

var accounts map[int64][]Account // Глобальная переменная для хранения аккаунтов

var currencies = []string{"USD", "EUR", "JPY", "GBP", "AUD", "CAD", "CHF", "CNY", "SEK", "NZD", "MXN", "SGD", "HKD", "NOK", "KRW", "TRY", "INR", "RUB", "BRL", "ZAR"}

func main() {
	fmt.Println("Бот запущен. Инициализация БД ...")
	db, err := InitDB("account.db")
	if err != nil {
		log.Fatal(err)
	}
	accounts = make(map[int64][]Account)     // Инициализация аккаунтов
	bot, err := tgbotapi.NewBotAPI(botToken) // Создание нового бота
	if err != nil {
		log.Panic(err) // Если возникают ошибки, программа завершается
	}

	// Конфигурация бота
	bot.Debug = false
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	fmt.Println("Бот готов к работе, ждет команд.")
	updates, err := bot.GetUpdatesChan(u) // Получение сообщений от бота

	// Обработка сообщений
	var prevUpdate tgbotapi.Update
	for update := range updates {
		var cur string
		if update.CallbackQuery != nil {
			var cbQuery = update.CallbackQuery
			fmt.Println(cbQuery.Data)
			if strings.HasPrefix(cbQuery.Data, "/command getsum ") {
				cur = strings.TrimPrefix(cbQuery.Data, "/command getsum ")
				HandleGetSumCommand(bot, update, db, cur)
				startBotMenu(bot, update)
			}
			if strings.HasPrefix(cbQuery.Data, "/command edit ") {
				cur = strings.TrimPrefix(cbQuery.Data, "/command edit ")
				fmt.Println(cbQuery.Data)
				msg := tgbotapi.NewMessage(cbQuery.Message.Chat.ID, "Введите новую сумму для "+cur)
				msg.ReplyMarkup = tgbotapi.ForceReply{
					ForceReply: true,
					Selective:  true,
				}
				prevUpdate = update
				bot.Send(msg)
			}
			if strings.HasPrefix(cbQuery.Data, "/command add ") {
				cur = strings.TrimPrefix(cbQuery.Data, "/command add ")
				msg := tgbotapi.NewMessage(cbQuery.Message.Chat.ID, "Введите сумму для новго счета в "+cur)
				msg.ReplyMarkup = tgbotapi.ForceReply{
					ForceReply: true,
					Selective:  true,
				}
				prevUpdate = update
				bot.Send(msg)
			}
		} else if update.Message != nil && update.Message.ReplyToMessage != nil {
			newAmount := update.Message.Text
			if strings.HasPrefix(prevUpdate.CallbackQuery.Data, "/command edit ") {
				cur = strings.TrimPrefix(prevUpdate.CallbackQuery.Data, "/command edit ")
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Новая сумма - "+newAmount+" для валюты - "+cur))
				HandleEditAccCommand(bot, update, db, cur, newAmount)
				startBotMenu(bot, update)
			} else if strings.HasPrefix(prevUpdate.CallbackQuery.Data, "/command add ") {
				cur = strings.TrimPrefix(prevUpdate.CallbackQuery.Data, "/command add ")
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Cумма - "+newAmount+" для новой валюты - "+cur))
				HandleAddAccCommand(bot, update, db, cur, newAmount)
				startBotMenu(bot, update)
			}
		}
		// Пропустить сообщения без команд
		if update.Message == nil {
			continue
		}

		// Определение типа команды
		var txt string
		if update.Message.Command() == "" {
			txt = update.Message.Text
		} else {
			txt = update.Message.Command()
		}
		switch txt {
		case "start":
			startBotMenu(bot, update)
		case "add":
			if update.Message.Command() == "" {
				showCurrentsList(bot, update, db, txt)
			} else {
				HandleAddAccCommand(bot, update, db)
			}
		case "edit":
			if update.Message.Command() == "" {
				showCurrentsList(bot, update, db, txt)
			} else {
				HandleEditAccCommand(bot, update, db)
			}
		case "getsum":
			HandleGetSumCommand(bot, update, db)
			if update.Message.Command() == "" {
				showCurrentsList(bot, update, db, txt)
			}
		}
	}
}

var isFirstMessage = true

func startBotMenu(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	// Создание меню бота с помощью кнопок
	var keyboard = tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("add"),
			tgbotapi.NewKeyboardButton("edit"),
			tgbotapi.NewKeyboardButton("getsum"),
		))
	var chatID int64
	if (update.Message != nil) {
                chatID = update.Message.Chat.ID
	} else {
                chatID = update.CallbackQuery.Message.Chat.ID
	}
	var msg = tgbotapi.NewMessage(chatID,"Пожалуйста, выберите действие из меню")

	if isFirstMessage {
		msg = tgbotapi.NewMessage(update.Message.Chat.ID,
			`Этот бот поможет вам управлять вашими счетами в различных валютах.

    Вот, что вы можете сделать:

    - "add": Добавить новый счет.
    - "edit": Изменить существующий счет.
    - "getsum": Получить суммарную информацию по всем счетам.

    Просто нажмите на соответствующую кнопку ниже, чтобы начать:`)
	isFirstMessage = false 
	}
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}

func showCurrentsList(bot *tgbotapi.BotAPI, update tgbotapi.Update, db *sql.DB, commandType string) {
	var query string
	query = "SELECT currency FROM accounts WHERE chat_id = ? AND currency != 'GOLD'"
	if commandType == "add" {
		// Build the SQL query
		baseQuery := "SELECT currency FROM ( "
		unionQueries := make([]string, len(currencies))

		for i, currency := range currencies {
			unionQueries[i] = fmt.Sprintf("SELECT '%s' AS currency ", currency)
		}

		query = baseQuery + strings.Join(unionQueries, " UNION ALL ") + " ) AS c WHERE currency NOT IN ( SELECT currency FROM accounts WHERE chat_id = ? )"
	}

	rows, err := db.Query(query, update.Message.Chat.ID)
	if err != nil {
		log.Println("Error querying accounts:", err)
		return
	}
	defer rows.Close()

	var accounts []Account
	for rows.Next() {
		var account Account
		err := rows.Scan(&account.Currency)
		if err != nil {
			log.Println("Error scanning account:", err)
			continue
		}
		accounts = append(accounts, account)
	}
	if err := rows.Err(); err != nil {
		log.Println("Error iterating accounts:", err)
		return
	}

	var rowsOfButtons [][]tgbotapi.InlineKeyboardButton
	for _, account := range accounts {
		command := commandType + " " + account.Currency
		callbackData := "/command " + command
		row := []tgbotapi.InlineKeyboardButton{tgbotapi.NewInlineKeyboardButtonData(account.Currency, callbackData)}
		rowsOfButtons = append(rowsOfButtons, row)
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(rowsOfButtons...)

	var msgTxt string
	switch commandType {
	case "add":
		msgTxt = "Выберите счет для добавления:"
	case "edit":
		msgTxt = "Выберите счет для изменения:"
	case "getsum":
		msgTxt = "Выберите валюту для отображения данных:"
	default:
		msgTxt = "Ошибка выбора команды"
	}

	msg := tgbotapi.NewMessage(update.Message.Chat.ID, msgTxt)
	msg.ReplyMarkup = keyboard

	bot.Send(msg)
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
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS gold_prices (id INTEGER PRIMARY KEY, date DATE, oz_usd_price REAL)")
	if err != nil {
		return nil, err
	}
	return db, nil
}

// Обработка команды "add"
func HandleAddAccCommand(bot *tgbotapi.BotAPI, update tgbotapi.Update, db *sql.DB, args ...string) {
	//args := strings.Split(update.Message.CommandArguments(), " ")
	if len(args) < 2 {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Please insert current name (RUB, USD, EUR, GOLD ...)")
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

func HandleEditAccCommand(bot *tgbotapi.BotAPI, update tgbotapi.Update, db *sql.DB, args ...string) {
	var chatID int64
	var currency string
	var newAmount float64

	if update.Message != nil {
		chatID = update.Message.Chat.ID
		currency = update.Message.CommandArguments()
		if currency == "" {
			currency = args[0]
		}
	} else {
		chatID = update.CallbackQuery.Message.Chat.ID
		currency = args[0]
	}

	if len(args) < 2 {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Not enough parameters")
		bot.Send(msg)
		return
	}

	newAmount, err := strconv.ParseFloat(args[1], 64)
	if err != nil {
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Invalid amount")
		bot.Send(msg)
		return
	}

	if amount, err := GetAccountBalance(db, chatID, currency); err != nil {
		fmt.Println("Ошибка получения суммы счета:", err)
	} else {
		if amount != -1 {
			// Обновление существующего счета
			query := "UPDATE accounts SET amount = ? WHERE chat_id = ? AND currency = ?"
			_, err = db.Exec(query, newAmount, chatID, currency)
			if err != nil {
				fmt.Println("Ошибка обновления счета:", err)
				return
			}

			msg := tgbotapi.NewMessage(chatID, "Счет изменен")
			bot.Send(msg)
		} else {
			msg := tgbotapi.NewMessage(chatID, "Счет не найден")
			bot.Send(msg)
		}
	}
}

func GetAccountBalance(db *sql.DB, chatID int64, currency string) (int, error) {
	query := "SELECT amount FROM accounts WHERE chat_id = ? AND currency = ?"
	row := db.QueryRow(query, chatID, currency)
	var accountBalance int
	err := row.Scan(&accountBalance)
	if err != nil {
		if err == sql.ErrNoRows {
			return -1, nil // Здесь используем -1 для обозначения отсутствующего счета
		} else {
			return 0, err
		}
	}
	return accountBalance, nil
}

// Обработка команды "getsum"
func HandleGetSumCommand(bot *tgbotapi.BotAPI, update tgbotapi.Update, db *sql.DB, args ...string) {
	var chatID int64
	var currency string

	if update.Message != nil {
		chatID = update.Message.Chat.ID
		currency = update.Message.CommandArguments()
	} else {
		chatID = update.CallbackQuery.Message.Chat.ID
		currency = args[0]
	}

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
	if currency != "" {
		res, err := http.Get("https://api.exchangerate-api.com/v4/latest/" + currency)
		if err != nil {
			msg := tgbotapi.NewMessage(chatID, "Unsupported currency: "+currency)
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
		if currency != "" {
			// переведем стоимость валюты в необходимую
			rate, ok := rates.Rates[acc.Currency]
			if !ok {
				msg := tgbotapi.NewMessage(chatID, "Unsupported currency: "+acc.Currency)
				bot.Send(msg)
				return
			}
			amount = (acc.Amount / rate)
		}

		msgText += fmt.Sprintf("%s: %.2f %s\n", acc.Currency, amount, currency)
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
		if currency != "" {
			// переведем в необходимую стоимость
			amount = goldAmount * goldValue / rates.Rates["USD"]
			msgText += fmt.Sprintf("GOLD: %.2f g. (%.2f %s)", goldAmount, amount, currency) + "\n"
		} else {
			amount = goldAmount * goldValue
			msgText += fmt.Sprintf("GOLD: %.2f g. (%.2f$)", goldAmount, amount) + "\n"
		}
		total += amount
	}
	if currency != "" {
		msgText += fmt.Sprintf("Total in %s: %.2f", currency, total) + "\n"
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

func GetGoldValue(db *sql.DB, args ...int) (float64, error) {
	var grammInOz = 28.3495
	var coefToSell = 0.84
	var goldPrice GoldApiResponse
	var goldApiNumber int

	// Если передан
	if len(args) > 0 {
		goldApiNumber = args[0]
	} else {
		goldApiNumber = 0
	}

	query := "select oz_usd_price from gold_prices where date = date() limit 1"
	row := db.QueryRow(query)
	var err = row.Scan(&goldPrice.Price)
	if err != nil {
		fmt.Println("Not yet saved gold prices for today in local history table. Creating reuest to API...")
	} else {
		fmt.Println("Last TODAY's gold price have gotten from local history table - ", goldPrice.Price)
		return goldPrice.Price / grammInOz * coefToSell, nil
	}

	req, err := http.NewRequest("GET", "https://www.goldapi.io/api/XAU/USD", nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("x-access-token", goldApiToken[goldApiNumber])
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

	err = json.Unmarshal(body, &goldPrice)
	if err != nil {
		return 0, err
	}
	if goldPrice.Price == 0 {
		var lastDate string
		goldApiNumber++
		if goldApiNumber < len(goldApiToken) {
			fmt.Println(fmt.Sprintf("GOLD API error: %v. Trying to get from the next [%v] token", string(body), goldApiNumber))
			GetGoldValue(db, goldApiNumber)
		}
		fmt.Println(fmt.Sprintf("GOLD API error: %v try unsuccessful. Trying to get from local history table", goldApiNumber))
		query := "select oz_usd_price, date from gold_prices order by date desc limit 1"
		row := db.QueryRow(query)
		err = row.Scan(&goldPrice.Price, &lastDate)
		if err != nil {
			fmt.Println(fmt.Sprintf("Error getting gold price from history table:", err))
		} else {
			fmt.Println(fmt.Sprintf("Last gold price have from %v gotten from local history table - %v", lastDate, goldPrice.Price))
		}
	} else {
		query := "INSERT INTO gold_prices (date, oz_usd_price) VALUES (date(), ?)"
		_, err = db.Exec(query, goldPrice.Price)
		if err != nil {
			fmt.Errorf("Error INSERT INTO gold_prices: %v", err)
		} else {
			fmt.Println(fmt.Sprintf("Gold price have goten from API - %v", goldPrice.Price))
		}
	}
	return goldPrice.Price / grammInOz * coefToSell, nil
}
