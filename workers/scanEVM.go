package workers

import (
	"gobglbridge/config"
	"gobglbridge/redis"
	"gobglbridge/types"
	"log"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/google/uuid"
	"github.com/ybbus/jsonrpc"
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
		//if scannedBlockNum == -1 {
		// init starting block numbers when running in new environment
		//scannedBlockNum = config.EVMChains[chainId].ScannedBlockNum
		//}

		lastScannedBlock := scannedBlockNum

		rpcClient := jsonrpc.NewClient(config.EVMChains[chainId].RPCList[0])

		latestBlockStr := new(string)
		err = rpcClient.CallFor(latestBlockStr, "eth_blockNumber")
		if err != nil {
			log.Printf("Error getting last EVM block eth_blockNumber: %s", err.Error())
			continue
		}
		latestBlock, _ := hexutil.DecodeUint64(*latestBlockStr)
		//fmt.Printf("Latest block on %s is: %d, last scanned is: %d\n", config.EVMChains[chainId].Name, latestBlock, scannedBlockNum)

		if scannedBlockNum == -1 {
			scannedBlockNum = int(latestBlock) - config.EVMChains[chainId].SafetyWindow
		} else {
			scannedBlockNum = scannedBlockNum - config.EVMChains[chainId].SafetyWindow
		}

		// check WBGL transfers where recipient is bridge custodian EOA
		for blockNum := scannedBlockNum + 1; blockNum < int(latestBlock); blockNum = blockNum + config.EVMChains[chainId].BlockBatch {
			fromBlock := uint64(blockNum)
			toBlock := uint64(blockNum + config.EVMChains[chainId].BlockBatch - 1)
			if uint64(blockNum+config.EVMChains[chainId].BlockBatch-1) > latestBlock {
				toBlock = uint64(latestBlock)
				log.Printf("Scanning blocks %s from %v to %v (latest)...\n", config.EVMChains[chainId].Name, blockNum, latestBlock)
			} else {
				log.Printf("Scanning blocks %s from %v to %v...\n", config.EVMChains[chainId].Name, blockNum, toBlock)
			}
			params := reqData{Address: config.EVMChains[chainId].ContractAddress, FromBlock: hexutil.EncodeUint64(fromBlock), ToBlock: hexutil.EncodeUint64(toBlock)}
			paramarr := make([]reqData, 0, 1)
			paramarr = append(paramarr, params)

			resp, err := rpcClient.Call("eth_getLogs", paramarr)
			if err != nil {
				log.Printf("Error querying EVM RPC: %s\n", err.Error())
				break
			}

			if resp.Error != nil {
				log.Printf("Result Error: %s\n", resp.Error.Message)
				break
			} else {
				respDataArray := resp.Result.([]interface{})
				for i := range respDataArray {
					respDataInterface := respDataArray[i]
					respData := respDataInterface.(map[string]interface{})

					if respData["topics"].([]interface{})[0] == config.EVM_TOKEN_TRANSFER {

						txHash, _ := respData["transactionHash"].(string)

						strData := respData["data"].(string)

						sender := common.HexToAddress(respData["topics"].([]interface{})[1].(string))
						recipient := common.HexToAddress(respData["topics"].([]interface{})[2].(string))

						amount, _ := math.ParseBig256(strData[0:66])

						if recipient.Hex() == common.HexToAddress(config.Config.EVM.PublicAddress).Hex() {

							// never add record if a record present with same source tx hash, otherwise could be double send
							existingOp, err := redis.FindBridgeOperationSourceTxHash(txHash)

							if existingOp == nil && err == nil {
								log.Printf("Found new WBGL transfer %s: from: %s, to: %v, amount: %v. Saving incoming bridge tx.", txHash, sender, recipient, amount)

								// store new bridge tx to redis
								err = redis.UpsertBridgeOperation(&types.BridgeOperation{
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
								})

								if err != nil {
									// don't consider this block as processed
									log.Printf("Cannot create pending bridge operation, Redis error: %s", err.Error())
									break
								}
							} else if existingOp != nil {
								log.Printf("Found existing bridge operation record with same source tx hash: %+v", existingOp)
							} else {
								log.Printf("Error searching Redis: %s", err.Error())
							}
						} else if sender.Hex() == common.HexToAddress(config.Config.EVM.PublicAddress).Hex() {

							// record should be present with same source tx hash or destination tx hash, otherwise this orphaned (manual?) transfer from bridge wallet
							// in destination tx hashes when processing in progress
							existingOp, err := redis.FindBridgeOperationDestinationTxHash(txHash)

							if existingOp == nil && err == nil {
								log.Printf("Error: found no existing bridge operation record with destination tx hash: %s (manual tx?)", txHash)
							} else if err != nil {
								log.Printf("Error searching Redis: %s", err.Error())
							} else {

								// TODO: take into consideration failed (reverted) txs

								log.Printf("WBGL transfer %s: from: %s, to: %v, amount: %v. Finalizing outgoing/returned bridge tx.", txHash, sender, recipient, amount)

								prevStatus := existingOp.Status
								if existingOp.Status == "executing" {
									existingOp.Status = "success"
								} else if existingOp.Status == "returning" {
									existingOp.Status = "returnsuccess"
								} else if existingOp.Status == "success" || existingOp.Status == "returnsuccess" {
									// do nothing, tx processed, all ok
									break
								} else {
									log.Printf("Error: found existing operation with destination hash %s with unexpected status %s", txHash, existingOp.Status)
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
				}

				lastScannedBlock = int(toBlock)
				time.Sleep(50 * time.Millisecond)
			}

			redis.SetEVMScannedBlock(chainId, lastScannedBlock)
		}
	}
}
