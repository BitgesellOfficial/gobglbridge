package workers

import (
	"context"
	"fmt"
	"gobglbridge/BGLRPC"
	"gobglbridge/EVMRPC"
	"gobglbridge/EVMRPC/ierc20"
	"gobglbridge/config"
	"gobglbridge/redis"
	"gobglbridge/types"
	"log"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

func Worker_processExecution() {
	for !WorkerShutdown {
		time.Sleep(3 * time.Second)

		pending, err := redis.FindBridgeOperationStatus("pending")
		if err != nil {
			log.Printf("Error getting pending bridge operations by status: %v", err)
		} else if pending == nil {
			//log.Printf("No pending operations are present")
		} else {
			if pending.SourceChain > 0 {
				// WBGL to BGL
				log.Printf("Found pending WBGL to BGL bridge operation, %#v\n", pending)

				addrbookRecord, err := redis.GetAddressBookBySourceAddress(types.CHAINKEY_EVM, strings.ToLower(pending.SourceAddress))
				if err != nil {
					pending.Status = "failed"
					msg := fmt.Sprintf("Error getting address book record: %s", err.Error())
					log.Print(msg)
					if pending.Message == "" {
						pending.Message = msg
					} else {
						pending.Message += "; " + msg
					}
				} else if addrbookRecord == nil {
					pending.Status = "failed"
					msg := "Missing address book record"
					log.Print(msg)
					if pending.Message == "" {
						pending.Message = msg
					} else {
						pending.Message += "; " + msg
					}
				} else {
					amountBI, _ := big.NewInt(0).SetString(pending.Amount, 10)
					divider, _ := big.NewInt(0).SetString("10000000000", 10)
					amountBI = amountBI.Div(amountBI, divider)
					amountFloat, _ := amountBI.Float64()
					amountFloat = amountFloat / 100000000.0

					// deduct bridge fee
					amountFee := amountFloat * float64(config.Config.FeePercentage) / 100.0
					amountFloat = amountFloat - amountFee

					log.Printf("Sending BGL mainnet tx: %.8f (fee %.8f) to %s", amountFloat, amountFee, addrbookRecord.DestAddress)

					tx, err := BGLRPC.GetClient().SendToAddress(addrbookRecord.DestAddress, amountFloat)
					if err == nil {
						pending.Status = "executing"
						pending.DestAddress = addrbookRecord.DestAddress
						pending.DestChain = 0
						pending.DestTxHash = tx
					} else {
						msg := fmt.Sprintf("Error sending %.8f BGL to %v: %v, trying to return %s WBGL on %v", amountFloat, addrbookRecord.DestAddress, err, pending.Amount, config.EVMChains[pending.SourceChain])
						log.Print(msg)
						if pending.Message == "" {
							pending.Message = msg
						} else {
							pending.Message += "; " + msg
						}

						amountBIReturn, _ := big.NewInt(0).SetString(pending.Amount, 10)
						tx, err := sendWBGL(pending.SourceChain, addrbookRecord.SourceAddress, amountBIReturn)

						pending.DestChain = pending.SourceChain
						pending.DestAddress = pending.SourceAddress
						if err == nil {
							log.Printf("Executed returning %s WBGL(%s) to %v, txid: %v", amountBI.String(), config.EVMChains[pending.DestChain].Name, addrbookRecord.DestAddress, tx.Hash().Hex())
							pending.Status = "returning"
							pending.DestTxHash = tx.Hash().Hex()
						} else {
							log.Printf("Error returning %s WBGL to %v: %v", amountBI.String(), addrbookRecord.DestAddress, err)
							pending.Status = "returnfail"
						}

						// don't rush, it's decentralized nodes, etc.
						time.Sleep(5 * time.Second)
					}
				}

				// update record
				err = redis.ChangeBridgeOperationStatus(pending, "pending")
				if err != nil {
					// emergency exit
					log.Printf("Error saving updated bridge operation: %v, emergency exit to avoid looping", err)
					WorkerShutdown = true
				}
			} else {
				// BGL to WBGL
				log.Printf("Found pending BGL to WBGL bridge operation, %#v\n", pending)

				sleep := false
				addrbookRecord, err := redis.GetAddressBookBySourceAddress(types.CHAINKEY_BGL, strings.ToLower(pending.SourceAddress))
				if err != nil {
					log.Printf("Error getting address book record: %s", err.Error())
					pending.Status = "failed"
				} else if addrbookRecord == nil {
					log.Printf("Missing address book record")
					pending.Status = "failed"
				} else {
					amountBF, _ := big.NewFloat(0).SetString(pending.Amount)
					multiplier, _ := big.NewFloat(0).SetString("1000000000000000000")
					amountBF = amountBF.Mul(amountBF, multiplier)

					amountBI := big.NewInt(0)
					amountBI, _ = amountBF.Int(amountBI)

					// deduct bridge fee
					amountFee := big.NewInt(0).Set(amountBI)
					amountFee = amountFee.Mul(amountFee, big.NewInt(int64(config.Config.FeePercentage)))
					amountFee = amountFee.Div(amountFee, big.NewInt(100))
					amountBI = amountBI.Sub(amountBI, amountFee)

					// update record immediately to prevent looped sending if some error
					pending.Status = "executing"
					err = redis.ChangeBridgeOperationStatus(pending, "pending")
					if err != nil {
						// emergency exit
						log.Printf("Error saving updated bridge operation: %v, emergency exit to avoid looping", err)
						WorkerShutdown = true
					}

					log.Printf("Sending WBGL(%s) tx: %s (fee %s) to %s", config.EVMChains[pending.DestChain].Name, amountBI.String(), amountFee.String(), addrbookRecord.DestAddress)
					tx, err := sendWBGL(pending.DestChain, addrbookRecord.DestAddress, amountBI)
					sleep = true

					if err == nil {
						log.Printf("Executed sending %s WBGL(%s) to %v, txid: %v", amountBI.String(), config.EVMChains[pending.DestChain].Name, addrbookRecord.DestAddress, tx.Hash().Hex())
						pending.DestTxHash = tx.Hash().Hex()

					} else {

						msg := fmt.Sprintf("Error sending %s WBGL to %v: %v, trying to return %s BGL", amountBI.String(), addrbookRecord.DestAddress, err, pending.Amount)
						log.Print(msg)
						if pending.Message == "" {
							pending.Message = msg
						} else {
							pending.Message += "; " + msg
						}

						sourceSenderAddress, err := BGLRPC.GetClient().GetFromAddressForTransaction(pending.SourceTxHash)
						if err != nil {
							log.Printf("Error getting sender address for txid %s BGL to %v: %v", pending.Amount, pending.SourceTxHash, err)
							pending.Status = "returnfail"
						} else {
							pending.DestChain = pending.SourceChain
							pending.DestAddress = sourceSenderAddress
							amountFloatReturn, _ := strconv.ParseFloat(pending.Amount, 64)
							tx, err := BGLRPC.GetClient().SendToAddress(sourceSenderAddress, amountFloatReturn)
							if err == nil {
								log.Printf("Executed returning %.8f BGL to %v, txid: %v", amountFloatReturn, sourceSenderAddress, tx)
								pending.Status = "returning"
								pending.DestTxHash = tx
							} else {
								log.Printf("Error returning %.8f BGL to %v: %v", amountFloatReturn, sourceSenderAddress, err)
								pending.Status = "returnfail"
							}
						}
					}
				}

				// update record
				err = redis.ChangeBridgeOperationStatus(pending, "pending")
				if err != nil {
					// emergency exit
					log.Printf("Error saving updated bridge operation: %v, emergency exit to avoid looping", err)
					WorkerShutdown = true
				}

				if sleep {
					// don't rush, it's decentralized nodes, etc.
					time.Sleep(5 * time.Second)
				}
			}
		}

		//break // debugging
	}
}

func sendWBGL(chainId int, address string, amount *big.Int) (*ethtypes.Transaction, error) {
	var tx *ethtypes.Transaction

	var reterr error
	for i := 0; i < config.EVM_RETRIES; i++ {
		client := EVMRPC.GetClient(chainId, i)
		nonce, err := client.PendingNonceAt(context.Background(), common.HexToAddress(config.Config.EVM.PublicAddress))
		if err != nil {
			reterr = fmt.Errorf("error getting nonce for wallet: %s", err)
			log.Print(err.Error())
			continue
		}

		gasPrice, err := client.SuggestGasPrice(context.Background())
		if err != nil {
			reterr = fmt.Errorf("error getting suggested gas price: %s", err)
			log.Print(err.Error())
			continue
		}

		privateKey, err := crypto.HexToECDSA(config.Config.EVM.PrivateKey)
		if err != nil {
			return nil, fmt.Errorf("error instantiating private key: %s", err)
		}
		auth, err := bind.NewKeyedTransactorWithChainID(privateKey, big.NewInt(int64(chainId)))
		if err != nil {
			return nil, fmt.Errorf("error instantiating contract call: %s", err)
		}

		auth.Nonce = big.NewInt(int64(nonce))
		auth.Value = big.NewInt(0)
		auth.GasLimit = uint64(200000)
		if chainId == 1 {
			auth.GasPrice = gasPrice
		} else {
			auth.GasPrice = gasPrice.Mul(gasPrice, big.NewInt(2))
		}

		WBGLcontract, err := ierc20.NewIerc20(common.HexToAddress(config.EVMChains[chainId].ContractAddress), client)
		if err != nil {
			reterr = fmt.Errorf("error instantiating contract: %s", err)
			log.Print(err.Error())
			continue
		}

		tx, err = WBGLcontract.Transfer(auth, common.HexToAddress(address), amount)
		if err != nil {
			reterr = fmt.Errorf("error calling transfer method: %s", err)
			log.Print(err.Error())
			continue
		}

		return tx, nil
	}

	return tx, reterr
}
