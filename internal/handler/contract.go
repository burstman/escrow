package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"escrow/internal/fabric"
	"escrow/internal/middleware"
	"escrow/internal/model"
	"escrow/internal/payment"
	"escrow/internal/storage"
	"escrow/web/templates"
)

type ContractHandler struct {
	db      *pgxpool.Pool
	fc      *fabric.Client
	pmtProv payment.Provider
	store   storage.FileStorage
}

func NewContractHandler(db *pgxpool.Pool, fc *fabric.Client, pmtProv payment.Provider, store storage.FileStorage) *ContractHandler {
	return &ContractHandler{db: db, fc: fc, pmtProv: pmtProv, store: store}
}

func (h *ContractHandler) NewForm(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	templates.NewContract(nil).Render(r.Context(), w)
}

func (h *ContractHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		templates.NewContract(map[string]string{"form": "Bad request"}).Render(r.Context(), w)
		return
	}

	role := r.FormValue("role")
	freelancerEmail := r.FormValue("freelancer_email")
	title := r.FormValue("title")
	amount := r.FormValue("amount")
	depositAmount := r.FormValue("deposit_amount")
	deadline := r.FormValue("deadline")
	revisions := r.FormValue("revisions")
	contentType := r.FormValue("content_type")
	duration := r.FormValue("duration")
	format := r.FormValue("format")
	language := r.FormValue("language")
	referenceLinks := r.FormValue("reference_links")
	avoidNotes := r.FormValue("avoid_notes")
	musicPreference := r.FormValue("music_preference")

	style := r.Form["style"]
	scenes := r.Form["scenes"]

	if freelancerEmail == "" || title == "" || amount == "" || depositAmount == "" ||
		deadline == "" || revisions == "" || contentType == "" || duration == "" ||
		format == "" || language == "" {
		templates.NewContract(map[string]string{"form": "Missing required fields"}).Render(r.Context(), w)
		return
	}

	var other model.User
	err := h.db.QueryRow(r.Context(),
		`SELECT id, name FROM users WHERE email = $1`, freelancerEmail,
	).Scan(&other.ID, &other.Name)
	if err != nil {
		templates.NewContract(map[string]string{"freelancer_email": "User not found"}).Render(r.Context(), w)
		return
	}

	clientID := user.ID
	freelancerID := other.ID
	initialState := model.StatePendingFreelancer

	if role == "freelancer" {
		clientID = other.ID
		freelancerID = user.ID
		initialState = model.StatePendingDeposit
	}

	amountF := parseFloat(amount)
	depositF := parseFloat(depositAmount)
	revCount := parseInt(revisions)

	deadlineParsed, err := time.Parse("2006-01-02", deadline)
	if err != nil {
		templates.NewContract(map[string]string{"deadline": "Invalid date format"}).Render(r.Context(), w)
		return
	}

	if !deadlineParsed.After(time.Now()) {
		templates.NewContract(map[string]string{"deadline": "Deadline must be in the future"}).Render(r.Context(), w)
		return
	}
	if depositF >= amountF {
		templates.NewContract(map[string]string{"deposit_amount": "Deposit must be less than total"}).Render(r.Context(), w)
		return
	}
	if len(scenes) == 0 {
		templates.NewContract(map[string]string{"scenes": "At least one scene is required"}).Render(r.Context(), w)
		return
	}

	brief := model.Brief{
		ContentType:     contentType,
		Duration:        duration,
		Format:          format,
		Language:        language,
		Style:           style,
		Scenes:          scenes,
		ReferenceLinks:  referenceLinks,
		AvoidNotes:      avoidNotes,
		MusicPreference: musicPreference,
	}

	briefHash, err := model.ComputeBriefHash(brief)
	if err != nil {
		templates.NewContract(map[string]string{"form": "Internal error"}).Render(r.Context(), w)
		return
	}

	fabricID := fmt.Sprintf("contract-%d", time.Now().UnixNano())

	txID, err := h.fc.CreateContract(
		fabricID, clientID, freelancerID, title, briefHash,
		amountF, depositF, deadlineParsed.Format(time.RFC3339), revCount,
	)
	if err != nil {
		log.Printf("fabric error: %v", err)
		templates.NewContract(map[string]string{"form": "Failed to create contract on ledger"}).Render(r.Context(), w)
		return
	}

	scenesData, _ := json.Marshal(scenes)
	var contractID string
	err = h.db.QueryRow(r.Context(),
		`INSERT INTO contracts (client_id, freelancer_id, title, content_type, duration, format, language,
		 style, scenes, reference_links, avoid_notes, amount, deposit_amount, revision_count, state,
		 brief_hash, deadline, fabric_tx_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
		 RETURNING id`,
		clientID, freelancerID, title, contentType, duration, format, language,
		style, scenesData, nullIfEmpty(referenceLinks), nullIfEmpty(avoidNotes),
		amountF, depositF, revCount, initialState,
		briefHash, deadlineParsed, txID,
	).Scan(&contractID)
	if err != nil {
		log.Printf("db error: %v", err)
		templates.NewContract(map[string]string{"form": "Failed to save contract"}).Render(r.Context(), w)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/contracts/%s", contractID), http.StatusSeeOther)
}

func (h *ContractHandler) View(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Contract ID required", http.StatusBadRequest)
		return
	}

	c, err := h.getContract(r, id)
	if err != nil {
		http.Error(w, "Contract not found", http.StatusNotFound)
		return
	}

	if c.ClientID != user.ID && c.FreelancerID != user.ID && user.Role != "admin" {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	remainingHours := 0
	if c.AutoReleaseAt != nil && c.State == model.StateDelivered {
		remainingHours = int(time.Until(*c.AutoReleaseAt).Hours())
		if remainingHours < 0 {
			remainingHours = 0
		}
	}

	templates.ViewContract(c, user, true, remainingHours).Render(r.Context(), w)
}

func (h *ContractHandler) List(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	rows, err := h.db.Query(r.Context(),
		`SELECT c.id, c.title, c.state, c.amount, c.created_at,
		        cl.name as client_name, fl.name as freelancer_name
		 FROM contracts c
		 JOIN users cl ON cl.id = c.client_id
		 JOIN users fl ON fl.id = c.freelancer_id
		 WHERE c.client_id = $1 OR c.freelancer_id = $1
		 ORDER BY c.created_at DESC`,
		user.ID,
	)
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var items []templates.ContractListItem
	for rows.Next() {
		var item templates.ContractListItem
		if err := rows.Scan(&item.ID, &item.Title, &item.State, &item.Amount, &item.CreatedAt, &item.ClientName, &item.FreelancerName); err != nil {
			continue
		}
		items = append(items, item)
	}

	templates.ListContracts(items, user).Render(r.Context(), w)
}

func (h *ContractHandler) Accept(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	user := middleware.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	c, err := h.getContract(r, id)
	if err != nil {
		http.Error(w, "Contract not found", http.StatusNotFound)
		return
	}
	if c.State != model.StatePendingFreelancer {
		http.Error(w, "Invalid state", http.StatusBadRequest)
		return
	}
	if c.FreelancerID != user.ID && user.Role != "admin" {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	txID, err := h.fc.FreelancerAccept(id)
	if err != nil {
		http.Error(w, "Ledger error", http.StatusInternalServerError)
		return
	}

	_, err = h.db.Exec(r.Context(),
		`UPDATE contracts SET state = $1, fabric_tx_id = $2 WHERE id = $3`,
		model.StatePendingDeposit, txID, id,
	)
	if err != nil {
		log.Printf("db update: %v", err)
	}

	http.Redirect(w, r, fmt.Sprintf("/contracts/%s", id), http.StatusSeeOther)
}

func (h *ContractHandler) SubmitDeposit(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	id := chi.URLParam(r, "id")
	c, err := h.getContract(r, id)
	if err != nil {
		http.Error(w, "Contract not found", http.StatusNotFound)
		return
	}
	if c.ClientID != user.ID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if c.State != model.StatePendingDeposit {
		http.Error(w, "Invalid state", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	bankRef := r.FormValue("bank_reference")
	if bankRef == "" {
		http.Error(w, "Bank reference required", http.StatusBadRequest)
		return
	}

	txID, err := h.fc.RecordDeposit(id, bankRef)
	if err != nil {
		http.Error(w, "Ledger error", http.StatusInternalServerError)
		return
	}

	_, err = h.db.Exec(r.Context(),
		`UPDATE contracts SET state = $1, fabric_tx_id = $2 WHERE id = $3`,
		model.StateDepositPendingConfirm, txID, id,
	)
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	_, err = h.db.Exec(r.Context(),
		`UPDATE payments SET bank_reference = $1 WHERE contract_id = $2 AND direction = 'deposit'`,
		bankRef, id,
	)
	if err != nil {
		log.Printf("update payment bank ref: %v", err)
	}

	http.Redirect(w, r, fmt.Sprintf("/contracts/%s", id), http.StatusSeeOther)
}

func (h *ContractHandler) Deliver(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	id := chi.URLParam(r, "id")
	c, err := h.getContract(r, id)
	if err != nil {
		http.Error(w, "Contract not found", http.StatusNotFound)
		return
	}
	if c.FreelancerID != user.ID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if c.State != model.StateFunded {
		http.Error(w, "Invalid state", http.StatusBadRequest)
		return
	}

	if err := r.ParseMultipartForm(100 << 20); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "File required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	fileID := fmt.Sprintf("%s/%s", id, "deliverable")
	hash, err := h.store.Upload(r.Context(), fileID, file)
	if err != nil {
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}

	txID, err := h.fc.RecordDelivery(id, hash)
	if err != nil {
		http.Error(w, "Ledger error", http.StatusInternalServerError)
		return
	}

	autoReleaseAt := time.Now().Add(5 * 24 * time.Hour)

	_, err = h.db.Exec(r.Context(),
		`UPDATE contracts SET state = $1, deliverable_hash = $2, auto_release_at = $3, fabric_tx_id = $4
		 WHERE id = $5`,
		model.StateDelivered, hash, autoReleaseAt, txID, id,
	)
	if err != nil {
		log.Printf("db update deliver: %v", err)
	}

	_, err = h.db.Exec(r.Context(),
		`INSERT INTO files (contract_id, uploader_id, original_name, stored_name, mime_type, size_bytes, sha256_hash)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		id, user.ID, header.Filename, fileID, header.Header.Get("Content-Type"), header.Size, hash,
	)
	if err != nil {
		log.Printf("db insert file: %v", err)
	}

	http.Redirect(w, r, fmt.Sprintf("/contracts/%s", id), http.StatusSeeOther)
}

func (h *ContractHandler) Approve(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	user := middleware.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	c, err := h.getContract(r, id)
	if err != nil {
		http.Error(w, "Contract not found", http.StatusNotFound)
		return
	}
	if c.State != model.StateDelivered && c.State != model.StateRevisionRequested {
		http.Error(w, "Invalid state", http.StatusBadRequest)
		return
	}
	if c.ClientID != user.ID && user.Role != "admin" {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	txID, err := h.fc.ApproveDelivery(id)
	if err != nil {
		http.Error(w, "Ledger error", http.StatusInternalServerError)
		return
	}

	_, err = h.db.Exec(r.Context(),
		`UPDATE contracts SET state = $1, completed_at = NOW(), fabric_tx_id = $2 WHERE id = $3`,
		model.StateCompleted, txID, id,
	)
	if err != nil {
		log.Printf("db update approve: %v", err)
	}

	http.Redirect(w, r, fmt.Sprintf("/contracts/%s", id), http.StatusSeeOther)
}

func (h *ContractHandler) Revision(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	user := middleware.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	c, err := h.getContract(r, id)
	if err != nil {
		http.Error(w, "Contract not found", http.StatusNotFound)
		return
	}
	if c.State != model.StateDelivered {
		http.Error(w, "Invalid state", http.StatusBadRequest)
		return
	}
	if c.ClientID != user.ID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	txID, err := h.fc.RequestRevision(id)
	if err != nil {
		http.Error(w, "Ledger error", http.StatusInternalServerError)
		return
	}

	nextState := model.StateRevisionRequested
	if c.RevisionCount <= 0 {
		nextState = model.StateCompleted
	}

	_, err = h.db.Exec(r.Context(),
		`UPDATE contracts SET state = $1, fabric_tx_id = $2 WHERE id = $3`,
		nextState, txID, id,
	)
	if err != nil {
		log.Printf("db update revision: %v", err)
	}

	http.Redirect(w, r, fmt.Sprintf("/contracts/%s", id), http.StatusSeeOther)
}

func (h *ContractHandler) Dispute(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	id := chi.URLParam(r, "id")
	_, err := h.getContract(r, id)
	if err != nil {
		http.Error(w, "Contract not found", http.StatusNotFound)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	reason := r.FormValue("reason")
	if reason == "" {
		http.Error(w, "Dispute reason required", http.StatusBadRequest)
		return
	}

	txID, err := h.fc.RaiseDispute(id, reason)
	if err != nil {
		http.Error(w, "Ledger error", http.StatusInternalServerError)
		return
	}

	_, err = h.db.Exec(r.Context(),
		`UPDATE contracts SET state = $1, dispute_reason = $2, fabric_tx_id = $3 WHERE id = $4`,
		model.StateDisputed, reason, txID, id,
	)
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/contracts/%s", id), http.StatusSeeOther)
}

func (h *ContractHandler) getContract(r *http.Request, id string) (*model.Contract, error) {
	var c model.Contract
	var style []string
	var scenesData []byte
	var disputeReason, adminDecision, fabricTxID, deliverableHash *string
	var autoReleaseAt, completedAt *time.Time
	var referenceLinks, avoidNotes *string

	err := h.db.QueryRow(r.Context(),
		`SELECT c.id, c.client_id, c.freelancer_id, c.title, c.content_type, c.duration,
		        c.format, c.language, c.style, c.scenes, c.reference_links, c.avoid_notes,
		        c.amount, c.deposit_amount, c.revision_count, c.state,
		        c.dispute_reason, c.admin_decision, c.fabric_tx_id,
		        c.brief_hash, c.deliverable_hash, c.deadline,
		        c.auto_release_at, c.created_at, c.completed_at,
		        cl.name as client_name, fl.name as freelancer_name
		 FROM contracts c
		 JOIN users cl ON cl.id = c.client_id
		 JOIN users fl ON fl.id = c.freelancer_id
		 WHERE c.id = $1`, id,
	).Scan(
		&c.ID, &c.ClientID, &c.FreelancerID, &c.Title, &c.ContentType, &c.Duration,
		&c.Format, &c.Language, &style, &scenesData, &referenceLinks, &avoidNotes,
		&c.Amount, &c.DepositAmount, &c.RevisionCount, &c.State,
		&disputeReason, &adminDecision, &fabricTxID,
		&c.BriefHash, &deliverableHash, &c.Deadline,
		&autoReleaseAt, &c.CreatedAt, &completedAt,
		&c.ClientName, &c.FreelancerName,
	)
	if err != nil {
		return nil, err
	}

	if disputeReason != nil {
		c.DisputeReason = disputeReason
	}
	if adminDecision != nil {
		c.AdminDecision = adminDecision
	}
	if fabricTxID != nil {
		c.FabricTxID = fabricTxID
	}
	if deliverableHash != nil {
		c.DeliverableHash = deliverableHash
	}
	if autoReleaseAt != nil {
		c.AutoReleaseAt = autoReleaseAt
	}
	if completedAt != nil {
		c.CompletedAt = completedAt
	}
	if referenceLinks != nil {
		c.ReferenceLinks = *referenceLinks
	}
	if avoidNotes != nil {
		c.AvoidNotes = *avoidNotes
	}

	c.Scenes = []string{}
	if len(scenesData) > 0 {
		json.Unmarshal(scenesData, &c.Scenes)
	}
	c.Style = style

	return &c, nil
}

func parseFloat(s string) float64 {
	var v float64
	fmt.Sscanf(s, "%f", &v)
	return v
}

func parseInt(s string) int {
	var v int
	fmt.Sscanf(s, "%d", &v)
	return v
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
