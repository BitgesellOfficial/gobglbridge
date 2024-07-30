package redis

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"gobglbridge/config"
	"gobglbridge/types"

	"github.com/ethereum/go-ethereum/common"
	"github.com/gomodule/redigo/redis"
	"github.com/google/uuid"
)

var pool *redis.Pool

func timeoutDialOptions() []redis.DialOption {
	return []redis.DialOption{
		redis.DialConnectTimeout(5 * time.Second),
		redis.DialReadTimeout(5 * time.Second),
		redis.DialWriteTimeout(5 * time.Second),
	}
}

func Init() {
	redisAddr := fmt.Sprintf("%s:%d", config.Config.Server.RedisHost, config.Config.Server.RedisPort)
	pool = &redis.Pool{
		MaxIdle: 5,
		Dial:    func() (redis.Conn, error) { return redis.Dial("tcp", redisAddr, timeoutDialOptions()...) },
	}
}

func GetBGLScannedBlock() (string, error) {
	conn := pool.Get()
	defer conn.Close()

	blockHash, err := redis.String(conn.Do("GET", "BGLBlockHash"))
	if err == nil {
		return blockHash, nil
	}

	if errors.Is(err, redis.ErrNil) {
		return "", nil
	}

	log.Printf("error Redis get: %s", err.Error())
	return "", err
}

func SetBGLScannedBlock(blockHash string) error {
	conn := pool.Get()
	defer conn.Close()

	_, err := conn.Do("SET", "BGLBlockHash", blockHash)
	if err != nil {
		log.Printf("error Redis set: %s", err.Error())
		return err
	}

	return nil
}

func GetEVMScannedBlock(chainID int) (int, error) {
	conn := pool.Get()
	defer conn.Close()

	blockHeight, err := redis.Int(conn.Do("GET", fmt.Sprintf("chainBlockScanned:%d", chainID)))
	if err == nil {
		return blockHeight, nil
	}

	if errors.Is(err, redis.ErrNil) {
		return -1, nil
	}

	log.Printf("error Redis get: %s", err.Error())
	return -1, err
}

func SetEVMScannedBlock(chainID int, blockHeight int) error {
	conn := pool.Get()
	defer conn.Close()

	_, err := conn.Do("SET", fmt.Sprintf("chainBlockScanned:%d", chainID), blockHeight)
	if err != nil {
		log.Printf("error Redis set: %s", err.Error())
		return err
	}

	return nil
}

// note that multiple sets should not contain one operation
func UpsertBridgeOperation(op *types.BridgeOperation) error {
	conn := pool.Get()
	defer conn.Close()

	if op == nil {
		return errors.New("null object to store")
	}

	if op.Status == "" {
		return errors.New("bridge operation cannot have empty status")
	}

	if op.ID == "" {
		op.ID = uuid.New().String()
	}
	recordKey := fmt.Sprintf("bridgeop:%s:%s", op.Status, op.ID)

	opJSON, err := json.Marshal(op)
	if err != nil {
		return fmt.Errorf("cannot marshal bridge operation to JSON: %s", err.Error())
	}

	_, err = conn.Do("SET", recordKey, opJSON)
	if err != nil {
		log.Printf("error Redis SET: %s", err.Error())
		return err
	}

	// also add the key to the corresponding SET
	_, err = conn.Do("SADD", config.RedisStatusSets[op.Status], recordKey)
	if err != nil {
		log.Printf("error Redis SET: %s", err.Error())
		return err
	}

	return nil
}

func ChangeBridgeOperationStatus(op *types.BridgeOperation, prevStatus string) error {
	conn := pool.Get()
	defer conn.Close()

	if op == nil {
		return errors.New("null object to store")
	}

	if op.Status == "" {
		return errors.New("bridge operation cannot have empty status")
	}

	if op.ID == "" {
		op.ID = uuid.New().String()
	}

	prevRecordKey := fmt.Sprintf("bridgeop:%s:%s", prevStatus, op.ID)
	recordKey := fmt.Sprintf("bridgeop:%s:%s", op.Status, op.ID)

	opJSON, err := json.Marshal(op)
	if err != nil {
		return fmt.Errorf("cannot marshal bridge operation to JSON: %s", err.Error())
	}

	_, err = conn.Do("SREM", config.RedisStatusSets[prevStatus], prevRecordKey)
	if err != nil {
		log.Printf("error Redis SREM: %s", err.Error())
		return err
	}

	_, err = conn.Do("DEL", prevRecordKey)
	if err != nil {
		log.Printf("error Redis DEL: %s", err.Error())
		return err
	}

	_, err = conn.Do("SET", recordKey, opJSON)
	if err != nil {
		log.Printf("error Redis SET: %s", err.Error())
		return err
	}

	_, err = conn.Do("SADD", config.RedisStatusSets[op.Status], recordKey)
	if err != nil {
		log.Printf("error Redis SADD: %s", err.Error())
		return err
	}

	return nil
}

// Attention, this operation scans everything that is present
// Older/processed should be moved to another place otherwise performance will degrade (athough O(n) still)
func FindBridgeOperationSourceTxHash(txHash string) (*types.BridgeOperation, error) {
	return FindBridgeOperationAllStatuses("SourceTxHash", txHash)
}

func FindBridgeOperationDestinationTxHash(txHash string) (*types.BridgeOperation, error) {
	return FindBridgeOperationAllStatuses("DestTxHash", txHash)
}

func FindBridgeOperationAllStatuses(field string, value string) (*types.BridgeOperation, error) {
	for status := range config.RedisStatusSets {
		op, err := FindBridgeOperationByFieldStringValue(field, value, status)
		if err != nil {
			return nil, err
		}
		if op != nil {
			return op, nil
		}
	}
	return nil, nil
}

func FindBridgeOperationStatus(status string) (*types.BridgeOperation, error) {
	return FindBridgeOperationByFieldStringValue("Status", status, status)
}

func FindBridgeOperationByFieldStringValue(field, value string, status string) (*types.BridgeOperation, error) {
	conn := pool.Get()
	defer conn.Close()

	if field == "" || value == "" {
		return nil, errors.New("empty search field name or value")
	}

	// scan every operation present in Redis
	var cursor int64

	for {
		values, err := redis.Values(conn.Do("SSCAN", config.RedisStatusSets[status], cursor))
		if err != nil {
			return nil, err
		}

		var opKeys []string
		values, err = redis.Scan(values, &cursor, &opKeys)
		if err != nil {
			return nil, err
		}

		for _, key := range opKeys {
			op, err := redis.Bytes(conn.Do("GET", key))
			if err != nil && !errors.Is(err, redis.ErrNil) {
				log.Printf("error Redis GET: %s", err.Error())
				return nil, err
			}

			var opStruct types.BridgeOperation
			// TODO: a record can be missing, don't crash
			// fmt.Printf("record:" + string(op) + "\n")
			err = json.Unmarshal([]byte(op), &opStruct)
			if err != nil {
				return nil, err
			}
			if field == "Status" && opStruct.Status == value {
				return &opStruct, nil
			}
			if field == "SourceTxHash" && opStruct.SourceTxHash == value {
				return &opStruct, nil
			}
			if field == "DestTxHash" && opStruct.DestTxHash == value {
				return &opStruct, nil
			}
		}

		if cursor == 0 {
			break
		}
	}

	return nil, nil
}

func UpsertAddressBookRecord(rec *types.AddressBookRecord) error {
	conn := pool.Get()
	defer conn.Close()

	if rec == nil {
		return errors.New("null object to store")
	}

	if rec.SourceAddress == "" {
		return errors.New("address book record cannot have empty status")
	}

	// addrbook for all EVM->BGL mainnet tuples is the same
	chainKeyPart := types.CHAINKEY_BGL
	if rec.SourceChain > 0 {
		chainKeyPart = types.CHAINKEY_EVM
	}

	if rec.ID == "" {
		rec.ID = uuid.New().String()
	}
	recordKey := fmt.Sprintf("addrbook:%d:%s", chainKeyPart, strings.ToLower(rec.SourceAddress))

	recJSON, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("cannot marshal address book record to JSON: %s", err.Error())
	}

	_, err = conn.Do("SET", recordKey, recJSON)
	if err != nil {
		log.Printf("error Redis SET: %s", err.Error())
		return err
	}

	return nil
}

func GetAddressBookBySourceAddress(chainType types.ChainType, address string) (*types.AddressBookRecord, error) {
	conn := pool.Get()
	defer conn.Close()

	addrbook, err := redis.Bytes(conn.Do("GET", fmt.Sprintf("addrbook:%d:%s", chainType, strings.ToLower(address))))

	// bug when tolower wasn't called and address went in mixed case
	// let's retry with mixed case
	if errors.Is(err, redis.ErrNil) {
		addrbook, err = redis.Bytes(conn.Do("GET", fmt.Sprintf("addrbook:%d:%s", chainType, common.HexToAddress(address).Hex())))
	}

	if errors.Is(err, redis.ErrNil) {
		return nil, nil
	}

	if err != nil {
		log.Printf("error Redis get: %s", err.Error())
		return nil, err
	}

	var addrbookRecord types.AddressBookRecord
	err = json.Unmarshal(addrbook, &addrbookRecord)
	if err != nil {
		return nil, err
	}
	return &addrbookRecord, nil
}

func FindAllBridgeOperationsByStatus(status string) ([]*types.BridgeOperation, error) {
	conn := pool.Get()
	defer conn.Close()

	if _, ok := config.RedisStatusSets[status]; !ok {
		return nil, errors.New("redis key not found for status")
	}

	ops := make([]*types.BridgeOperation, 0, 0)

	// scan every operation present in Redis
	var cursor int64

	for {
		values, err := redis.Values(conn.Do("SSCAN", config.RedisStatusSets[status], cursor))
		if err != nil {
			return nil, err
		}

		var opKeys []string
		values, err = redis.Scan(values, &cursor, &opKeys)
		if err != nil {
			return nil, err
		}

		for _, key := range opKeys {
			op, err := redis.Bytes(conn.Do("GET", key))
			if err != nil && !errors.Is(err, redis.ErrNil) {
				log.Printf("error Redis GET: %s", err.Error())
				return nil, err
			}

			var opStruct types.BridgeOperation
			// TODO: a record can be missing, don't crash
			// fmt.Printf("record:" + string(op) + "\n")
			err = json.Unmarshal([]byte(op), &opStruct)
			if err != nil {
				return nil, err
			}
			if opStruct.Status == status {
				ops = append(ops, &opStruct)
			}
		}

		if cursor == 0 {
			break
		}
	}

	return ops, nil
}
