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
	DBHost     string `default:"localhost"`
	DBPort     int    `default:"5432"`
	DBUser     string `default:"postgres"`
	DBPassword string `default:"postgres"`
	DBName     string `default:"public"`
	Version    bool   `default:"false"`
	DBSSLMode  string `default:"disable"`
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
		err = callback()
		if err == nil {
			return
		}

		if i >= (attempts - 1) {
			break
		}

		time.Sleep(sleep)
		log.Println("Retrying after error:", err)
	}
	return fmt.Errorf("After %d attempts, last error: %s", attempts, err)
}

func initDBConnection(host string, port int, user string, password string, dbname string, sslmode string) (*sql.DB, error) {
	fmt.Println("# initDBConnection: called!")
	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s", host, port, user, password, dbname, sslmode)
	db, err := sql.Open("postgres", psqlInfo)
	fmt.Println("# initDBConnection: sqlOpen()!")
	if err != nil {
		fmt.Println("# initDBConnection: error on sql open: ", err)
		return db, err
	}
	err = db.Ping()
	fmt.Println("# initDBConnection: sql.Ping()!")
	if err != nil {
		fmt.Println("# initDBConnection: error on sql ping: ", err)
		return db, err
	}
	return db, err
}

func main() {

	//env vars
	var s Specification
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
	var db *sql.DB
	err = retry(5, 3*time.Second, func() (err error) {
		db, err = initDBConnection(s.DBHost, s.DBPort, s.DBUser, s.DBPassword, s.DBName, s.DBSSLMode)
		return
	})
	if err != nil {
		fmt.Println("# db con error: ", err)
	}

	//get exchange rates
	var coins []Coin
	start := time.Now()
	err = getJSON("http://coincap.io/front", &coins)
	t := time.Now()
	elapsed := t.Sub(start)

	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("# Time: %s\n", start.Format("20060102_150405"))
	fmt.Printf("# Processing '%d' coins took: %v ...\n", len(coins), elapsed)

	sort.Slice(coins, func(i, j int) bool {
		return (strings.Compare(coins[i].Symbol, coins[j].Symbol) < 1)
	})

	//db check schema
	if _, err = db.Exec(schema); err != nil {
		fmt.Printf("Schema Creation skipped: %v \n", err)
	}

	//insert exchange rates
	ts := start.Format("200601021504")
	fmt.Printf("# Inserting '%d' coin exchange rate datepoints ...\n", len(coins))

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

		fmt.Print(".")
	}
	fmt.Println()
	query := fmt.Sprintf("INSERT INTO exchange_rates(id, symbol, name, price, volume, supply, percentage, timestamp) VALUES %s", strings.Join(valueStrings, ","))
	_, err = db.Exec(query, valueArgs...)
	t = time.Now()
	elapsed = t.Sub(start)

	if err != nil {
		fmt.Printf("=> Adding new exchange rates skipped! This is most propably caused multiple updates running within one minute. (Processing time: %v, Error Message:%v)", elapsed, err)
	} else {
		fmt.Printf("=> Adding %d exchange rates processed in %s ...!", len(coins), elapsed)
	}
}
