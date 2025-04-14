// api/models/file.go

package models

import (
	"time"
)

type File struct {
	ID          int       `json:"id" db:"id"`
	UserID      int       `json:"user_id" db:"user_id"`
	Filename    string    `json:"filename" db:"filename"`
	S3Key       string    `json:"s3_key" db:"s3_key"`
	ContentType string    `json:"content_type" db:"content_type"`
	Size        int64     `json:"size" db:"size"`
	UploadedAt  time.Time `json:"uploaded_at" db:"uploaded_at"`
	Processed   bool      `db:"processed" json:"processed"`
	DownloadURL string    `db:"-" json:"downloadUrl,omitempty"`
}
