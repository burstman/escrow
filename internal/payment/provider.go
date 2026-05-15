package payment

import "context"

type PaymentRequest struct {
	ContractID  string
	Amount      float64
	ClientName  string
	Description string
}

type PaymentResponse struct {
	PaymentID    string
	Instructions string
	Status       string
}

type Provider interface {
	InitiateDeposit(ctx context.Context, req PaymentRequest) (*PaymentResponse, error)
	VerifyPayment(ctx context.Context, paymentID string) (string, error)
	InitiateRelease(ctx context.Context, contractID, recipientRIB string, amount float64) error
}
