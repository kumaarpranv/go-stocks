package main

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/shopspring/decimal"
	"github.com/tidwall/gjson"
)

var email string
var password string

func main() {
	// channel to receive data from the source
	dataChan := make(chan StockData)
	err := godotenv.Load("creds.env")
	if err != nil {
		log.Fatalf("Some error occured. Err: %s", err)
	}
	email = os.Getenv("EMAIL")
	password = os.Getenv("PASSWORD")

	// goroutine to read data from the source and send it to the channel
	go func() {
		for {
			data, err := readDataFromSource()
			if err != nil {
				fmt.Println(err)
				continue
			}

			dataChan <- data
		}
	}()

	// goroutine to process the data as it comes in
	go func() {
		for {
			data := <-dataChan

			_, err := processData(data)
			if err != nil {
				fmt.Println(err)
				continue
			}

			// err = sendDataDownstream(processedData)
			// if err != nil {
			// 	fmt.Println(err)
			// 	continue
			// }
		}
	}()

	for {
		time.Sleep(time.Second)
	}
}

type StockData struct {
	Ticker string
	Price  decimal.Decimal
}

func readDataFromSource() (StockData, error) {
	client := NewYahooFinanceClient()
	ticker := "AAPL"
	price, err := client.GetLatestPrice(ticker)
	if err != nil {
		return StockData{}, err
	}

	return StockData{Ticker: ticker, Price: price}, nil
}

func processData(data StockData) (string, error) {
	previousPrice := getPreviousPrice(data.Ticker)
	change := decimal.New(0, 1)
	if !previousPrice.IsZero() {
		change = data.Price.Sub(previousPrice).Div(previousPrice).Mul(decimal.NewFromInt(100))
	} else {
		change = data.Price.Sub(previousPrice).Div(decimal.New(1, 1)).Mul(decimal.NewFromInt(100))
	}
	file, _ := os.OpenFile("prices.csv", os.O_RDWR|os.O_CREATE, 0644)
	updateCell(file, data.Ticker, data.Price)
	if change.Abs().GreaterThan(decimal.NewFromFloat(5)) {
		fmt.Println(change, "223232")
		subject := "Stock Alert: " + data.Ticker
		body := fmt.Sprintf("The price of %s has changed by %s%% compared to the previous price.", data.Ticker, change.String())
		status := sendEmail(email, email, subject, body)

		if status != nil {
			fmt.Println("err: ", status)
			time.Sleep(2 * time.Second)
			return "", status
		}
	}

	return "", nil
}

// sendDataDownstream sends the processed data to downstream consumers.
func sendDataDownstream(data string) error {
	return nil
}

func sendEmail(from, to, subject, body string) error {
	// Connect to the Gmail SMTP server
	c, err := smtp.Dial("smtp.gmail.com:587")
	if err != nil {
		return err
	}
	defer c.Close()

	// Upgrade the connection to a secure one
	if err := c.StartTLS(&tls.Config{InsecureSkipVerify: true}); err != nil {
		return err
	}
	auth := smtp.PlainAuth("", email, password, "smtp.gmail.com")
	if err := c.Auth(auth); err != nil {
		return err
	}

	// Set the sender and recipient
	if err := c.Mail(from); err != nil {
		return err
	}
	if err := c.Rcpt(to); err != nil {
		return err
	}

	// Set the message
	wc, err := c.Data()
	if err != nil {
		return err
	}
	defer wc.Close()
	_, err = fmt.Fprintf(wc, "Subject: %s\r\n", subject)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(wc, "Content-Type: text/plain; charset=utf-8\r\n")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(wc, "MIME-Version: 1.0\r\n")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(wc, "Content-Transfer-Encoding: base64\r\n\r\n")
	if err != nil {
		return err
	}
	_, err = wc.Write([]byte(base64.StdEncoding.EncodeToString([]byte(body))))
	if err != nil {
		return err
	}

	// Send the message
	status := c.Quit()
	if status != nil {
		return status
	}

	return nil
}

type YahooFinanceClient struct{}

func NewYahooFinanceClient() *YahooFinanceClient {
	return &YahooFinanceClient{}
}

// GetLatestPrice gets the latest price for the given stock ticker.
func (c *YahooFinanceClient) GetLatestPrice(ticker string) (decimal.Decimal, error) {
	url := fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/chart/%s?range=1d&interval=1m", ticker)
	res, err := http.Get(url)
	if err != nil {
		return decimal.Decimal{}, err
	}
	defer res.Body.Close()

	var data map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
		return decimal.Decimal{}, err
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return decimal.Decimal{}, err
	}

	priceStr := gjson.Get(string(jsonData), "chart.result.0.meta.regularMarketPrice").String()
	price, err := decimal.NewFromString(priceStr)
	if err != nil {
		return decimal.Decimal{}, err
	}

	return price, nil
}

func getPreviousPrice(ticker string) decimal.Decimal {
	file, err := os.OpenFile("prices.csv", os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		fmt.Println(err)
		updateCell(file, ticker, decimal.NewFromInt(0))
		return decimal.NewFromInt(0)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		fmt.Println(err)
		updateCell(file, ticker, decimal.NewFromInt(0))
		return decimal.NewFromInt(0)
	}

	var previousPrice decimal.Decimal
	var noRecord bool = false
	for _, record := range records {
		if record[0] == ticker {
			previousPrice, err = decimal.NewFromString(record[1])
			noRecord = true
			if err != nil {
				fmt.Println(err)
				return decimal.NewFromInt(0)
			}
			break
		}
	}
	if !noRecord {
		updateCell(file, ticker, decimal.NewFromInt(0))
	}
	return previousPrice
}

func updateCell(file *os.File, ticker string, price decimal.Decimal) {
	w := csv.NewWriter(file)
	defer w.Flush()

	records := [][]string{
		{"ticker", "price"},
		{ticker, price.String()},
	}

	for _, record := range records {
		if err := w.Write(record); err != nil {
			log.Fatalln("error writing record to file", err)
		}
	}

}
