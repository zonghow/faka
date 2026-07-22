package models

import "time"

type Space struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	Name       string    `gorm:"size:20;uniqueIndex;not null" json:"name"`
	CardPrefix string    `gorm:"size:10;uniqueIndex;not null" json:"card_prefix"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (Space) TableName() string { return "spaces" }

type ManagedFile struct {
	ID               uint       `gorm:"primaryKey" json:"id"`
	OriginalName     string     `gorm:"size:255;index;not null" json:"original_name"`
	StoredPath       string     `gorm:"size:500;not null" json:"stored_path"`
	GeneratedAt      time.Time  `gorm:"index" json:"generated_at"`
	UploadedAt       time.Time  `gorm:"index" json:"uploaded_at"`
	SpaceID          uint       `gorm:"index;not null" json:"space_id"`
	SoldAt           *time.Time `gorm:"index" json:"sold_at"`
	LatestDownloadAt *time.Time `gorm:"index" json:"latest_download_at"`
	VoidedAt         *time.Time `gorm:"index" json:"voided_at"`
	Status           string     `gorm:"size:20;index;default:available" json:"status"`
	BatchName        *string    `gorm:"size:255" json:"batch_name"`
	SoldCardID       *uint      `gorm:"index" json:"sold_card_id"`
	SoldCard         *Card      `gorm:"foreignKey:SoldCardID" json:"sold_card,omitempty"`
}

func (ManagedFile) TableName() string { return "files" }

type UploadRecord struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	SpaceID          uint      `gorm:"index;not null" json:"space_id"`
	Filename         string    `gorm:"size:255;not null" json:"filename"`
	CreatedCount     int       `gorm:"not null;default:0" json:"created_count"`
	OverwrittenCount int       `gorm:"not null;default:0" json:"overwritten_count"`
	CreatedAt        time.Time `gorm:"index" json:"created_at"`
}

func (UploadRecord) TableName() string { return "upload_records" }

type Card struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	Code      string     `gorm:"size:128;uniqueIndex;not null" json:"code"`
	SpaceID   uint       `gorm:"index;not null" json:"space_id"`
	FileCount int        `gorm:"not null" json:"file_count"`
	Status    string     `gorm:"size:20;index;default:available" json:"status"`
	CreatedAt time.Time  `gorm:"index" json:"created_at"`
	UsedAt    *time.Time `gorm:"index" json:"used_at"`
	VoidedAt  *time.Time `gorm:"index" json:"voided_at"`
}

func (Card) TableName() string { return "cards" }

type Redemption struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	CardID       uint      `gorm:"index;not null" json:"card_id"`
	RedeemedAt   time.Time `gorm:"index" json:"redeemed_at"`
	OutputFormat string    `gorm:"size:20;index;default:cpa" json:"output_format"`
	DownloadPath string    `gorm:"size:500;not null" json:"download_path"`
	FileIDs      string    `gorm:"type:text;not null" json:"file_ids"`
	Status       string    `gorm:"size:20;index;default:completed" json:"status"`
}

func (Redemption) TableName() string { return "redemptions" }

type AuditLog struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	SpaceID    *uint     `gorm:"index" json:"space_id"`
	Action     string    `gorm:"size:80;index;not null" json:"action"`
	TargetType string    `gorm:"size:80;index;not null" json:"target_type"`
	TargetID   *uint     `gorm:"index" json:"target_id"`
	Detail     *string   `gorm:"type:text" json:"detail"`
	CreatedAt  time.Time `gorm:"index" json:"created_at"`
}

func (AuditLog) TableName() string { return "audit_logs" }
