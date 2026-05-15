package handler

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"escrow/internal/fabric"
	"escrow/internal/middleware"
	"escrow/internal/model"
	"escrow/web/templates"
)

type AdminHandler struct {
	db *pgxpool.Pool
	fc *fabric.Client
}

func NewAdminHandler(db *pgxpool.Pool, fc *fabric.Client) *AdminHandler {
	return &AdminHandler{db: db, fc: fc}
}

func (h *AdminHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	payments := h.listPendingPayments(r)
	disputes := h.listDisputes(r)
	completed := h.listCompleted(r)

	templates.AdminDashboard(payments, disputes, completed, user).Render(r.Context(), w)
}

func (h *AdminHandler) listPendingPayments(r *http.Request) []templates.PendingPaymentItem {
	rows, err := h.db.Query(r.Context(),
		`SELECT p.id, p.contract_id, u.name, p.amount, COALESCE(p.bank_reference, ''), p.created_at
		 FROM payments p
		 JOIN contracts c ON c.id = p.contract_id
		 JOIN users u ON u.id = c.client_id
		 WHERE p.status = 'pending' AND p.direction = 'deposit'
		 ORDER BY p.created_at DESC`,
	)
	if err != nil {
		log.Printf("query pending payments: %v", err)
		return nil
	}
	defer rows.Close()

	var items []templates.PendingPaymentItem
	for rows.Next() {
		var pp templates.PendingPaymentItem
		if err := rows.Scan(&pp.ID, &pp.ContractID, &pp.ClientName, &pp.Amount, &pp.BankReference, &pp.CreatedAt); err != nil {
			continue
		}
		items = append(items, pp)
	}
	return items
}

func (h *AdminHandler) listDisputes(r *http.Request) []templates.DisputeItem {
	rows, err := h.db.Query(r.Context(),
		`SELECT c.id, c.title, cl.name, fl.name, COALESCE(c.dispute_reason, ''), c.amount
		 FROM contracts c
		 JOIN users cl ON cl.id = c.client_id
		 JOIN users fl ON fl.id = c.freelancer_id
		 WHERE c.state = 'DISPUTED'
		 ORDER BY c.created_at DESC`,
	)
	if err != nil {
		log.Printf("query disputes: %v", err)
		return nil
	}
	defer rows.Close()

	var items []templates.DisputeItem
	for rows.Next() {
		var dc templates.DisputeItem
		if err := rows.Scan(&dc.ID, &dc.Title, &dc.ClientName, &dc.FreelancerName, &dc.DisputeReason, &dc.Amount); err != nil {
			continue
		}
		items = append(items, dc)
	}
	return items
}

func (h *AdminHandler) listCompleted(r *http.Request) []templates.CompletedItem {
	rows, err := h.db.Query(r.Context(),
		`SELECT c.id, c.title, c.state, c.completed_at
		 FROM contracts c
		 WHERE c.state IN ('COMPLETED', 'RESOLVED')
		 ORDER BY c.completed_at DESC NULLS LAST LIMIT 20`,
	)
	if err != nil {
		log.Printf("query completed: %v", err)
		return nil
	}
	defer rows.Close()

	var items []templates.CompletedItem
	for rows.Next() {
		var cc templates.CompletedItem
		if err := rows.Scan(&cc.ID, &cc.Title, &cc.State, &cc.CompletedAt); err != nil {
			continue
		}
		items = append(items, cc)
	}
	return items
}

func (h *AdminHandler) ConfirmPayment(w http.ResponseWriter, r *http.Request) {
	paymentID := chi.URLParam(r, "id")
	if paymentID == "" {
		http.Error(w, "Payment ID required", http.StatusBadRequest)
		return
	}

	var contractID string
	err := h.db.QueryRow(r.Context(),
		`UPDATE payments SET status = 'confirmed', confirmed_by = $1, confirmed_at = NOW()
		 WHERE id = $2 AND status = 'pending'
		 RETURNING contract_id`,
		nil, paymentID,
	).Scan(&contractID)
	if err != nil {
		http.Error(w, "Payment not found or already processed", http.StatusNotFound)
		return
	}

	txID, err := h.fc.ConfirmDeposit(contractID)
	if err != nil {
		log.Printf("fabric confirm deposit: %v", err)
		http.Error(w, "Ledger error", http.StatusInternalServerError)
		return
	}

	_, err = h.db.Exec(r.Context(),
		`UPDATE contracts SET state = $1, fabric_tx_id = $2 WHERE id = $3`,
		model.StateFunded, txID, contractID,
	)
	if err != nil {
		log.Printf("db update contract state: %v", err)
	}

	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (h *AdminHandler) RejectPayment(w http.ResponseWriter, r *http.Request) {
	paymentID := chi.URLParam(r, "id")
	if paymentID == "" {
		http.Error(w, "Payment ID required", http.StatusBadRequest)
		return
	}

	_, err := h.db.Exec(r.Context(),
		`UPDATE payments SET status = 'rejected', confirmed_by = $1, confirmed_at = NOW()
		 WHERE id = $2 AND status = 'pending'`,
		nil, paymentID,
	)
	if err != nil {
		http.Error(w, "Payment not found", http.StatusNotFound)
		return
	}
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (h *AdminHandler) ResolveDispute(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Contract ID required", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	decision := r.FormValue("decision")
	if decision == "" {
		http.Error(w, "Decision required", http.StatusBadRequest)
		return
	}

	txID, err := h.fc.AdminResolve(id, decision)
	if err != nil {
		http.Error(w, "Ledger error", http.StatusInternalServerError)
		return
	}

	_, err = h.db.Exec(r.Context(),
		`UPDATE contracts SET state = $1, admin_decision = $2, completed_at = NOW(), fabric_tx_id = $3
		 WHERE id = $4`,
		model.StateResolved, decision, txID, id,
	)
	if err != nil {
		log.Printf("db update resolved: %v", err)
	}

	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}
