package fabric

type EscrowContract struct {
	ID              string `json:"id"`
	ClientID        string `json:"clientId"`
	FreelancerID    string `json:"freelancerID"`
	Title           string `json:"title"`
	BriefHash       string `json:"briefHash"`
	Amount          float64 `json:"amount"`
	DepositAmount   float64 `json:"depositAmount"`
	Deadline        string `json:"deadline"`
	RevisionCount   int    `json:"revisionCount"`
	DeliverableHash string `json:"deliverableHash"`
	State           string `json:"state"`
	DisputeReason   string `json:"disputeReason,omitempty"`
	AdminDecision   string `json:"adminDecision,omitempty"`
	CreatedAt       string `json:"createdAt"`
	DeliveredAt     string `json:"deliveredAt,omitempty"`
	AutoReleaseAt   string `json:"autoReleaseAt,omitempty"`
	CompletedAt     string `json:"completedAt,omitempty"`
}

const (
	StatePendingFreelancer     = "PENDING_FREELANCER"
	StatePendingDeposit        = "PENDING_DEPOSIT"
	StateDepositPendingConfirm = "DEPOSIT_PENDING_CONFIRM"
	StateFunded                = "FUNDED"
	StateDelivered             = "DELIVERED"
	StateRevisionRequested     = "REVISION_REQUESTED"
	StateDisputed              = "DISPUTED"
	StateCompleted             = "COMPLETED"
	StateCancelled             = "CANCELLED"
	StateResolved              = "RESOLVED"
)
