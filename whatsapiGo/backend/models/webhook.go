package models

import (
	"time"
)

// WebhookConfig - Configuración de webhook para instancias
type WebhookConfig struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	InstanceID  string    `json:"instance_id" gorm:"not null;index"`
	WebhookID   string    `json:"webhook_id" gorm:"unique;not null"`
	URL         string    `json:"url" gorm:"not null"`
	Secret      string    `json:"secret" gorm:"not null"`
	Events      string    `json:"events" gorm:"type:text"` // JSON array de eventos
	IsActive    bool      `json:"is_active" gorm:"default:true"`
	MaxRetries  int       `json:"max_retries" gorm:"default:5"`
	Timeout     int       `json:"timeout" gorm:"default:30"` // segundos
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// WebhookMetrics - Métricas de performance de webhooks
type WebhookMetrics struct {
	ID              uint      `json:"id" gorm:"primaryKey"`
	WebhookID       string    `json:"webhook_id" gorm:"not null;index"`
	InstanceID      string    `json:"instance_id" gorm:"not null;index"`
	TotalSent       int64     `json:"total_sent" gorm:"default:0"`
	TotalSuccess    int64     `json:"total_success" gorm:"default:0"`
	TotalFailed     int64     `json:"total_failed" gorm:"default:0"`
	SuccessRate     float64   `json:"success_rate" gorm:"default:0"`
	AvgResponseTime float64   `json:"avg_response_time" gorm:"default:0"` // milisegundos
	LastEvent       time.Time `json:"last_event"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// WebhookLog - Logs detallados de eventos de webhook
type WebhookLog struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	WebhookID    string    `json:"webhook_id" gorm:"not null;index"`
	InstanceID   string    `json:"instance_id" gorm:"not null;index"`
	EventID      string    `json:"event_id" gorm:"not null;unique"`
	EventType    string    `json:"event_type" gorm:"not null"`
	Payload      string    `json:"payload" gorm:"type:text"`
	StatusCode   int       `json:"status_code"`
	ResponseTime int       `json:"response_time"` // milisegundos
	AttemptCount int       `json:"attempt_count" gorm:"default:1"`
	IsSuccess    bool      `json:"is_success" gorm:"default:false"`
	ErrorMessage string    `json:"error_message"`
	NextRetry    time.Time `json:"next_retry"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// CallRejectConfig - Configuración de rechazo automático de llamadas
type CallRejectConfig struct {
	ID                uint      `json:"id" gorm:"primaryKey"`
	InstanceID        string    `json:"instance_id" gorm:"unique;not null"`
	AutoReject        bool      `json:"auto_reject" gorm:"default:true"`
	WhitelistNumbers  string    `json:"whitelist_numbers" gorm:"type:text"` // JSON array
	CustomMessages    string    `json:"custom_messages" gorm:"type:text"`   // JSON object
	ScheduleEnabled   bool      `json:"schedule_enabled" gorm:"default:false"`
	ScheduleConfig    string    `json:"schedule_config" gorm:"type:text"` // JSON object
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}
