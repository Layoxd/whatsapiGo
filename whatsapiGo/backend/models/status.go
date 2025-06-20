package models

import (
    "time"
    "gorm.io/gorm"
)

// Status - Modelo de estado/story
type Status struct {
    ID              string         `json:"id" gorm:"primaryKey"`
    InstanceID      string         `json:"instance_id" gorm:"not null;index"`
    AuthorJID       string         `json:"author_jid" gorm:"not null;index"`
    AuthorLID       string         `json:"author_lid" gorm:"index"`
    AuthorPhone     string         `json:"author_phone" gorm:"not null;index"`
    AuthorName      string         `json:"author_name"`
    Type            string         `json:"type" gorm:"not null"` // text, image, video, audio
    Content         string         `json:"content"`              // Texto del estado
    Caption         string         `json:"caption"`              // Caption para multimedia
    MediaURL        string         `json:"media_url"`            // URL del archivo
    MediaMimeType   string         `json:"media_mime_type"`      // Tipo MIME
    BackgroundColor string         `json:"background_color"`     // Color de fondo
    Font            int            `json:"font"`                 // ID de fuente
    IsOwn           bool           `json:"is_own" gorm:"not null;index"`
    ViewCount       int            `json:"view_count" gorm:"default:0"`
    Privacy         string         `json:"privacy" gorm:"default:'contacts'"` // all, contacts, selected, except
    PublishedAt     time.Time      `json:"published_at" gorm:"not null;index"`
    ExpiresAt       time.Time      `json:"expires_at" gorm:"not null;index"`
    CreatedAt       time.Time      `json:"created_at"`
    UpdatedAt       time.Time      `json:"updated_at"`
    DeletedAt       gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

func (Status) TableName() string {
    return "statuses"
}

// StatusViewer - Modelo de visualizaciones de estado
type StatusViewer struct {
    ID       uint      `json:"id" gorm:"primaryKey"`
    StatusID string    `json:"status_id" gorm:"not null;index"`
    ViewerJID string   `json:"viewer_jid" gorm:"not null"`
    ViewerLID string   `json:"viewer_lid"`
    ViewerPhone string `json:"viewer_phone"`
    ViewerName string  `json:"viewer_name"`
    ViewedAt time.Time `json:"viewed_at" gorm:"not null"`
    CreatedAt time.Time `json:"created_at"`
}

func (StatusViewer) TableName() string {
    return "status_viewers"
}

// StatusPrivacy - Configuraciones de privacidad de estados
type StatusPrivacy struct {
    ID             uint      `json:"id" gorm:"primaryKey"`
    InstanceID     string    `json:"instance_id" gorm:"not null;unique;index"`
    DefaultPrivacy string    `json:"default_privacy" gorm:"default:'contacts'"` // all, contacts, selected, except
    AllowList      string    `json:"allow_list"`      // JSON array de JIDs permitidos
    DenyList       string    `json:"deny_list"`       // JSON array de JIDs bloqueados
    ReadReceipts   bool      `json:"read_receipts" gorm:"default:true"`
    AllowReplies   bool      `json:"allow_replies" gorm:"default:true"`
    CreatedAt      time.Time `json:"created_at"`
    UpdatedAt      time.Time `json:"updated_at"`
}

func (StatusPrivacy) TableName() string {
    return "status_privacy"
}

// StatusAudience - Audiencia específica para estados con privacidad "selected" o "except"
type StatusAudience struct {
    ID       uint   `json:"id" gorm:"primaryKey"`
    StatusID string `json:"status_id" gorm:"not null;index"`
    JID      string `json:"jid" gorm:"not null"`
    LID      string `json:"lid"`
    Phone    string `json:"phone"`
    Type     string `json:"type" gorm:"not null"` // allow, deny
    CreatedAt time.Time `json:"created_at"`
}

func (StatusAudience) TableName() string {
    return "status_audience"
}
