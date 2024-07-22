package workers

import (
	"fmt"
	"gobglbridge/BGLRPC"
	"gobglbridge/config"
	"gobglbridge/redis"
	"gobglbridge/types"
	"log"
	"time"

	"github.com/google/uuid"
)

func Worker_scanBGL() {
	for !WorkerShutdown {
		// BGL is Bitcoin-derived and we don't need high frequency iterating
		time.Sleep(30 * time.Second)

		scannedBlockHash, err := redis.GetBGLScannedBlock()
		if err != nil {
			log.Printf("Error getting last scanned BGL block hash: %s", err.Error())
			continue
		}

		transactions, lastblock, err := BGLRPC.GetClient().ListSinceBlock(scannedBlockHash, uint32(config.Config.BGL.Confirmations))
		if err != nil {
			log.Printf("Error getting BGL transactions since block hash %s: %s", scannedBlockHash, err.Error())
			continue
		}

		for _, tx := range transactions {
			if tx.Confirmations >= int64(config.Config.BGL.Confirmations) && tx.Category == "receive" {
				addrbook, err := redis.GetAddressBookBySourceAddress(types.CHAINKEY_BGL, tx.Address)
				if err != nil {
					log.Printf("Error checking address book record: %s", err.Error())
					continue
				}

				if addrbook != nil {

					log.Printf("BGL transfer %s: from: %s, to: %v, amount: %v. Saving incoming bridge tx.", tx.TxID, "-", tx.Address, tx.Amount)

					// never add record if a record present with same source tx hash, otherwise could be double send
					existingOp, err := redis.FindBridgeOperationSourceTxHash(tx.TxID)

					if existingOp == nil && err == nil {
						// store new bridge tx to redis
						err = redis.UpsertBridgeOperation(&types.BridgeOperation{
							ID:            uuid.New().String(),
							Status:        "pending",
							SourceChain:   0,
							DestChain:     addrbook.DestChain, // only support now bridging to/from BGL mainnet
							TsFound:       time.Now().Unix(),
							Amount:        fmt.Sprintf("%.8f", tx.Amount),
							SourceAddress: tx.Address,
							DestAddress:   addrbook.DestAddress,
							SourceTxHash:  tx.TxID,
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
				} else {
					log.Printf("ERROR: missing address book record for %d:%s", 0, tx.Address)

					err = redis.UpsertBridgeOperation(&types.BridgeOperation{
						ID:            uuid.New().String(),
						Status:        "failed",
						SourceChain:   0,
						DestChain:     -1, // unknown
						TsFound:       time.Now().Unix(),
						Amount:        fmt.Sprintf("%.8f", tx.Amount),
						SourceAddress: tx.Address,
						DestAddress:   "",
						SourceTxHash:  tx.TxID,
						DestTxHash:    "",
						Message:       "Missing address book record",
					})

					if err != nil {
						// don't consider this block as processed
						log.Printf("Cannot create pending bridge operation, Redis error: %s", err.Error())
						break
					}

					// TODO return funds
				}
			}
		}

		redis.SetBGLScannedBlock(lastblock)
	}
}
