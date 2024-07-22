package types

// it is assumed BGL mainnet is id 0
// Eth mainnet id 1
// BNB id 56, etc.

type ChainType int

const CHAINKEY_BGL ChainType = 0
const CHAINKEY_EVM ChainType = 1

// Address Book is stored as Redis list
type AddressBookRecord struct {
	ID            string
	SourceChain   int
	DestChain     int
	SourceAddress string
	DestAddress   string
	TsCreated     int64
}

// address book is populated in memory
// and is stored in redis for persistence

// Bridge operation is a single bridge pair or transaction (input and output)
// having a status
type BridgeOperation struct {
	ID            string
	Status        string
	SourceChain   int
	DestChain     int
	TsFound       int64
	Amount        string // amount in WEI (1e18) or in BGL Satoshis (only have 1e8 precision)
	SourceAddress string
	DestAddress   string // filled when destination transaction is executed (or returned)
	SourceTxHash  string // transaction where funds are received by bridge
	DestTxHash    string // transaction where funds are sent by bridge
	Message       string // messsages that help to track processing/errors
}
