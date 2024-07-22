package EVMRPC

import (
	"gobglbridge/config"
	"log"

	"github.com/ethereum/go-ethereum/ethclient"
)

var clients map[int]*ethclient.Client = make(map[int]*ethclient.Client)

// remember last good RPC and round-robin on failures
var roundrobin map[int]int = make(map[int]int)

func GetClient(chainId int, attempt int) *ethclient.Client {
	if _, ok := roundrobin[chainId]; !ok {
		roundrobin[chainId] = 0
	}
	if attempt > 0 {
		// try next RPC
		roundrobin[chainId] = roundrobin[chainId] + 1
	}

	if clients[chainId] == nil {
		client, err := ethclient.Dial(config.EVMChains[chainId].RPCList[roundrobin[chainId]])
		if err != nil {
			log.Fatalln(err)
		}
		clients[chainId] = client
		return client
	}
	return clients[chainId]
}
