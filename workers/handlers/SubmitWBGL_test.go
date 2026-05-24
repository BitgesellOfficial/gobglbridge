package handlers

import "testing"

func TestValidateMsgSignatureRejectsShortSignature(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("validateMsgSignature panicked for short signature: %v", r)
		}
	}()

	if addr, err := validateMsgSignature("message", "0x00"); err == nil || addr != nil {
		t.Fatalf("validateMsgSignature(short signature) = (%v, %v), want nil address and error", addr, err)
	}
}
