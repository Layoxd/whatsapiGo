package models

import (
    "time"
    "gorm.io/gorm"
)

// Contact - Modelo de contacto con soporte LID
type Contact struct {
    ID           uint           `json:"id" gorm:"primaryKey"`
    InstanceID   string         `json:"instance_id" gorm:"not null;index"`
    JID          string         `json:"jid" gorm:"not null;index"`
    LID          string         `json:"lid" gorm:"index"`
    Phone        string         `json:"phone" gorm:"not null;index"`
    Name         string         `json:"name"`
    PushName     string         `json:"push_name"`
    BusinessName string         `json:"business_name"`
    Status       string         `json:"status"`
    AvatarID     string         `json:"avatar_id"`
    IsBlocked    bool           `json:"is_blocked" gorm:"default:false"`
    IsInContacts bool           `json:"is_in_contacts" gorm:"default:false"`
    IsOnWhatsApp bool           `json:"is_on_whatsapp" gorm:"default:true"`
    LastSeen     *time.Time     `json:"last_seen"`
    CreatedAt    time.Time      `json:"created_at"`
    UpdatedAt    time.Time      `json:"updated_at"`
    DeletedAt    gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

func (Contact) TableName() string {
    return "contacts"
}

// ContactBlock - Modelo para contactos bloqueados
type ContactBlock struct {
    ID         uint      `json:"id" gorm:"primaryKey"`
    InstanceID string    `json:"instance_id" gorm:"not null;index"`
    ContactJID string    `json:"contact_jid" gorm:"not null"`
    ContactLID string    `json:"contact_lid"`
    BlockedAt  time.Time `json:"blocked_at"`
    CreatedAt  time.Time `json:"created_at"`
}

func (ContactBlock) TableName() string {
    return "contact_blocks"
}// Archivo base: contact.go
