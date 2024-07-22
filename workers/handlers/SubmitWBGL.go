package handlers

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"gobglbridge/BGLRPC"
	"gobglbridge/config"
	"gobglbridge/redis"
	"gobglbridge/types"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	ethav "github.com/KOREAN139/ethereum-address-validator"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

type WBGLtoBGLBindingRequest struct {
	EthAddress string `json:"ethAddress"`
	Chain      string `json:"chain"`
	BGLAddress string `json:"bglAddress"`
	Signature  string `json:"signature"`
}

func SubmitWBGL(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading request body: %s", err.Error())
		responseJSON(w, &APIResponse{
			Status:  "error",
			Message: "Error reading request body",
		}, http.StatusBadRequest)
		return
	}

	var req WBGLtoBGLBindingRequest
	err = json.Unmarshal(body, &req)
	if err != nil {
		log.Printf("Error unmarshalling request body: %s\n", err.Error())
		responseJSON(w, &APIResponse{
			Status:  "error",
			Message: "Cannot unmarshal input JSON",
		}, http.StatusBadRequest)
		return
	}

	if err := ethav.Validate(common.HexToAddress(req.EthAddress).Hex()); err != nil {
		log.Printf("Error validating Eth address '%s': %s\n", req.EthAddress, err.Error())
		responseJSON(w, &APIResponse{
			Status:  "error",
			Field:   "ethAddress",
			Message: "No ethereum address or invalid address provided",
		}, http.StatusBadRequest)
		return
	}

	/*
		if valid, err := BGLRPC.GetClient().ValidateAddress(req.BGLAddress); err != nil || !valid {
			if err != nil {
				log.Printf("Error validating BGL address '%s': %s\n", req.BGLAddress, err.Error())
				responseJSON(w, &APIResponse{
					Status:  "error",
					Message: "Cannot check if BGL address is valid",
				}, http.StatusInternalServerError)
				return
			}
			log.Printf("Error validating BGL address '%s': invalid\n", req.BGLAddress)
			responseJSON(w, &APIResponse{
				Status:  "error",
				Field:   "bglAddress",
				Message: "No Bitgesell address or invalid address provided",
			}, http.StatusBadRequest)
			return
		}
	*/

	// TODO replace with config evms list check
	if req.Chain != "eth" && req.Chain != "bnb" && req.Chain != "arb" && req.Chain != "op" {
		responseJSON(w, &APIResponse{
			Status:  "error",
			Field:   "chain",
			Message: "EVM chain not provided or not supported",
		}, http.StatusBadRequest)
		return
	}

	// check that signature is valid
	address, err := validateMsgSignature(req.BGLAddress, req.Signature)
	if err != nil || address == nil {
		log.Printf("Error recovering sig address '%s': %s\n", req.Signature, err.Error())
		responseJSON(w, &APIResponse{
			Status:  "error",
			Field:   "signature",
			Message: "No signature or malformed signature provided",
		}, http.StatusBadRequest)
		return
	}

	if !strings.EqualFold(req.EthAddress, address.Hex()) {
		log.Printf("Recovered sig address '%s', provided '%s'\n", address.Hex(), req.EthAddress)
		responseJSON(w, &APIResponse{
			Status:  "error",
			Field:   "signature",
			Message: "Signature does not match the address provided",
		}, http.StatusBadRequest)
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
		SourceChain:   chain,
		SourceAddress: req.EthAddress,
		DestChain:     0,
		DestAddress:   req.BGLAddress,
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

	balanceBGL, err := BGLRPC.GetClient().GetBalance()
	if err != nil {
		log.Printf("Error getting BGL custodian balance: %s", err.Error())
		// continue processing nonetheless
	}

	//ctx := context.Background()
	responseJSON(w, &APIResponseAddressBook{
		Status:        "ok",
		ID:            rec.ID,
		Address:       config.Config.EVM.PublicAddress,
		Balance:       balanceBGL,
		FeePercentage: fmt.Sprintf("%d", config.Config.FeePercentage),
	}, http.StatusOK)
}

func prefixHash(data []byte) common.Hash {
	msg := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(data), data)
	return crypto.Keccak256Hash([]byte(msg))
}

func publicKeyBytesToAddress(publicKey []byte) *common.Address {
	if len(publicKey) < 1 {
		return nil
	}

	hash := crypto.Keccak256Hash(publicKey[1:]).Bytes()
	address := hash[12:]

	addr := common.HexToAddress(hex.EncodeToString(address))
	return &addr
}

func validateMsgSignature(msg string, sig string) (*common.Address, error) {

	sigBytes, err := hexutil.Decode(sig)
	if err != nil {
		log.Printf("Invalid signature '%s' hex: %s", sig, err.Error())
		return nil, fmt.Errorf("invalid signature hex")
	}

	if sigBytes[64] != 27 && sigBytes[64] != 28 && sigBytes[64] != 0 && sigBytes[64] != 1 {
		log.Printf("Wrong signature '%s' checksum: %v", sig, sigBytes[64])
		return nil, fmt.Errorf("wrong signature checksum")
	}

	if sigBytes[64] == 27 || sigBytes[64] == 28 {
		sigBytes[64] = sigBytes[64] - 27
	}

	msgHash := prefixHash([]byte(msg))
	sigPublicKey, err := crypto.Ecrecover(msgHash.Bytes(), sigBytes)
	if err != nil {
		log.Printf("Cannot decode public key: %s", err.Error())
		return nil, fmt.Errorf("cannot decode public key")
	}

	address := publicKeyBytesToAddress(sigPublicKey)

	return address, nil
}
