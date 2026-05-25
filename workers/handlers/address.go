package handlers

import (
	"errors"
	"strings"

	ethav "github.com/KOREAN139/ethereum-address-validator"
	"github.com/ethereum/go-ethereum/common"
)

func validateEVMAddress(address string) error {
	if !common.IsHexAddress(address) {
		return errors.New("invalid ethereum address format")
	}

	if strings.HasPrefix(address, "0x") || strings.HasPrefix(address, "0X") {
		return ethav.Validate(address)
	}
	return ethav.Validate("0x" + address)
}
