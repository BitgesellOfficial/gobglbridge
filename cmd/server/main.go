package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"gobglbridge/config"
	"gobglbridge/redis"
	"gobglbridge/workers"
)

func main() {
	log.Print("Starting BGL/WBGL bridge")

	f, err := os.OpenFile(fmt.Sprintf("logs/log_%s.txt", time.Now().Format("2006-01-02")), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening log file for writing: %v", err)
	}
	defer f.Close()

	log.SetOutput(f)

	config.Init()
	// this is for debug, makes output contain sensitive info
	fmt.Printf("%+v", config.Config)

	// connect to Redis, without persistence do not continue
	redis.Init()

	// there are 7 worker threads:
	// * listen to BGL blocks
	// * listen to Eth, BNB, Optimism, Arbitrum blocks
	// * execute pending transactions
	// * static app service and API serving HTTPS server (serves as main worker thread)
	go workers.Worker_scanBGL()
	go workers.Worker_scanEVM(1)
	go workers.Worker_scanEVM(10)
	go workers.Worker_scanEVM(56)
	go workers.Worker_scanEVM(42161)
	go workers.Worker_processExecution()

	workers.Worker_HTTP()
}
