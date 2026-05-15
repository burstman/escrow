package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

type SmartContract struct {
	contractapi.Contract
}

type EscrowContract struct {
	ID              string  `json:"id"`
	ClientID        string  `json:"clientId"`
	FreelancerID    string  `json:"freelancerID"`
	Title           string  `json:"title"`
	BriefHash       string  `json:"briefHash"`
	Amount          float64 `json:"amount"`
	DepositAmount   float64 `json:"depositAmount"`
	Deadline        string  `json:"deadline"`
	RevisionCount   int     `json:"revisionCount"`
	DeliverableHash string  `json:"deliverableHash"`
	State           string  `json:"state"`
	DisputeReason   string  `json:"disputeReason,omitempty"`
	AdminDecision   string  `json:"adminDecision,omitempty"`
	CreatedAt       string  `json:"createdAt"`
	DeliveredAt     string  `json:"deliveredAt,omitempty"`
	AutoReleaseAt   string  `json:"autoReleaseAt,omitempty"`
	CompletedAt     string  `json:"completedAt,omitempty"`
}

func (s *SmartContract) CreateContract(ctx contractapi.TransactionContextInterface, id, clientID, freelancerID, title, briefHash string, amount, deposit float64, deadline string, revisions int) error {
	exists, err := s.Exists(ctx, id)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("contract %s already exists", id)
	}

	c := EscrowContract{
		ID:            id,
		ClientID:      clientID,
		FreelancerID:  freelancerID,
		Title:         title,
		BriefHash:     briefHash,
		Amount:        amount,
		DepositAmount: deposit,
		Deadline:      deadline,
		RevisionCount: revisions,
		State:         "PENDING_FREELANCER",
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
	}

	return s.putContract(ctx, c)
}

func (s *SmartContract) FreelancerAccept(ctx contractapi.TransactionContextInterface, id string) error {
	c, err := s.getContract(ctx, id)
	if err != nil {
		return err
	}
	if c.State != "PENDING_FREELANCER" {
		return fmt.Errorf("invalid state transition from %s to PENDING_DEPOSIT", c.State)
	}
	c.State = "PENDING_DEPOSIT"
	return s.putContract(ctx, *c)
}

func (s *SmartContract) RecordDeposit(ctx contractapi.TransactionContextInterface, id, paymentRef string) error {
	c, err := s.getContract(ctx, id)
	if err != nil {
		return err
	}
	if c.State != "PENDING_DEPOSIT" {
		return fmt.Errorf("invalid state transition from %s to DEPOSIT_PENDING_CONFIRM", c.State)
	}
	c.State = "DEPOSIT_PENDING_CONFIRM"
	return s.putContract(ctx, *c)
}

func (s *SmartContract) ConfirmDeposit(ctx contractapi.TransactionContextInterface, id string) error {
	msp, err := ctx.GetClientIdentity().GetMSPID()
	if err != nil || msp != "PlatformMSP" {
		return fmt.Errorf("unauthorized")
	}

	c, err := s.getContract(ctx, id)
	if err != nil {
		return err
	}
	if c.State != "DEPOSIT_PENDING_CONFIRM" {
		return fmt.Errorf("invalid state transition from %s to FUNDED", c.State)
	}
	c.State = "FUNDED"
	return s.putContract(ctx, *c)
}

func (s *SmartContract) RecordDelivery(ctx contractapi.TransactionContextInterface, id, fileHash string) error {
	c, err := s.getContract(ctx, id)
	if err != nil {
		return err
	}
	if c.State != "FUNDED" {
		return fmt.Errorf("invalid state transition from %s to DELIVERED", c.State)
	}
	c.State = "DELIVERED"
	c.DeliverableHash = fileHash
	c.DeliveredAt = time.Now().UTC().Format(time.RFC3339)

	autoReleaseTime := time.Now().UTC().Add(5 * 24 * time.Hour)
	c.AutoReleaseAt = autoReleaseTime.Format(time.RFC3339)

	return s.putContract(ctx, *c)
}

func (s *SmartContract) ApproveDelivery(ctx contractapi.TransactionContextInterface, id string) error {
	c, err := s.getContract(ctx, id)
	if err != nil {
		return err
	}
	if c.State != "DELIVERED" && c.State != "REVISION_REQUESTED" {
		return fmt.Errorf("invalid state transition from %s to COMPLETED", c.State)
	}
	c.State = "COMPLETED"
	c.CompletedAt = time.Now().UTC().Format(time.RFC3339)
	return s.putContract(ctx, *c)
}

func (s *SmartContract) RequestRevision(ctx contractapi.TransactionContextInterface, id string) error {
	c, err := s.getContract(ctx, id)
	if err != nil {
		return err
	}
	if c.State != "DELIVERED" {
		return fmt.Errorf("invalid state transition from %s to REVISION_REQUESTED", c.State)
	}

	if c.RevisionCount <= 0 {
		c.State = "COMPLETED"
		c.CompletedAt = time.Now().UTC().Format(time.RFC3339)
		return s.putContract(ctx, *c)
	}

	c.RevisionCount--
	c.State = "REVISION_REQUESTED"
	return s.putContract(ctx, *c)
}

func (s *SmartContract) RaiseDispute(ctx contractapi.TransactionContextInterface, id, reason string) error {
	c, err := s.getContract(ctx, id)
	if err != nil {
		return err
	}
	if c.State != "DELIVERED" && c.State != "FUNDED" {
		return fmt.Errorf("invalid state transition from %s to DISPUTED", c.State)
	}
	c.State = "DISPUTED"
	c.DisputeReason = reason
	return s.putContract(ctx, *c)
}

func (s *SmartContract) AdminResolve(ctx contractapi.TransactionContextInterface, id, decision string) error {
	msp, err := ctx.GetClientIdentity().GetMSPID()
	if err != nil || msp != "PlatformMSP" {
		return fmt.Errorf("unauthorized")
	}

	c, err := s.getContract(ctx, id)
	if err != nil {
		return err
	}
	if c.State != "DISPUTED" {
		return fmt.Errorf("contract is not in DISPUTED state")
	}
	c.State = "RESOLVED"
	c.AdminDecision = decision
	c.CompletedAt = time.Now().UTC().Format(time.RFC3339)
	return s.putContract(ctx, *c)
}

func (s *SmartContract) AutoRelease(ctx contractapi.TransactionContextInterface, id string) error {
	c, err := s.getContract(ctx, id)
	if err != nil {
		return err
	}
	if c.State != "DELIVERED" {
		return fmt.Errorf("contract is not in DELIVERED state")
	}

	autoTime, err := time.Parse(time.RFC3339, c.AutoReleaseAt)
	if err != nil {
		return fmt.Errorf("invalid auto_release_at: %w", err)
	}
	if time.Now().UTC().Before(autoTime) {
		return fmt.Errorf("auto-release timer has not expired yet")
	}

	c.State = "COMPLETED"
	c.CompletedAt = time.Now().UTC().Format(time.RFC3339)
	return s.putContract(ctx, *c)
}

func (s *SmartContract) GetContract(ctx contractapi.TransactionContextInterface, id string) (*EscrowContract, error) {
	return s.getContract(ctx, id)
}

func (s *SmartContract) Exists(ctx contractapi.TransactionContextInterface, id string) (bool, error) {
	data, err := ctx.GetStub().GetState(id)
	if err != nil {
		return false, err
	}
	return data != nil, nil
}

func (s *SmartContract) getContract(ctx contractapi.TransactionContextInterface, id string) (*EscrowContract, error) {
	data, err := ctx.GetStub().GetState(id)
	if err != nil {
		return nil, fmt.Errorf("read from ledger: %w", err)
	}
	if data == nil {
		return nil, fmt.Errorf("contract %s not found", id)
	}
	var c EscrowContract
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return &c, nil
}

func (s *SmartContract) putContract(ctx contractapi.TransactionContextInterface, c EscrowContract) error {
	data, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return ctx.GetStub().PutState(c.ID, data)
}
