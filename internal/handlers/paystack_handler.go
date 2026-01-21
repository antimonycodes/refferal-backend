package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/cirvee/referral-backend/internal/config"
)

type PaystackHandler struct {
	cfg *config.PaystackConfig
}

func NewPaystackHandler(cfg *config.PaystackConfig) *PaystackHandler {
	return &PaystackHandler{cfg: cfg}
}

// Bank represents a Nigerian bank from Paystack
type Bank struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	Code      string `json:"code"`
	Longcode  string `json:"longcode"`
	Gateway   string `json:"gateway"`
	Active    bool   `json:"active"`
	IsDeleted bool   `json:"is_deleted"`
	Country   string `json:"country"`
	Currency  string `json:"currency"`
	Type      string `json:"type"`
}

// PaystackBankListResponse represents the response from Paystack bank list API
type PaystackBankListResponse struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	Data    []Bank `json:"data"`
}

// ResolveAccountData represents resolved account data
type ResolveAccountData struct {
	AccountNumber string `json:"account_number"`
	AccountName   string `json:"account_name"`
	BankID        int    `json:"bank_id"`
}

// PaystackResolveResponse represents the response from Paystack resolve API
type PaystackResolveResponse struct {
	Status  bool               `json:"status"`
	Message string             `json:"message"`
	Data    ResolveAccountData `json:"data"`
}

// ListBanks godoc
// @Summary List Nigerian banks
// @Description Get list of all Nigerian banks from Paystack
// @Tags Banks
// @Produce json
// @Success 200 {object} PaystackBankListResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /api/v1/banks [get]
func (h *PaystackHandler) ListBanks(w http.ResponseWriter, r *http.Request) {
	url := fmt.Sprintf("%s/bank?country=nigeria&perPage=100", h.cfg.BaseURL)

	req, err := http.NewRequestWithContext(r.Context(), "GET", url, nil)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create request")
		return
	}

	req.Header.Set("Authorization", "Bearer "+h.cfg.SecretKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to fetch banks")
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to read response")
		return
	}

	if resp.StatusCode != http.StatusOK {
		respondError(w, http.StatusBadGateway, "paystack API error")
		return
	}

	var paystackResp PaystackBankListResponse
	if err := json.Unmarshal(body, &paystackResp); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to parse response")
		return
	}

	// Return simplified bank list
	type SimplifiedBank struct {
		Name string `json:"name"`
		Code string `json:"code"`
	}

	banks := make([]SimplifiedBank, 0, len(paystackResp.Data))
	for _, b := range paystackResp.Data {
		if b.Active && !b.IsDeleted {
			banks = append(banks, SimplifiedBank{
				Name: b.Name,
				Code: b.Code,
			})
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status": true,
		"data":   banks,
	})
}

// ResolveAccount godoc
// @Summary Resolve bank account
// @Description Get account holder name from account number and bank code
// @Tags Banks
// @Produce json
// @Param account_number query string true "Account number"
// @Param bank_code query string true "Bank code"
// @Success 200 {object} PaystackResolveResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /api/v1/banks/resolve [get]
func (h *PaystackHandler) ResolveAccount(w http.ResponseWriter, r *http.Request) {
	accountNumber := r.URL.Query().Get("account_number")
	bankCode := r.URL.Query().Get("bank_code")

	if accountNumber == "" || bankCode == "" {
		respondError(w, http.StatusBadRequest, "account_number and bank_code are required")
		return
	}

	url := fmt.Sprintf("%s/bank/resolve?account_number=%s&bank_code=%s", h.cfg.BaseURL, accountNumber, bankCode)

	req, err := http.NewRequestWithContext(r.Context(), "GET", url, nil)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create request")
		return
	}

	req.Header.Set("Authorization", "Bearer "+h.cfg.SecretKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to resolve account")
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to read response")
		return
	}

	// Forward the response status from Paystack
	if resp.StatusCode != http.StatusOK {
		var errorResp map[string]interface{}
		json.Unmarshal(body, &errorResp)

		message := "could not resolve account"
		if msg, ok := errorResp["message"].(string); ok {
			message = msg
		}

		respondError(w, http.StatusBadRequest, message)
		return
	}

	var paystackResp PaystackResolveResponse
	if err := json.Unmarshal(body, &paystackResp); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to parse response")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status": true,
		"data": map[string]string{
			"account_name":   paystackResp.Data.AccountName,
			"account_number": paystackResp.Data.AccountNumber,
		},
	})
}
