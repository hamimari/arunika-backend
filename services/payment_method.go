package services

import (
	"encoding/json"
	"strings"
)

var paymentTypeLabels = map[string]string{
	"credit_card":    "Kartu Kredit/Debit",
	"gopay":          "GoPay",
	"qris":           "QRIS",
	"shopeepay":      "ShopeePay",
	"dana":           "DANA",
	"echannel":       "Mandiri Bill Payment",
	"bca_klikpay":    "BCA KlikPay",
	"bca_klikbca":    "KlikBCA",
	"cimb_clicks":    "CIMB Clicks",
	"danamon_online": "Danamon Online Banking",
	"bri_epay":       "BRImo",
	"akulaku":        "Akulaku",
	"kredivo":        "Kredivo",
	"uob_ezpay":      "UOB EZPay",
}

// PaymentMethodLabel turns a Midtrans payment_type (plus the raw notification
// payload, for the bank/store name) into a human-readable label. specific
// reports whether the label names the exact bank/store rather than just the
// method family — e.g. "BCA Virtual Account" vs "Virtual Account".
func PaymentMethodLabel(paymentType, rawPayload string) (label string, specific bool) {
	var payload struct {
		VANumbers []struct {
			Bank string `json:"bank"`
		} `json:"va_numbers"`
		PermataVANumber string `json:"permata_va_number"`
		Store           string `json:"store"`
	}
	_ = json.Unmarshal([]byte(rawPayload), &payload)

	switch paymentType {
	case "":
		return "", false
	case "bank_transfer":
		if len(payload.VANumbers) > 0 && payload.VANumbers[0].Bank != "" {
			return strings.ToUpper(payload.VANumbers[0].Bank) + " Virtual Account", true
		}
		if payload.PermataVANumber != "" {
			return "Permata Virtual Account", true
		}
		return "Virtual Account", false
	case "cstore":
		if payload.Store != "" {
			return strings.ToUpper(payload.Store[:1]) + payload.Store[1:], true
		}
		return "Gerai Retail", false
	}
	if l, ok := paymentTypeLabels[paymentType]; ok {
		return l, true
	}
	// Unknown/new Midtrans type: "some_type" -> "Some Type".
	words := strings.Split(paymentType, "_")
	for i, w := range words {
		if w != "" {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " "), true
}
