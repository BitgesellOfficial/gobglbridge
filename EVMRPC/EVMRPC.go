package EVMRPC

import (
	"fmt"
	"log"

	"gobglbridge/config"

	"github.com/ethereum/go-ethereum/ethclient"
)

func WithClient[T any](chainId int, f func(client *ethclient.Client) (T, error)) (res T, err error) {
	var client *ethclient.Client
	for _, url := range config.EVMChains[chainId].RPCList {
		client, err = ethclient.Dial(url)
		if err != nil {
			log.Println(fmt.Sprintf("Error connecting to %s: %s", url, err.Error()))
			continue
		}

		res, err = f(client)
		client.Close()
		if err == nil {
			return
		}
	}
	return
}
