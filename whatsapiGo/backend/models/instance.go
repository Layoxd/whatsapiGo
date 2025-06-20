package models

import (
    "time"
    "go.mau.fi/whatsmeow/types"
)

// Instance - Modelo de base de datos para instancias
type Instance struct {
    ID          string     `json:"id" gorm:"primaryKey"`
    Name        string     `json:"name" gorm:"not null"`
    Phone       string     `json:"phone"`
    Status      string     `json:"status" gorm:"default:'disconnected'"`
    Webhook     string     `json:"webhook"`
    WebhookBase64 bool     `json:"webhook_base64" gorm:"default:false"`
    UserJID     string     `json:"user_jid"`
    CreatedAt   time.Time  `json:"created_at"`
    UpdatedAt   time.Time  `json:"updated_at"`
    LastSeen    *time.Time `json:"last_seen"`
}

// TableName - Nombre de la tabla en PostgreSQL
func (Instance) TableName() string {
    return "instances"
}// Archivo base: instance.go
