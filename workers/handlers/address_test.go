package handlers

import "testing"

func TestValidateEVMAddressRejectsMalformedInput(t *testing.T) {
	tests := []string{
		"",
		"not-an-address",
		"0x1",
		"0x000000000000000000000000000000000000000",
		"0x00000000000000000000000000000000000000000",
		"0x00000000000000000000000000000000000000zz",
	}

	for _, tt := range tests {
		if err := validateEVMAddress(tt); err == nil {
			t.Fatalf("validateEVMAddress(%q) returned nil error", tt)
		}
	}
}

func TestValidateEVMAddressAcceptsChecksummedAndLowercaseInput(t *testing.T) {
	tests := []string{
		"0x52908400098527886E0F7030069857D2E4169EE7",
		"0xde709f2102306220921060314715629080e2fb77",
		"de709f2102306220921060314715629080e2fb77",
	}

	for _, tt := range tests {
		if err := validateEVMAddress(tt); err != nil {
			t.Fatalf("validateEVMAddress(%q) returned error: %s", tt, err)
		}
	}
}
