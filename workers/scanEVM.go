package workers

import (
	"context"
	"log"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/ethclient"
	"gobglbridge/EVMRPC"
	"gobglbridge/config"
	"gobglbridge/redis"
	"gobglbridge/types"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/common/math"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/google/uuid"
)

type reqData struct {
	Address   string `json:"address,omitempty"`
	FromBlock string `json:"fromBlock,omitempty"`
	ToBlock   string `json:"toBlock,omitempty"`
}

func Worker_scanEVM(chainId int) {
	for !WorkerShutdown {
		// latency of <30 sec should be ok for EVM chains (even if Arb is faster)
		time.Sleep(10 * time.Second)

		scannedBlockNum, err := redis.GetEVMScannedBlock(chainId)
		if err != nil {
			log.Printf("Error getting last scanned EVM block hash: %s", err.Error())
			continue
		}
		// if scannedBlockNum == -1 {
		// init starting block numbers when running in new environment
		// scannedBlockNum = config.EVMChains[chainId].ScannedBlockNum
		// }

		lastScannedBlock := scannedBlockNum

		latestBlock, err := EVMRPC.WithClient(
			chainId, func(client *ethclient.Client) (uint64, error) {
				return client.BlockNumber(context.Background())
			},
		)
		if err != nil {
			log.Printf("Error getting last EVM block eth_blockNumber: %s", err.Error())
			continue
		}
		// fmt.Printf("Latest block on %s is: %d, last scanned is: %d\n", config.EVMChains[chainId].Name, latestBlock, scannedBlockNum)

		if scannedBlockNum == -1 {
			scannedBlockNum = int(latestBlock) - config.EVMChains[chainId].SafetyWindow
		} else {
			scannedBlockNum = scannedBlockNum - config.EVMChains[chainId].SafetyWindow
		}

		// check WBGL transfers where recipient is bridge custodian EOA
		for blockNum := scannedBlockNum + 1; blockNum < int(latestBlock); blockNum = blockNum + config.EVMChains[chainId].BlockBatch {
			fromBlock := int64(blockNum)
			toBlock := int64(blockNum + config.EVMChains[chainId].BlockBatch - 1)
			if uint64(blockNum+config.EVMChains[chainId].BlockBatch-1) > latestBlock {
				toBlock = int64(latestBlock)
				log.Printf(
					"Scanning blocks %s from %v to %v (latest)...\n",
					config.EVMChains[chainId].Name,
					blockNum,
					latestBlock,
				)
			} else {
				log.Printf("Scanning blocks %s from %v to %v...\n", config.EVMChains[chainId].Name, blockNum, toBlock)
			}

			logs, err := EVMRPC.WithClient(
				chainId, func(client *ethclient.Client) ([]ethtypes.Log, error) {
					return client.FilterLogs(
						context.Background(), ethereum.FilterQuery{
							FromBlock: big.NewInt(fromBlock),
							ToBlock:   big.NewInt(toBlock),
							Addresses: []common.Address{common.HexToAddress(config.EVMChains[chainId].ContractAddress)},
							Topics:    [][]common.Hash{{common.HexToHash(config.EVM_TOKEN_TRANSFER)}},
						},
					)
				},
			)
			if err != nil {
				log.Printf("Error querying EVM RPC: %s\n", err.Error())
				break
			}

			for _, l := range logs {
				txHash := l.TxHash.String()
				sender := common.HexToAddress(l.Topics[1].String())
				recipient := common.HexToAddress(l.Topics[2].String())
				data := hexutil.Encode(l.Data)
				amount, _ := math.ParseBig256(data[0:66])

				if recipient.Hex() == common.HexToAddress(config.Config.EVM.PublicAddress).Hex() {

					// never add record if a record present with same source tx hash, otherwise could be double send
					existingOp, err := redis.FindBridgeOperationSourceTxHash(txHash)

					if existingOp == nil && err == nil {
						log.Printf(
							"Found new WBGL transfer %s: from: %s, to: %v, amount: %v. Saving incoming bridge tx.",
							txHash,
							sender,
							recipient,
							amount,
						)

						// store new bridge tx to redis
						err = redis.UpsertBridgeOperation(
							&types.BridgeOperation{
								ID:            uuid.New().String(),
								Status:        "pending",
								SourceChain:   chainId,
								DestChain:     0, // only support bridging to/from BGL mainnet
								TsFound:       time.Now().Unix(),
								Amount:        amount.String(),
								SourceAddress: sender.Hex(),
								DestAddress:   "", // filled by execution worker
								SourceTxHash:  txHash,
								DestTxHash:    "",
							},
						)

						if err != nil {
							// don't consider this block as processed
							log.Printf("Cannot create pending bridge operation, Redis error: %s", err.Error())
							break
						}
					} else if existingOp != nil {
						log.Printf(
							"Found existing bridge operation record with same source tx hash: %+v",
							existingOp,
						)
					} else {
						log.Printf("Error searching Redis: %s", err.Error())
					}
				} else if sender.Hex() == common.HexToAddress(config.Config.EVM.PublicAddress).Hex() {

					// record should be present with same source tx hash or destination tx hash, otherwise this orphaned (manual?) transfer from bridge wallet
					// in destination tx hashes when processing in progress
					existingOp, err := redis.FindBridgeOperationDestinationTxHash(txHash)

					if existingOp == nil && err == nil {
						log.Printf(
							"Error: found no existing bridge operation record with destination tx hash: %s (manual tx?)",
							txHash,
						)
					} else if err != nil {
						log.Printf("Error searching Redis: %s", err.Error())
					} else {

						// TODO: take into consideration failed (reverted) txs

						log.Printf(
							"WBGL transfer %s: from: %s, to: %v, amount: %v. Finalizing outgoing/returned bridge tx.",
							txHash,
							sender,
							recipient,
							amount,
						)

						prevStatus := existingOp.Status
						if existingOp.Status == "executing" {
							existingOp.Status = "success"
						} else if existingOp.Status == "returning" {
							existingOp.Status = "returnsuccess"
						} else if existingOp.Status == "success" || existingOp.Status == "returnsuccess" {
							// do nothing, tx processed, all ok
							break
						} else {
							log.Printf(
								"Error: found existing operation with destination hash %s with unexpected status %s",
								txHash,
								existingOp.Status,
							)
							break
						}
						// update info about operation in redis
						err = redis.ChangeBridgeOperationStatus(existingOp, prevStatus)

						if err != nil {
							// don't consider this block as processed
							log.Printf("Cannot update bridge operation status, Redis error: %s", err.Error())
							break
						}
					}
				}
			}

			lastScannedBlock = int(toBlock)
			time.Sleep(50 * time.Millisecond)

			redis.SetEVMScannedBlock(chainId, lastScannedBlock)
		}
	}
}
