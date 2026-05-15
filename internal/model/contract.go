package model

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
)

type Brief struct {
	ContentType    string   `json:"contentType"`
	Duration       string   `json:"duration"`
	Format         string   `json:"format"`
	Language       string   `json:"language"`
	Style          []string `json:"style"`
	Scenes         []string `json:"scenes"`
	ReferenceLinks string   `json:"referenceLinks"`
	AvoidNotes     string   `json:"avoidNotes"`
	MusicPreference string  `json:"musicPreference"`
}

func ComputeBriefHash(brief Brief) (string, error) {
	data, err := json.Marshal(brief)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

type Contract struct {
	ID              string     `json:"id"`
	ClientID        string     `json:"clientId"`
	FreelancerID    string     `json:"freelancerId"`
	Title           string     `json:"title"`
	ContentType     string     `json:"contentType"`
	Duration        string     `json:"duration"`
	Format          string     `json:"format"`
	Language        string     `json:"language"`
	Style           []string   `json:"style"`
	Scenes          []string   `json:"scenes"`
	ReferenceLinks  string     `json:"referenceLinks"`
	AvoidNotes      string     `json:"avoidNotes"`
	Amount          float64    `json:"amount"`
	DepositAmount   float64    `json:"depositAmount"`
	RevisionCount   int        `json:"revisionCount"`
	State           string     `json:"state"`
	DisputeReason   *string    `json:"disputeReason"`
	AdminDecision   *string    `json:"adminDecision"`
	FabricTxID      *string    `json:"fabricTxId"`
	BriefHash       string     `json:"briefHash"`
	DeliverableHash *string    `json:"deliverableHash"`
	Deadline        time.Time  `json:"deadline"`
	AutoReleaseAt   *time.Time `json:"autoReleaseAt"`
	CreatedAt       time.Time  `json:"createdAt"`
	CompletedAt     *time.Time `json:"completedAt"`

	ClientName     string `json:"clientName"`
	FreelancerName string `json:"freelancerName"`
}

const (
	StatePendingFreelancer    = "PENDING_FREELANCER"
	StatePendingDeposit       = "PENDING_DEPOSIT"
	StateDepositPendingConfirm = "DEPOSIT_PENDING_CONFIRM"
	StateFunded                = "FUNDED"
	StateDelivered             = "DELIVERED"
	StateRevisionRequested     = "REVISION_REQUESTED"
	StateDisputed              = "DISPUTED"
	StateCompleted             = "COMPLETED"
	StateCancelled             = "CANCELLED"
	StateResolved              = "RESOLVED"
)
