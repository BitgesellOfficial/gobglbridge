package handlers

import (
	"errors"

	ethav "github.com/KOREAN139/ethereum-address-validator"
	"github.com/ethereum/go-ethereum/common"
)

func validateEVMAddress(address string) error {
	if !common.IsHexAddress(address) {
		return errors.New("invalid ethereum address format")
	}

	return ethav.Validate(common.HexToAddress(address).Hex())
}
