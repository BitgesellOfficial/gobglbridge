package handlers

import (
	"fmt"
	"gobglbridge/EVMRPC"
	"gobglbridge/EVMRPC/ierc20"
	"gobglbridge/config"
	"log"
	"math/big"
	"net/http"

	"github.com/ethereum/go-ethereum/common"
)

func BalanceEth(w http.ResponseWriter, r *http.Request) {
	BalanceEVM(w, r, 1)
}

func BalanceOP(w http.ResponseWriter, r *http.Request) {
	BalanceEVM(w, r, 10)
}

func BalanceBNB(w http.ResponseWriter, r *http.Request) {
	BalanceEVM(w, r, 56)
}

func BalanceArb(w http.ResponseWriter, r *http.Request) {
	BalanceEVM(w, r, 42161)
}

func BalanceEVM(w http.ResponseWriter, r *http.Request, chainId int) {

	balanceBI, err := WBGLBalanceInt(chainId)
	if err != nil {
		responsePlain(w, []byte("error"), http.StatusInternalServerError)
		return
	}

	divisor, _ := big.NewInt(0).SetString("1000000000000000000", 10)
	balanceBI = balanceBI.Div(balanceBI, divisor)
	responsePlain(w, []byte(balanceBI.String()), 200)
}

func WBGLBalanceInt(chainId int) (*big.Int, error) {
	var reterr error
	for i := 0; i < config.EVM_RETRIES; i++ {
		client := EVMRPC.GetClient(chainId, i)

		WBGLcontract, err := ierc20.NewIerc20(common.HexToAddress(config.EVMChains[chainId].ContractAddress), client)
		if err != nil {
			reterr = fmt.Errorf("error creating contract instance: %s", err)
			log.Print(err.Error())
			continue
		}

		balanceBI, err := WBGLcontract.BalanceOf(nil, common.HexToAddress(config.Config.EVM.PublicAddress))
		if err != nil {
			reterr = fmt.Errorf("error getting balance: %s", err)
			log.Print(err.Error())
			continue
		}

		return balanceBI, nil
	}
	return nil, reterr
}
