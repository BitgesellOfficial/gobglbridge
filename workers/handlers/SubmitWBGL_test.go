package handlers

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
)

func TestValidateMsgSignatureRejectsMalformedLength(t *testing.T) {
	malformedSignatures := []string{
		"0x",
		"0x00",
		"0x" + strings.Repeat("00", 64),
		"0x" + strings.Repeat("00", 66),
	}

	for _, sig := range malformedSignatures {
		t.Run(sig, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("validateMsgSignature panicked for malformed signature: %v", r)
				}
			}()

			addr, err := validateMsgSignature("message", sig)
			if err == nil {
				t.Fatalf("expected malformed signature error")
			}
			if addr != nil {
				t.Fatalf("expected nil address for malformed signature, got %s", addr.Hex())
			}
			if !strings.Contains(err.Error(), "length") && !strings.Contains(err.Error(), "hex") {
				t.Fatalf("expected length or hex validation error, got %q", err.Error())
			}
		})
	}
}

func TestValidateMsgSignatureAcceptsRecoveryIDs(t *testing.T) {
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	message := "bgl-address"
	hash := prefixHash([]byte(message))
	sig, err := crypto.Sign(hash.Bytes(), privateKey)
	if err != nil {
		t.Fatalf("sign message: %v", err)
	}

	expected := crypto.PubkeyToAddress(privateKey.PublicKey)
	for _, recoveryID := range []byte{sig[64], sig[64] + 27} {
		sigWithRecovery := append([]byte(nil), sig...)
		sigWithRecovery[64] = recoveryID

		addr, err := validateMsgSignature(message, "0x"+hex.EncodeToString(sigWithRecovery))
		if err != nil {
			t.Fatalf("expected valid signature with recovery ID %d: %v", recoveryID, err)
		}
		if addr == nil || *addr != expected {
			t.Fatalf("expected recovered address %s, got %v", expected.Hex(), addr)
		}
	}
}
