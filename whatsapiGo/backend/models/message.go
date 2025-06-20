package models

import (
    "time"
    "gorm.io/gorm"
)

// Message - Modelo de base de datos para mensajes
type Message struct {
    ID          string         `json:"id" gorm:"primaryKey"`
    InstanceID  string         `json:"instance_id" gorm:"not null;index"`
    Phone       string         `json:"phone" gorm:"not null;index"`
    IsGroup     bool           `json:"is_group" gorm:"default:false"`
    Type        string         `json:"type" gorm:"not null"` // text, image, video, audio, document, location, contact
    Content     string         `json:"content"`
    Caption     string         `json:"caption"`
    MediaURL    string         `json:"media_url"`
    MimeType    string         `json:"mime_type"`
    FileName    string         `json:"file_name"`
    Status      string         `json:"status" gorm:"default:'sent'"` // sent, delivered, read, failed
    QuotedMsgID string         `json:"quoted_msg_id"`
    Direction   string         `json:"direction" gorm:"not null"` // incoming, outgoing
    Timestamp   time.Time      `json:"timestamp"`
    CreatedAt   time.Time      `json:"created_at"`
    UpdatedAt   time.Time      `json:"updated_at"`
    DeletedAt   gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

// TableName - Nombre de la tabla en PostgreSQL
func (Message) TableName() string {
    return "messages"
}

// MessageDelivery - Modelo para seguimiento de entregas
type MessageDelivery struct {
    ID         uint      `json:"id" gorm:"primaryKey"`
    MessageID  string    `json:"message_id" gorm:"not null;index"`
    Phone      string    `json:"phone" gorm:"not null"`
    Status     string    `json:"status" gorm:"not null"` // sent, delivered, read
    Timestamp  time.Time `json:"timestamp"`
    CreatedAt  time.Time `json:"created_at"`
}

func (MessageDelivery) TableName() string {
    return "message_deliveries"
}
