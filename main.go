package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/kelseyhightower/envconfig"
	_ "github.com/lib/pq"
)

type Coin struct {
	Last24hChangeRate     float32 `json:"cap24hrChange"`
	Name                  string  `json:"long"`
	MarketCaptialization  float32 `json:"mktcap"`
	Percentage            float32 `json:"perc"`
	Price                 float32 `json:"price"`
	AvailableOnShapeShift bool    `json:"shapeshift"`
	Symbol                string  `json:"short"`
	Supply                int     `json:"supply"`
	USDVolume             float32 `json:"usdVolume"`
	Volume                float32 `json:"volume"`
	VwapData              float32 `json:"vwapData"`
	VwapDataBTC           float32 `json:"vwapDataBTC"`
}

type Specification struct {
	DBHost              string `default:"localhost"`
	DBPort              int    `default:"5432"`
	DBUser              string `default:"postgres"`
	DBPassword          string `default:"postgres"`
	DBName              string `default:"public"`
	Version             bool   `default:"false"`
	DBSSLMode           string `default:"disable"`
	DBConnectionRetries int    `default:"5"`
	DBConnectionBackoff int    `default:"3"`
}

var schema = `
CREATE TABLE exchange_rates (
	id varchar(24) PRIMARY KEY,
    symbol varchar(10) NOT NULL,
	name varchar(30) NOT NULL,
	price real NOT NULL,
	volume real,
	supply bigint,
	percentage real,
    timestamp bigint
);`

var coinCapClient = &http.Client{Timeout: 10 * time.Second}

func getJSON(url string, target interface{}) error {
	r, err := coinCapClient.Get(url)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(target)
}

func retry(attempts int, sleep time.Duration, callback func() error) (err error) {
	for i := 0; ; i++ {
		fmt.Printf(" ** Connect to db (%d) ...: \n", (i + 1))

		err = callback()
		if err == nil {
			return
		}

		if i >= (attempts - 1) {
			break
		}

		time.Sleep(sleep)
		fmt.Printf(" ** Error during establising db connection in %d. try: ", err)
	}
	return fmt.Errorf("After %d attempts, last error: %s", attempts, err)
}

func connectToDatabase(host string, port int, user string, password string, dbname string, sslmode string) (*sql.DB, error) {
	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s", host, port, user, password, dbname, sslmode)
	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		fmt.Println(" *** sql.Open(...) - Error: ", err)
		return db, err
	}
	fmt.Println(" *** sql.Open(...) - Success!")

	err = db.Ping()
	if err != nil {
		fmt.Println(" *** sql.Ping(...) - Error: ", err)
		return db, err
	}
	fmt.Println(" *** sql.Ping(...) - Success!")
	return db, err
}

func initializeDBConnection(host string, port int, user string, password string, dbname string, sslmode string, retries int, backoff int) {
	fmt.Printf(" * Initializing DB Connection (Retries: %d, Backoff: %d)\n", retries, backoff)
	err := retry(retries, time.Duration(backoff)*time.Second, func() (err error) {
		db, err = connectToDatabase(host, port, user, password, dbname, sslmode)
		return
	})
	if err != nil {
		fmt.Println(" * => Initialize DB Connection unsuccessful! Error: ", err)
	} else {
		fmt.Println(" * => Initialize DB Connection successful!")
	}
}

func initializeDBSchemas() {
	if _, err := db.Exec(schema); err != nil {
		fmt.Printf(" ** Schema Creation skipped: %v \n", err)
	} else {
		fmt.Printf(" ** Schema Creation successful!\n")
	}
}

func doEvery(d time.Duration, f func()) {
	fmt.Printf(" ** doEvery: %v", d)
	for _ = range time.Tick(d) {
		f()
	}
}

func crawlCoinCap() {
	var coins []Coin
	start := time.Now()
	fmt.Printf(" * Start Crawling CoinCap (%s)...  \n", start)

	err := getJSON("http://coincap.io/front", &coins)
	if err != nil {
		log.Fatal(err)
	}

	sort.Slice(coins, func(i, j int) bool {
		return (strings.Compare(coins[i].Symbol, coins[j].Symbol) < 1)
	})
	t := time.Now()
	elapsed := t.Sub(start)

	fmt.Printf(" * Processing '%d' coins took: %v ...\n", len(coins), elapsed)

	//insert exchange rates
	ts := start.Format("200601021504")
	fmt.Printf(" * Inserting '%d' coin exchange rate datapoints ...\n", len(coins))

	valueStrings := make([]string, 0, len(coins))
	valueArgs := make([]interface{}, 0, len(coins)*8)
	i := 1
	start = time.Now()
	for _, coin := range coins {
		id := fmt.Sprintf("%s_%s", ts, coin.Symbol)

		valueStrings = append(valueStrings, fmt.Sprintf("($%d, $%d, $%d,$%d, $%d, $%d,$%d, $%d)", i, i+1, i+2, i+3, i+4, i+5, i+6, i+7))
		i = i + 8
		valueArgs = append(valueArgs, id)
		valueArgs = append(valueArgs, coin.Symbol)
		valueArgs = append(valueArgs, coin.Name)
		valueArgs = append(valueArgs, coin.Price)
		valueArgs = append(valueArgs, coin.Volume)
		valueArgs = append(valueArgs, coin.Supply)
		valueArgs = append(valueArgs, coin.Percentage)
		valueArgs = append(valueArgs, ts)
	}
	query := fmt.Sprintf("INSERT INTO exchange_rates(id, symbol, name, price, volume, supply, percentage, timestamp) VALUES %s", strings.Join(valueStrings, ","))
	_, err = db.Exec(query, valueArgs...)
	t = time.Now()
	elapsed = t.Sub(start)

	if err != nil {
		fmt.Printf(" * Adding new exchange rates skipped! Processing time: %v, Error Message: %v\n", elapsed, err)
	} else {
		fmt.Printf(" * Adding %d exchange rates processed in %s ...!\n", len(coins), elapsed)
	}
}

//db check schema
func crawlExchangeRates(interval int) {
	if interval > 0 {
		intervalDuration := time.Duration(interval) * time.Second
		fmt.Printf("=> Scheduling crawling CoinCap every %v!\n", intervalDuration)
		doEvery(intervalDuration, crawlCoinCap)
	} else {
		fmt.Printf("=> Crawling CoinCap once!\n")
		crawlCoinCap()
	}
}

var db *sql.DB
var s Specification

func main() {

	//env vars
	err := envconfig.Process("ccrawler", &s)
	if err != nil {
		log.Fatal(err.Error())
	}

	format := "Used Config Values:\n - Host: %s\n - Port: %d\n - User: %s\n - Password: %s\n - DBName: %s\n - SSLMode: %s\n"
	_, err = fmt.Printf(format, s.DBHost, s.DBPort, s.DBUser, s.DBPassword, s.DBName, s.DBSSLMode)
	if err != nil {
		log.Fatal(err.Error())
	}

	fmt.Println("CCrawler")
	fmt.Println("===================================")
	fmt.Println("Git Commit:", GitCommit)
	fmt.Println("Version:", Version)
	if VersionPrerelease != "" {
		fmt.Println("Version PreRelease:", VersionPrerelease)
	}

	//db connection
	initializeDBConnection(s.DBHost, s.DBPort, s.DBUser, s.DBPassword, s.DBName, s.DBSSLMode, s.DBConnectionRetries, s.DBConnectionBackoff)
	initializeDBSchemas()

	//get exchange rates
	crawlExchangeRates(30)

}
