package model

import "time"

type User struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	Phone        string    `json:"phone"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"`
	BankAccount  string    `json:"bankAccount"`
	CreatedAt    time.Time `json:"createdAt"`
}
