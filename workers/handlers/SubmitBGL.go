package handlers

import (
	"encoding/json"
	"fmt"
	"gobglbridge/BGLRPC"
	"gobglbridge/config"
	"gobglbridge/redis"
	"gobglbridge/types"
	"io/ioutil"
	"log"
	"math/big"
	"net/http"
	"time"

	ethav "github.com/KOREAN139/ethereum-address-validator"
	"github.com/ethereum/go-ethereum/common"
)

type BGLtoWBGLBindingRequest struct {
	Address string `json:"address"`
	Chain   string `json:"chain"`
}

func SubmitBGL(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading request body: %s", err.Error())
		responseJSON(w, &APIResponse{
			Status:  "error",
			Message: "Error reading request body",
		}, http.StatusBadRequest)
		return
	}

	var req BGLtoWBGLBindingRequest
	err = json.Unmarshal(body, &req)
	if err != nil {
		log.Printf("Error unmarshalling request body: %s\n", err.Error())
		responseJSON(w, &APIResponse{
			Status:  "error",
			Message: "Cannot unmarshal input JSON",
		}, http.StatusBadRequest)
		return
	}

	if err := ethav.Validate(common.HexToAddress(req.Address).Hex()); err != nil {
		log.Printf("Error validating EVM address '%s': %s\n", req.Address, err.Error())
		responseJSON(w, &APIResponse{
			Status:  "error",
			Field:   "ethAddress",
			Message: "No ethereum address or invalid address provided",
		}, http.StatusBadRequest)
		return
	}

	// TODO replace with config evms list check
	if req.Chain != "eth" && req.Chain != "bnb" && req.Chain != "arb" && req.Chain != "op" {
		responseJSON(w, &APIResponse{
			Status:  "error",
			Field:   "chain",
			Message: "EVM chain not provided or not supported",
		}, http.StatusBadRequest)
		return
	}

	BGLaddress, err := BGLRPC.GetClient().GetNewAddress()
	if err != nil {
		log.Printf("Error creating new BGL address: %s\n", err.Error())
		responseJSON(w, &APIResponse{
			Status:  "error",
			Message: "Cannot create receiving address",
		}, http.StatusInternalServerError)
		return
	}

	chain := 1
	// for compatibility with current
	if req.Chain == "arb" {
		chain = 42161
	} else if req.Chain == "bnb" {
		chain = 56
	} else if req.Chain == "op" {
		chain = 10
	}

	rec := types.AddressBookRecord{
		SourceChain:   0,
		SourceAddress: BGLaddress,
		DestChain:     chain,
		DestAddress:   req.Address,
		TsCreated:     time.Now().Unix(),
	}

	err = redis.UpsertAddressBookRecord(&rec)
	if err != nil {
		log.Printf("Error storing address book record: %s\n", err.Error())
		responseJSON(w, &APIResponse{
			Status:  "error",
			Field:   "signature",
			Message: "Error binding address",
		}, http.StatusInternalServerError)
		return
	}

	log.Printf("Created new address record %d:%s to %d:%s at %d", rec.SourceChain, rec.SourceAddress, rec.DestChain, rec.DestAddress, rec.TsCreated)

	// get WBGL balance
	balanceFloat := 0.0
	balanceWBGL, _ := WBGLBalanceInt(chain)
	if balanceWBGL != nil {
		balanceBF := big.NewFloat(0.0).SetInt(balanceWBGL)
		divisor, _ := big.NewFloat(0.0).SetString("1000000000000000000.0")
		balanceBF = balanceBF.Quo(balanceBF, divisor)
		balanceFloat, _ = balanceBF.Float64()
	}

	//ctx := context.Background()
	responseJSON(w, &APIResponseAddressBook{
		Status:        "ok",
		ID:            rec.ID,
		BGLAddress:    BGLaddress,
		Balance:       balanceFloat,
		FeePercentage: fmt.Sprintf("%d", config.Config.FeePercentage),
	}, http.StatusOK)
}
