package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPaymentMethodLabel(t *testing.T) {
	cases := []struct {
		name, paymentType, raw, want string
		specific                     bool
	}{
		{"empty", "", `{}`, "", false},
		{"bca va", "bank_transfer", `{"va_numbers":[{"bank":"bca","va_number":"123"}]}`, "BCA Virtual Account", true},
		{"permata va", "bank_transfer", `{"permata_va_number":"8778"}`, "Permata Virtual Account", true},
		{"va without detail", "bank_transfer", `{"transaction_status":"settlement"}`, "Virtual Account", false},
		{"cstore", "cstore", `{"store":"indomaret"}`, "Indomaret", true},
		{"gopay", "gopay", `{}`, "GoPay", true},
		{"qris", "qris", `not json`, "QRIS", true},
		{"unknown type", "some_new_wallet", `{}`, "Some New Wallet", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, specific := PaymentMethodLabel(tc.paymentType, tc.raw)
			assert.Equal(t, tc.want, got)
			assert.Equal(t, tc.specific, specific)
		})
	}
}
