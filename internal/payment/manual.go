package payment

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ManualProvider struct {
	db           *pgxpool.Pool
	bankName     string
	bankRIB      string
	accountName  string
}

func NewManualProvider(db *pgxpool.Pool, bankName, bankRIB, accountName string) *ManualProvider {
	return &ManualProvider{
		db:          db,
		bankName:    bankName,
		bankRIB:     bankRIB,
		accountName: accountName,
	}
}

func (p *ManualProvider) InitiateDeposit(ctx context.Context, req PaymentRequest) (*PaymentResponse, error) {
	var paymentID string
	err := p.db.QueryRow(ctx,
		`INSERT INTO payments (contract_id, amount, direction, method, status)
		 VALUES ($1, $2, 'deposit', 'bank_transfer', 'pending')
		 RETURNING id`,
		req.ContractID, req.Amount,
	).Scan(&paymentID)
	if err != nil {
		return nil, fmt.Errorf("create payment record: %w", err)
	}

	instructions := fmt.Sprintf(
		"Virement bancaire vers %s\nBénéficiaire: %s\nRIB: %s\nRéférence: %s\nMontant: %.3f DT",
		p.bankName, p.accountName, p.bankRIB, req.ContractID, req.Amount,
	)

	return &PaymentResponse{
		PaymentID:    paymentID,
		Instructions: instructions,
		Status:       "pending",
	}, nil
}

func (p *ManualProvider) VerifyPayment(ctx context.Context, paymentID string) (string, error) {
	var status string
	err := p.db.QueryRow(ctx,
		`SELECT status FROM payments WHERE id = $1`, paymentID,
	).Scan(&status)
	if err != nil {
		return "", fmt.Errorf("query payment: %w", err)
	}
	return status, nil
}

func (p *ManualProvider) InitiateRelease(ctx context.Context, contractID, recipientRIB string, amount float64) error {
	_, err := p.db.Exec(ctx,
		`INSERT INTO payments (contract_id, amount, direction, method, status)
		 VALUES ($1, $2, 'release', 'bank_transfer', 'pending')`,
		contractID, amount,
	)
	if err != nil {
		return fmt.Errorf("create release record: %w", err)
	}
	return nil
}
