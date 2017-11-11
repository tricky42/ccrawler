package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"
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

var coinCapClient = &http.Client{Timeout: 10 * time.Second}

func getJSON(url string, target interface{}) error {
	r, err := coinCapClient.Get(url)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(target)
}

func main() {
	versionFlag := flag.Bool("version", false, "Version")
	flag.Parse()

	if *versionFlag {
		fmt.Println("Git Commit:", GitCommit)
		fmt.Println("Version:", Version)
		if VersionPrerelease != "" {
			fmt.Println("Version PreRelease:", VersionPrerelease)
		}
		return
	}

	var coins []Coin
	start := time.Now()
	err := getJSON("http://coincap.io/front", &coins)
	t := time.Now()
	elapsed := t.Sub(start)

	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("# Processing '%d' coins took: %v ...\n", len(coins), elapsed)

	sort.Slice(coins, func(i, j int) bool {
		return (strings.Compare(coins[i].Symbol, coins[j].Symbol) < 1)
	})
	/*
		for _, coin := range coins {
			//fmt.Printf("%s: %f \n", coin.Symbol, coin.Price)
		}
	*/
}
