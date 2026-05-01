package tools

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Henryarrovin/mcp-server/mcp"
)

func RegisterPaymentTools(s *mcp.Server, baseURL string) {
	client := &http.Client{Timeout: 10 * time.Second}

	do := func(method, url, token, body string) (string, error) {
		var r io.Reader
		if body != "" {
			r = strings.NewReader(body)
		}
		req, err := http.NewRequest(method, url, r)
		if err != nil {
			return "", err
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := client.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		data, _ := io.ReadAll(resp.Body)
		return string(data), nil
	}

	// Create Order
	s.AddTool(
		mcp.NewTool("payment_create_order",
			"Create a new payment order via Razorpay",
			map[string]mcp.Property{
				"token":    mcp.Str("JWT access token"),
				"amount":   mcp.Num("Amount in paise e.g. 50000 = ₹500"),
				"currency": mcp.Str("Currency code default INR"),
				"notes":    mcp.Str("Optional JSON metadata"),
			},
			[]string{"token", "amount"},
		),
		func(args map[string]any) (*mcp.ToolCallResult, error) {
			token := mcp.GetString(args, "token", "")
			amount := int64(mcp.GetFloat64(args, "amount", 0))
			currency := mcp.GetString(args, "currency", "INR")
			notes := mcp.GetString(args, "notes", "")
			body := fmt.Sprintf(`{"amount":%d,"currency":%q,"notes":%q}`, amount, currency, notes)
			result, err := do(http.MethodPost, baseURL+"/api/v1/payments/orders", token, body)
			if err != nil {
				return mcp.ErrorResult(err.Error()), nil
			}
			return mcp.TextResult(result), nil
		},
	)

	// Get Order
	s.AddTool(
		mcp.NewTool("payment_get_order",
			"Get order details by internal order ID",
			map[string]mcp.Property{
				"token":    mcp.Str("JWT access token"),
				"order_id": mcp.Str("Internal order UUID"),
			},
			[]string{"token", "order_id"},
		),
		func(args map[string]any) (*mcp.ToolCallResult, error) {
			token := mcp.GetString(args, "token", "")
			orderID := mcp.GetString(args, "order_id", "")
			result, err := do(http.MethodGet, baseURL+"/api/v1/payments/orders/"+orderID, token, "")
			if err != nil {
				return mcp.ErrorResult(err.Error()), nil
			}
			return mcp.TextResult(result), nil
		},
	)

	// List Orders
	s.AddTool(
		mcp.NewTool("payment_list_orders",
			"List all orders for the authenticated user",
			map[string]mcp.Property{
				"token": mcp.Str("JWT access token"),
			},
			[]string{"token"},
		),
		func(args map[string]any) (*mcp.ToolCallResult, error) {
			token := mcp.GetString(args, "token", "")
			result, err := do(http.MethodGet, baseURL+"/api/v1/payments/orders", token, "")
			if err != nil {
				return mcp.ErrorResult(err.Error()), nil
			}
			return mcp.TextResult(result), nil
		},
	)

	// Capture Payment
	s.AddTool(
		mcp.NewTool("payment_capture",
			"Capture a payment after user completes checkout",
			map[string]mcp.Property{
				"token":               mcp.Str("JWT access token"),
				"order_id":            mcp.Str("Internal order UUID"),
				"provider_payment_id": mcp.Str("Razorpay payment ID"),
				"provider_signature":  mcp.Str("Razorpay HMAC signature"),
				"method":              mcp.Str("Payment method: upi/card/netbanking"),
			},
			[]string{"token", "order_id", "provider_payment_id", "provider_signature"},
		),
		func(args map[string]any) (*mcp.ToolCallResult, error) {
			token := mcp.GetString(args, "token", "")
			orderID := mcp.GetString(args, "order_id", "")
			ppID := mcp.GetString(args, "provider_payment_id", "")
			sig := mcp.GetString(args, "provider_signature", "")
			method := mcp.GetString(args, "method", "upi")
			body := fmt.Sprintf(`{"provider_payment_id":%q,"provider_signature":%q,"method":%q}`, ppID, sig, method)
			result, err := do(http.MethodPost, baseURL+"/api/v1/payments/"+orderID+"/capture", token, body)
			if err != nil {
				return mcp.ErrorResult(err.Error()), nil
			}
			return mcp.TextResult(result), nil
		},
	)

	// Get Payment
	s.AddTool(
		mcp.NewTool("payment_get_payment",
			"Get payment details by payment ID",
			map[string]mcp.Property{
				"token":      mcp.Str("JWT access token"),
				"payment_id": mcp.Str("Internal payment UUID"),
			},
			[]string{"token", "payment_id"},
		),
		func(args map[string]interface{}) (*mcp.ToolCallResult, error) {
			token := mcp.GetString(args, "token", "")
			paymentID := mcp.GetString(args, "payment_id", "")
			result, err := do(http.MethodGet, baseURL+"/api/v1/payments/"+paymentID, token, "")
			if err != nil {
				return mcp.ErrorResult(err.Error()), nil
			}
			return mcp.TextResult(result), nil
		},
	)

	// Refund
	s.AddTool(
		mcp.NewTool("payment_refund",
			"Issue a refund for a payment — admin only",
			map[string]mcp.Property{
				"token":      mcp.Str("Admin JWT access token"),
				"payment_id": mcp.Str("Internal payment UUID"),
				"amount":     mcp.Num("Amount in paise 0 means full refund"),
				"notes":      mcp.Str("Reason for refund"),
			},
			[]string{"token", "payment_id"},
		),
		func(args map[string]any) (*mcp.ToolCallResult, error) {
			token := mcp.GetString(args, "token", "")
			paymentID := mcp.GetString(args, "payment_id", "")
			amount := int64(mcp.GetFloat64(args, "amount", 0))
			notes := mcp.GetString(args, "notes", "")
			body := fmt.Sprintf(`{"amount":%d,"notes":%q}`, amount, notes)
			result, err := do(http.MethodPost, baseURL+"/api/v1/payments/"+paymentID+"/refund", token, body)
			if err != nil {
				return mcp.ErrorResult(err.Error()), nil
			}
			return mcp.TextResult(result), nil
		},
	)

	// Generate Signature
	s.AddTool(
		mcp.NewTool("payment_generate_signature",
			"Generate Razorpay HMAC signature via mock server for testing capture",
			map[string]mcp.Property{
				"mock_url":   mcp.Str("Mock server URL default http://mock-razorpay-service:8090"),
				"order_id":   mcp.Str("Provider order ID starts with order_"),
				"payment_id": mcp.Str("Provider payment ID"),
				"secret":     mcp.Str("Razorpay key_secret from DB"),
			},
			[]string{"order_id", "payment_id", "secret"},
		),
		func(args map[string]any) (*mcp.ToolCallResult, error) {
			mockURL := mcp.GetString(args, "mock_url", "http://mock-razorpay-service:8090")
			orderID := mcp.GetString(args, "order_id", "")
			paymentID := mcp.GetString(args, "payment_id", "")
			secret := mcp.GetString(args, "secret", "")
			url := fmt.Sprintf("%s/v1/sign?order_id=%s&payment_id=%s&secret=%s", mockURL, orderID, paymentID, secret)
			result, err := do(http.MethodGet, url, "", "")
			if err != nil {
				return mcp.ErrorResult(err.Error()), nil
			}
			return mcp.TextResult(result), nil
		},
	)

	// Payment Health
	s.AddTool(
		mcp.NewTool("payment_health",
			"Check if payment-gateway is healthy and reachable",
			map[string]mcp.Property{},
			nil,
		),
		func(args map[string]any) (*mcp.ToolCallResult, error) {
			result, err := do(http.MethodGet, baseURL+"/api/v1/payments/health", "", "")
			if err != nil {
				return mcp.ErrorResult("payment-gateway unreachable: " + err.Error()), nil
			}
			return mcp.TextResult(result), nil
		},
	)
}
