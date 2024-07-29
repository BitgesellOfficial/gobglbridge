package BGLRPC

import (
	"errors"
	"log"

	"gobglbridge/config"

	"github.com/bitgesellofficial/go-bgld"
)

// simple wrapper (probaby could be omitted)
type RPCClient struct {
	Client *bgld.Bgld
}

var client *RPCClient

func GetClient() *RPCClient {
	if client == nil {
		cl, err := bgld.New(config.Config.BGL.Host, config.Config.BGL.Port, config.Config.BGL.RPCUser, config.Config.BGL.RPCPassword, false)
		if err != nil {
			log.Fatalln(err)
		}
		client = &RPCClient{
			Client: cl,
		}
		return client
	}
	return client
}

func (c *RPCClient) GetBlockCount() (uint64, error) {
	return c.Client.GetBlockCount()
}

func (c *RPCClient) GetBalance() (float64, error) {
	return c.Client.GetBalance("*", 0)
}

func (c *RPCClient) ValidateAddress(address string) (bool, error) {
	va, err := c.Client.ValidateAddress(address)
	return va.IsValid, err
}

func (c *RPCClient) GetNewAddress() (string, error) {
	return c.Client.GetNewAddress(config.Config.BGL.WalletName)
}

func (c *RPCClient) ListSinceBlock(blockHash string, confirmations uint32) ([]bgld.Transaction, string, error) {
	return c.Client.ListSinceBlock(blockHash, confirmations)
}

// getTransactionFromAddress
func (c *RPCClient) GetFromAddressForTransaction(txId string) (string, error) {
	rawTx, err := c.Client.GetRawTransaction(txId, true)
	if err != nil {
		return "", err
	}

	rawTxObj, ok := rawTx.(bgld.RawTransaction)
	if !ok {
		return "", errors.New("cannot unmarshal raw transaction")
	}
	vin := rawTxObj.Vin[0]

	rawTx, err = c.Client.GetRawTransaction(vin.Txid, true)
	if err != nil {
		return "", err
	}

	rawTxObj, ok = rawTx.(bgld.RawTransaction)
	if !ok {
		return "", errors.New("cannot unmarshal raw transaction")
	}

	return rawTxObj.Vout[vin.Vout].ScriptPubKey.Address, nil
}

func (c *RPCClient) SendToAddress(address string, amount float64) (string, error) {
	return c.Client.SendToAddress(address, amount, "", "")
}
