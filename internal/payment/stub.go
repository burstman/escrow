package payment

import (
	"context"
	"fmt"
)

type StubProvider struct{}

func NewStubProvider() *StubProvider {
	return &StubProvider{}
}

func (p *StubProvider) InitiateDeposit(ctx context.Context, req PaymentRequest) (*PaymentResponse, error) {
	return &PaymentResponse{
		PaymentID:    "stub-payment-" + req.ContractID,
		Instructions: fmt.Sprintf("[DEV MODE] Stub deposit for contract %s — amount %.3f DT", req.ContractID, req.Amount),
		Status:       "confirmed",
	}, nil
}

func (p *StubProvider) VerifyPayment(ctx context.Context, paymentID string) (string, error) {
	return "confirmed", nil
}

func (p *StubProvider) InitiateRelease(ctx context.Context, contractID, recipientRIB string, amount float64) error {
	return nil
}
