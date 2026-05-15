package model

import "time"

type File struct {
	ID           string    `json:"id"`
	ContractID   string    `json:"contractId"`
	UploaderID   string    `json:"uploaderId"`
	OriginalName string    `json:"originalName"`
	StoredName   string    `json:"storedName"`
	MimeType     string    `json:"mimeType"`
	SizeBytes    int64     `json:"sizeBytes"`
	SHA256Hash   string    `json:"sha256Hash"`
	CreatedAt    time.Time `json:"createdAt"`
}
