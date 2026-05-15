package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"escrow/internal/middleware"
	"escrow/internal/payment"
)

type PaymentHandler struct {
	db   *pgxpool.Pool
	prov payment.Provider
}

func NewPaymentHandler(db *pgxpool.Pool, prov payment.Provider) *PaymentHandler {
	return &PaymentHandler{db: db, prov: prov}
}

func (h *PaymentHandler) InitiateDeposit(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	contractID := chi.URLParam(r, "id")
	if contractID == "" {
		http.Error(w, "Contract ID required", http.StatusBadRequest)
		return
	}

	resp, err := h.prov.InitiateDeposit(r.Context(), payment.PaymentRequest{
		ContractID: contractID,
		ClientName: user.Name,
	})
	if err != nil {
		http.Error(w, "Failed to initiate deposit", http.StatusInternalServerError)
		return
	}

	_ = resp
	http.Redirect(w, r, "/contracts/"+contractID, http.StatusSeeOther)
}
