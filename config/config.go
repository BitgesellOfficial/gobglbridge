package config

type Configuration struct {
	// Server config
	Server struct {
		UseSSL    bool   `yaml:"ssl"`
		RedisPort int    `yaml:"redis_port"`
		RedisHost string `yaml:"redis_host"`
	} `yaml:"server"`
	// BGL-related config
	BGL struct {
		Host          string `yaml:"host"`
		Port          int    `yaml:"port"`
		Confirmations int    `yaml:"confirmations"`
		// important private stuff
		RPCUser     string `yaml:"rpc_user"`
		RPCPassword string `yaml:"rpc_pass"`
		WalletName  string `yaml:"wallet_name"`
	} `yaml:"BGL"`
	// EVM-related config
	EVM struct {
		PublicAddress string `yaml:"address"`
		PrivateKey    string `yaml:"private_key"`
	} `yaml:"EVM"`
	FeePercentage int `yaml:"fee_percentage"`
}

var Config Configuration

// log topic to look for
const EVM_TOKEN_TRANSFER = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"

// maximum number of EVM RPC retries
const EVM_RETRIES = 3

// EVM-chains configs
type ChainConfig struct {
	Name             string
	ChainID          int
	RPCList          []string
	ContractAddress  string // WBGL token address
	MinConfirmations int
	BlockBatch       int
	// StartingBlock    int // from when to start scan if no previous record
	SafetyWindow int // as logs go in another thread, make some room, and also to pickup txs sent by bridge to finalize
}

var EVMChains = map[int]ChainConfig{
	1: {
		Name:             "Eth",
		ChainID:          1,
		RPCList:          []string{"https://eth.drpc.org", "https://eth.llamarpc.com"},
		ContractAddress:  "0x2bA64EFB7A4Ec8983E22A49c81fa216AC33f383A",
		MinConfirmations: 3,
		BlockBatch:       512,
		SafetyWindow:     10,
	}, // Ethereum
	10: {
		Name:             "Optimism",
		ChainID:          10,
		RPCList:          []string{"https://rpc.ankr.com/optimism", "https://optimism.llamarpc.com", "https://optimism.drpc.org"},
		ContractAddress:  "0x2bA64EFB7A4Ec8983E22A49c81fa216AC33f383A",
		MinConfirmations: 3,
		BlockBatch:       512,
		SafetyWindow:     100,
	}, // Optimism
	56: {
		Name:             "BNB",
		ChainID:          56,
		RPCList:          []string{"https://rpc.ankr.com/bsc", "https://bsc.drpc.org", "https://bsc.meowrpc.com"},
		ContractAddress:  "0x2bA64EFB7A4Ec8983E22A49c81fa216AC33f383A",
		MinConfirmations: 3,
		BlockBatch:       512,
		SafetyWindow:     25,
	}, // BNB
	42161: {
		Name:             "Arbitrum",
		ChainID:          42161,
		RPCList:          []string{"https://rpc.ankr.com/arbitrum", "https://arbitrum.llamarpc.com", "https://arbitrum.meowrpc.com"},
		ContractAddress:  "0x2bA64EFB7A4Ec8983E22A49c81fa216AC33f383A",
		MinConfirmations: 3,
		BlockBatch:       512,
		SafetyWindow:     100,
	}, // Arbitrum
}

var RedisStatusSets = map[string]string{
	"pending":       "bridgeops:pending",       // souce transaction was scanned
	"failed":        "bridgeops:failed",        // failed to process, error occured and cannot return funds
	"executing":     "bridgeops:executing",     // desination transaction sent successfully
	"success":       "bridgeops:success",       // destination transaction entered block and was scanned
	"returning":     "bridgeops:returning",     // tried to return funds because destination has not enough BGL or gas
	"returnfail":    "bridgeops:returnfail",    // tried to initiate return but encountered a fn error
	"returnsuccess": "brdigeops:returnsuccess", // funds returned successfully
}
