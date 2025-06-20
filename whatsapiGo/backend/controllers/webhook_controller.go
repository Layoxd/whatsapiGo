package controllers

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// WebhookController - Controlador para gestión de webhooks empresariales
type WebhookController struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewWebhookController - Constructor para el controlador de webhooks
func NewWebhookController(db *gorm.DB, logger *zap.Logger) *WebhookController {
	return &WebhookController{
		db:     db,
		logger: logger,
	}
}

// Estructura para configuración de webhook
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

// Estructura para métricas de webhook
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

// Estructura para logs de eventos de webhook
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

// Estructura para configuración de rechazo de llamadas
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

// Estructuras para requests y responses

// ConfigureWebhookRequest - Request para configurar webhook principal
type ConfigureWebhookRequest struct {
	URL        string   `json:"url" binding:"required,url"`
	Events     []string `json:"events" binding:"required"`
	Secret     string   `json:"secret,omitempty"`
	MaxRetries int      `json:"max_retries,omitempty"`
	Timeout    int      `json:"timeout,omitempty"`
}

// AddWebhookRequest - Request para agregar webhook adicional
type AddWebhookRequest struct {
	URL        string   `json:"url" binding:"required,url"`
	Events     []string `json:"events" binding:"required"`
	Secret     string   `json:"secret,omitempty"`
	MaxRetries int      `json:"max_retries,omitempty"`
	Timeout    int      `json:"timeout,omitempty"`
}

// UpdateWebhookRequest - Request para actualizar webhook
type UpdateWebhookRequest struct {
	URL        string   `json:"url,omitempty"`
	Events     []string `json:"events,omitempty"`
	IsActive   *bool    `json:"is_active,omitempty"`
	MaxRetries int      `json:"max_retries,omitempty"`
	Timeout    int      `json:"timeout,omitempty"`
}

// WebhookFiltersRequest - Request para configurar filtros
type WebhookFiltersRequest struct {
	WebhookID     string   `json:"webhook_id" binding:"required"`
	EventTypes    []string `json:"event_types,omitempty"`
	ContactFilter []string `json:"contact_filter,omitempty"` // JIDs específicos
	GroupFilter   []string `json:"group_filter,omitempty"`   // Group IDs específicos
	Keywords      []string `json:"keywords,omitempty"`       // Palabras clave en mensajes
}

// CallRejectRequest - Request para configurar rechazo de llamadas
type CallRejectRequest struct {
	AutoReject        bool                   `json:"auto_reject"`
	WhitelistNumbers  []string               `json:"whitelist_numbers,omitempty"`
	CustomMessages    map[string]string      `json:"custom_messages,omitempty"`
	ScheduleEnabled   bool                   `json:"schedule_enabled,omitempty"`
	ScheduleConfig    map[string]interface{} `json:"schedule_config,omitempty"`
}

// WebhookResponse - Response genérica para operaciones de webhook
type WebhookResponse struct {
	Success   bool        `json:"success"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data,omitempty"`
	WebhookID string      `json:"webhook_id,omitempty"`
}

// MetricsResponse - Response para métricas de webhooks
type MetricsResponse struct {
	InstanceID      string    `json:"instance_id"`
	TotalWebhooks   int       `json:"total_webhooks"`
	ActiveWebhooks  int       `json:"active_webhooks"`
	TotalSent       int64     `json:"total_sent"`
	TotalSuccess    int64     `json:"total_success"`
	TotalFailed     int64     `json:"total_failed"`
	OverallSuccessRate float64 `json:"overall_success_rate"`
	AvgResponseTime float64   `json:"avg_response_time"`
	LastEvent       time.Time `json:"last_event"`
	WebhookDetails  []WebhookMetrics `json:"webhook_details"`
}

// =============================================================================
// ENDPOINTS DE GESTIÓN DE WEBHOOKS
// =============================================================================

// ConfigureWebhook - POST /webhooks/{instanceId}/configure
// Configurar webhook principal para la instancia
func (wc *WebhookController) ConfigureWebhook(c *gin.Context) {
	instanceID := c.Param("instanceId")
	
	var request ConfigureWebhookRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		wc.logger.Error("Error validating webhook configuration request", 
			zap.String("instanceId", instanceID), 
			zap.Error(err))
		c.JSON(http.StatusBadRequest, WebhookResponse{
			Success: false,
			Message: "Datos de configuración inválidos: " + err.Error(),
		})
		return
	}

	// Generar secret si no se proporciona
	if request.Secret == "" {
		request.Secret = wc.generateSecret()
	}

	// Valores por defecto
	if request.MaxRetries == 0 {
		request.MaxRetries = 5
	}
	if request.Timeout == 0 {
		request.Timeout = 30
	}

	// Convertir eventos a JSON
	eventsJSON, _ := json.Marshal(request.Events)

	// Verificar si ya existe webhook principal
	var existingWebhook WebhookConfig
	result := wc.db.Where("instance_id = ? AND webhook_id = ?", instanceID, "primary").First(&existingWebhook)
	
	webhookID := "primary"
	if result.Error == gorm.ErrRecordNotFound {
		// Crear nuevo webhook
		webhook := WebhookConfig{
			InstanceID:  instanceID,
			WebhookID:   webhookID,
			URL:         request.URL,
			Secret:      request.Secret,
			Events:      string(eventsJSON),
			IsActive:    true,
			MaxRetries:  request.MaxRetries,
			Timeout:     request.Timeout,
		}

		if err := wc.db.Create(&webhook).Error; err != nil {
			wc.logger.Error("Error creating webhook configuration", 
				zap.String("instanceId", instanceID), 
				zap.Error(err))
			c.JSON(http.StatusInternalServerError, WebhookResponse{
				Success: false,
				Message: "Error al crear configuración de webhook",
			})
			return
		}

		// Crear métricas iniciales
		metrics := WebhookMetrics{
			WebhookID:   webhookID,
			InstanceID:  instanceID,
			TotalSent:   0,
			TotalSuccess: 0,
			TotalFailed: 0,
			SuccessRate: 0,
		}
		wc.db.Create(&metrics)

	} else {
		// Actualizar webhook existente
		existingWebhook.URL = request.URL
		existingWebhook.Events = string(eventsJSON)
		existingWebhook.MaxRetries = request.MaxRetries
		existingWebhook.Timeout = request.Timeout
		existingWebhook.IsActive = true

		if err := wc.db.Save(&existingWebhook).Error; err != nil {
			wc.logger.Error("Error updating webhook configuration", 
				zap.String("instanceId", instanceID), 
				zap.Error(err))
			c.JSON(http.StatusInternalServerError, WebhookResponse{
				Success: false,
				Message: "Error al actualizar configuración de webhook",
			})
			return
		}
	}

	wc.logger.Info("Webhook configured successfully", 
		zap.String("instanceId", instanceID),
		zap.String("webhookId", webhookID),
		zap.String("url", request.URL))

	c.JSON(http.StatusOK, WebhookResponse{
		Success:   true,
		Message:   "Webhook configurado exitosamente",
		WebhookID: webhookID,
		Data: gin.H{
			"instance_id": instanceID,
			"webhook_id":  webhookID,
			"url":         request.URL,
			"events":      request.Events,
			"secret":      request.Secret,
		},
	})
}

// AddWebhook - POST /webhooks/{instanceId}/add
// Agregar webhook adicional a la instancia
func (wc *WebhookController) AddWebhook(c *gin.Context) {
	instanceID := c.Param("instanceId")
	
	var request AddWebhookRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		wc.logger.Error("Error validating add webhook request", 
			zap.String("instanceId", instanceID), 
			zap.Error(err))
		c.JSON(http.StatusBadRequest, WebhookResponse{
			Success: false,
			Message: "Datos de webhook inválidos: " + err.Error(),
		})
		return
	}

	// Verificar límite de webhooks por instancia (máximo 10)
	var count int64
	wc.db.Model(&WebhookConfig{}).Where("instance_id = ?", instanceID).Count(&count)
	if count >= 10 {
		c.JSON(http.StatusBadRequest, WebhookResponse{
			Success: false,
			Message: "Límite de webhooks alcanzado (máximo 10 por instancia)",
		})
		return
	}

	// Generar ID único para webhook
	webhookID := uuid.New().String()

	// Generar secret si no se proporciona
	if request.Secret == "" {
		request.Secret = wc.generateSecret()
	}

	// Valores por defecto
	if request.MaxRetries == 0 {
		request.MaxRetries = 5
	}
	if request.Timeout == 0 {
		request.Timeout = 30
	}

	// Convertir eventos a JSON
	eventsJSON, _ := json.Marshal(request.Events)

	// Crear webhook
	webhook := WebhookConfig{
		InstanceID:  instanceID,
		WebhookID:   webhookID,
		URL:         request.URL,
		Secret:      request.Secret,
		Events:      string(eventsJSON),
		IsActive:    true,
		MaxRetries:  request.MaxRetries,
		Timeout:     request.Timeout,
	}

	if err := wc.db.Create(&webhook).Error; err != nil {
		wc.logger.Error("Error creating additional webhook", 
			zap.String("instanceId", instanceID), 
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, WebhookResponse{
			Success: false,
			Message: "Error al crear webhook adicional",
		})
		return
	}

	// Crear métricas iniciales
	metrics := WebhookMetrics{
		WebhookID:   webhookID,
		InstanceID:  instanceID,
		TotalSent:   0,
		TotalSuccess: 0,
		TotalFailed: 0,
		SuccessRate: 0,
	}
	wc.db.Create(&metrics)

	wc.logger.Info("Additional webhook created successfully", 
		zap.String("instanceId", instanceID),
		zap.String("webhookId", webhookID),
		zap.String("url", request.URL))

	c.JSON(http.StatusCreated, WebhookResponse{
		Success:   true,
		Message:   "Webhook adicional creado exitosamente",
		WebhookID: webhookID,
		Data: gin.H{
			"instance_id": instanceID,
			"webhook_id":  webhookID,
			"url":         request.URL,
			"events":      request.Events,
		},
	})
}

// ListWebhooks - GET /webhooks/{instanceId}
// Listar todos los webhooks configurados para la instancia
func (wc *WebhookController) ListWebhooks(c *gin.Context) {
	instanceID := c.Param("instanceId")

	var webhooks []WebhookConfig
	if err := wc.db.Where("instance_id = ?", instanceID).Find(&webhooks).Error; err != nil {
		wc.logger.Error("Error listing webhooks", 
			zap.String("instanceId", instanceID), 
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, WebhookResponse{
			Success: false,
			Message: "Error al obtener lista de webhooks",
		})
		return
	}

	// Convertir eventos de JSON a array para cada webhook
	var webhookList []gin.H
	for _, webhook := range webhooks {
		var events []string
		json.Unmarshal([]byte(webhook.Events), &events)

		webhookList = append(webhookList, gin.H{
			"webhook_id":  webhook.WebhookID,
			"url":         webhook.URL,
			"events":      events,
			"is_active":   webhook.IsActive,
			"max_retries": webhook.MaxRetries,
			"timeout":     webhook.Timeout,
			"created_at":  webhook.CreatedAt,
			"updated_at":  webhook.UpdatedAt,
		})
	}

	c.JSON(http.StatusOK, WebhookResponse{
		Success: true,
		Message: "Lista de webhooks obtenida exitosamente",
		Data: gin.H{
			"instance_id": instanceID,
			"total":       len(webhooks),
			"webhooks":    webhookList,
		},
	})
}

// UpdateWebhook - PUT /webhooks/{instanceId}/{webhookId}
// Actualizar webhook específico
func (wc *WebhookController) UpdateWebhook(c *gin.Context) {
	instanceID := c.Param("instanceId")
	webhookID := c.Param("webhookId")

	var request UpdateWebhookRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, WebhookResponse{
			Success: false,
			Message: "Datos de actualización inválidos: " + err.Error(),
		})
		return
	}

	// Buscar webhook existente
	var webhook WebhookConfig
	if err := wc.db.Where("instance_id = ? AND webhook_id = ?", instanceID, webhookID).First(&webhook).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, WebhookResponse{
				Success: false,
				Message: "Webhook no encontrado",
			})
			return
		}
		wc.logger.Error("Error finding webhook", 
			zap.String("instanceId", instanceID),
			zap.String("webhookId", webhookID), 
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, WebhookResponse{
			Success: false,
			Message: "Error al buscar webhook",
		})
		return
	}

	// Actualizar campos proporcionados
	if request.URL != "" {
		webhook.URL = request.URL
	}
	if len(request.Events) > 0 {
		eventsJSON, _ := json.Marshal(request.Events)
		webhook.Events = string(eventsJSON)
	}
	if request.IsActive != nil {
		webhook.IsActive = *request.IsActive
	}
	if request.MaxRetries > 0 {
		webhook.MaxRetries = request.MaxRetries
	}
	if request.Timeout > 0 {
		webhook.Timeout = request.Timeout
	}

	if err := wc.db.Save(&webhook).Error; err != nil {
		wc.logger.Error("Error updating webhook", 
			zap.String("instanceId", instanceID),
			zap.String("webhookId", webhookID), 
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, WebhookResponse{
			Success: false,
			Message: "Error al actualizar webhook",
		})
		return
	}

	wc.logger.Info("Webhook updated successfully", 
		zap.String("instanceId", instanceID),
		zap.String("webhookId", webhookID))

	c.JSON(http.StatusOK, WebhookResponse{
		Success: true,
		Message: "Webhook actualizado exitosamente",
		Data: gin.H{
			"webhook_id": webhookID,
			"updated_at": time.Now(),
		},
	})
}

// DeleteWebhook - DELETE /webhooks/{instanceId}/{webhookId}
// Eliminar webhook específico
func (wc *WebhookController) DeleteWebhook(c *gin.Context) {
	instanceID := c.Param("instanceId")
	webhookID := c.Param("webhookId")

	// No permitir eliminar webhook principal
	if webhookID == "primary" {
		c.JSON(http.StatusBadRequest, WebhookResponse{
			Success: false,
			Message: "No se puede eliminar el webhook principal. Usa el endpoint de configuración para desactivarlo.",
		})
		return
	}

	// Eliminar webhook y sus métricas asociadas
	tx := wc.db.Begin()
	
	if err := tx.Where("instance_id = ? AND webhook_id = ?", instanceID, webhookID).Delete(&WebhookConfig{}).Error; err != nil {
		tx.Rollback()
		wc.logger.Error("Error deleting webhook", 
			zap.String("instanceId", instanceID),
			zap.String("webhookId", webhookID), 
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, WebhookResponse{
			Success: false,
			Message: "Error al eliminar webhook",
		})
		return
	}

	// Eliminar métricas y logs asociados
	tx.Where("instance_id = ? AND webhook_id = ?", instanceID, webhookID).Delete(&WebhookMetrics{})
	tx.Where("instance_id = ? AND webhook_id = ?", instanceID, webhookID).Delete(&WebhookLog{})

	tx.Commit()

	wc.logger.Info("Webhook deleted successfully", 
		zap.String("instanceId", instanceID),
		zap.String("webhookId", webhookID))

	c.JSON(http.StatusOK, WebhookResponse{
		Success: true,
		Message: "Webhook eliminado exitosamente",
	})
}

// TestWebhook - POST /webhooks/{instanceId}/test
// Probar conectividad de webhooks
func (wc *WebhookController) TestWebhook(c *gin.Context) {
	instanceID := c.Param("instanceId")

	// Obtener todos los webhooks activos
	var webhooks []WebhookConfig
	if err := wc.db.Where("instance_id = ? AND is_active = ?", instanceID, true).Find(&webhooks).Error; err != nil {
		wc.logger.Error("Error finding active webhooks", 
			zap.String("instanceId", instanceID), 
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, WebhookResponse{
			Success: false,
			Message: "Error al buscar webhooks activos",
		})
		return
	}

	if len(webhooks) == 0 {
		c.JSON(http.StatusNotFound, WebhookResponse{
			Success: false,
			Message: "No hay webhooks activos configurados",
		})
		return
	}

	// Preparar payload de test
	testPayload := gin.H{
		"event": "webhook.test",
		"instance_id": instanceID,
		"timestamp": time.Now().Unix(),
		"data": gin.H{
			"message": "Test de conectividad de webhook",
			"test_id": uuid.New().String(),
		},
	}

	results := make([]gin.H, 0)

	// Probar cada webhook
	for _, webhook := range webhooks {
		startTime := time.Now()
		success, statusCode, err := wc.sendWebhookEvent(webhook, testPayload)
		responseTime := time.Since(startTime).Milliseconds()

		result := gin.H{
			"webhook_id":    webhook.WebhookID,
			"url":          webhook.URL,
			"success":      success,
			"status_code":  statusCode,
			"response_time": responseTime,
		}

		if err != nil {
			result["error"] = err.Error()
		}

		results = append(results, result)

		wc.logger.Info("Webhook test completed", 
			zap.String("instanceId", instanceID),
			zap.String("webhookId", webhook.WebhookID),
			zap.Bool("success", success),
			zap.Int("statusCode", statusCode),
			zap.Int64("responseTime", responseTime))
	}

	c.JSON(http.StatusOK, WebhookResponse{
		Success: true,
		Message: "Test de webhooks completado",
		Data: gin.H{
			"instance_id": instanceID,
			"tested_webhooks": len(webhooks),
			"results": results,
		},
	})
}

// GetMetrics - GET /webhooks/{instanceId}/metrics
// Obtener métricas de delivery y performance
func (wc *WebhookController) GetMetrics(c *gin.Context) {
	instanceID := c.Param("instanceId")

	// Obtener métricas de todos los webhooks
	var metrics []WebhookMetrics
	if err := wc.db.Where("instance_id = ?", instanceID).Find(&metrics).Error; err != nil {
		wc.logger.Error("Error getting webhook metrics", 
			zap.String("instanceId", instanceID), 
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, WebhookResponse{
			Success: false,
			Message: "Error al obtener métricas de webhooks",
		})
		return
	}

	// Calcular métricas generales
	var totalSent, totalSuccess, totalFailed int64
	var totalResponseTime float64
	var lastEvent time.Time
	activeWebhooks := 0

	for _, metric := range metrics {
		totalSent += metric.TotalSent
		totalSuccess += metric.TotalSuccess
		totalFailed += metric.TotalFailed
		totalResponseTime += metric.AvgResponseTime

		if metric.LastEvent.After(lastEvent) {
			lastEvent = metric.LastEvent
		}
	}

	// Contar webhooks activos
	var activeCount int64
	wc.db.Model(&WebhookConfig{}).Where("instance_id = ? AND is_active = ?", instanceID, true).Count(&activeCount)
	activeWebhooks = int(activeCount)

	// Calcular tasa de éxito general
	var overallSuccessRate float64
	if totalSent > 0 {
		overallSuccessRate = (float64(totalSuccess) / float64(totalSent)) * 100
	}

	// Calcular tiempo de respuesta promedio
	var avgResponseTime float64
	if len(metrics) > 0 {
		avgResponseTime = totalResponseTime / float64(len(metrics))
	}

	response := MetricsResponse{
		InstanceID:         instanceID,
		TotalWebhooks:      len(metrics),
		ActiveWebhooks:     activeWebhooks,
		TotalSent:          totalSent,
		TotalSuccess:       totalSuccess,
		TotalFailed:        totalFailed,
		OverallSuccessRate: overallSuccessRate,
		AvgResponseTime:    avgResponseTime,
		LastEvent:          lastEvent,
		WebhookDetails:     metrics,
	}

	c.JSON(http.StatusOK, WebhookResponse{
		Success: true,
		Message: "Métricas obtenidas exitosamente",
		Data:    response,
	})
}

// RetryEvent - POST /webhooks/{instanceId}/retry/{eventId}
// Reintentar evento específico
func (wc *WebhookController) RetryEvent(c *gin.Context) {
	instanceID := c.Param("instanceId")
	eventID := c.Param("eventId")

	// Buscar evento en logs
	var log WebhookLog
	if err := wc.db.Where("instance_id = ? AND event_id = ?", instanceID, eventID).First(&log).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, WebhookResponse{
				Success: false,
				Message: "Evento no encontrado",
			})
			return
		}
		wc.logger.Error("Error finding event log", 
			zap.String("instanceId", instanceID),
			zap.String("eventId", eventID), 
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, WebhookResponse{
			Success: false,
			Message: "Error al buscar evento",
		})
		return
	}

	// Verificar si ya fue exitoso
	if log.IsSuccess {
		c.JSON(http.StatusBadRequest, WebhookResponse{
			Success: false,
			Message: "El evento ya fue entregado exitosamente",
		})
		return
	}

	// Buscar configuración del webhook
	var webhook WebhookConfig
	if err := wc.db.Where("instance_id = ? AND webhook_id = ?", instanceID, log.WebhookID).First(&webhook).Error; err != nil {
		c.JSON(http.StatusNotFound, WebhookResponse{
			Success: false,
			Message: "Configuración de webhook no encontrada",
		})
		return
	}

	// Deserializar payload
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(log.Payload), &payload); err != nil {
		c.JSON(http.StatusInternalServerError, WebhookResponse{
			Success: false,
			Message: "Error al procesar payload del evento",
		})
		return
	}

	// Intentar reenvío
	startTime := time.Now()
	success, statusCode, err := wc.sendWebhookEvent(webhook, payload)
	responseTime := time.Since(startTime).Milliseconds()

	// Actualizar log
	log.AttemptCount++
	log.StatusCode = statusCode
	log.ResponseTime = int(responseTime)
	log.IsSuccess = success
	log.UpdatedAt = time.Now()

	if err != nil {
		log.ErrorMessage = err.Error()
	} else {
		log.ErrorMessage = ""
	}

	wc.db.Save(&log)

	// Actualizar métricas si fue exitoso
	if success {
		wc.updateWebhookMetrics(webhook.WebhookID, instanceID, true, responseTime)
	}

	wc.logger.Info("Event retry completed", 
		zap.String("instanceId", instanceID),
		zap.String("eventId", eventID),
		zap.Bool("success", success),
		zap.Int("statusCode", statusCode))

	message := "Evento reenviado exitosamente"
	if !success {
		message = "Error al reenviar evento"
	}

	c.JSON(http.StatusOK, WebhookResponse{
		Success: success,
		Message: message,
		Data: gin.H{
			"event_id":      eventID,
			"success":       success,
			"status_code":   statusCode,
			"response_time": responseTime,
			"attempt_count": log.AttemptCount,
		},
	})
}

// GetLogs - GET /webhooks/{instanceId}/logs
// Obtener logs detallados de eventos
func (wc *WebhookController) GetLogs(c *gin.Context) {
	instanceID := c.Param("instanceId")

	// Parámetros de paginación
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset := (page - 1) * limit

	// Filtros opcionales
	webhookID := c.Query("webhook_id")
	eventType := c.Query("event_type")
	isSuccess := c.Query("success")

	// Construir query
	query := wc.db.Where("instance_id = ?", instanceID)
	
	if webhookID != "" {
		query = query.Where("webhook_id = ?", webhookID)
	}
	if eventType != "" {
		query = query.Where("event_type = ?", eventType)
	}
	if isSuccess != "" {
		success := isSuccess == "true"
		query = query.Where("is_success = ?", success)
	}

	// Obtener total de registros
	var total int64
	query.Model(&WebhookLog{}).Count(&total)

	// Obtener logs paginados
	var logs []WebhookLog
	if err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&logs).Error; err != nil {
		wc.logger.Error("Error getting webhook logs", 
			zap.String("instanceId", instanceID), 
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, WebhookResponse{
			Success: false,
			Message: "Error al obtener logs de webhooks",
		})
		return
	}

	c.JSON(http.StatusOK, WebhookResponse{
		Success: true,
		Message: "Logs obtenidos exitosamente",
		Data: gin.H{
			"instance_id": instanceID,
			"total":       total,
			"page":        page,
			"limit":       limit,
			"logs":        logs,
		},
	})
}

// ConfigureFilters - POST /webhooks/{instanceId}/filters
// Configurar filtros de eventos para webhook específico
func (wc *WebhookController) ConfigureFilters(c *gin.Context) {
	instanceID := c.Param("instanceId")

	var request WebhookFiltersRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, WebhookResponse{
			Success: false,
			Message: "Datos de filtros inválidos: " + err.Error(),
		})
		return
	}

	// Verificar que el webhook existe
	var webhook WebhookConfig
	if err := wc.db.Where("instance_id = ? AND webhook_id = ?", instanceID, request.WebhookID).First(&webhook).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, WebhookResponse{
				Success: false,
				Message: "Webhook no encontrado",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, WebhookResponse{
			Success: false,
			Message: "Error al buscar webhook",
		})
		return
	}

	// TODO: Implementar lógica de filtros en tabla separada o en campo JSON del webhook
	// Por ahora almacenamos en un campo JSON en la configuración del webhook
	
	filters := gin.H{
		"event_types":    request.EventTypes,
		"contact_filter": request.ContactFilter,
		"group_filter":   request.GroupFilter,
		"keywords":       request.Keywords,
	}

	filtersJSON, _ := json.Marshal(filters)
	
	// Actualizar webhook con filtros (usar campo extra o nueva tabla)
	wc.db.Model(&webhook).Update("events", string(filtersJSON))

	wc.logger.Info("Webhook filters configured", 
		zap.String("instanceId", instanceID),
		zap.String("webhookId", request.WebhookID))

	c.JSON(http.StatusOK, WebhookResponse{
		Success: true,
		Message: "Filtros de webhook configurados exitosamente",
		Data: gin.H{
			"webhook_id": request.WebhookID,
			"filters":    filters,
		},
	})
}

// =============================================================================
// ENDPOINTS DE GESTIÓN DE LLAMADAS
// =============================================================================

// RejectCall - POST /calls/{instanceId}/reject
// Rechazar llamada entrante con mensaje personalizado
func (wc *WebhookController) RejectCall(c *gin.Context) {
	instanceID := c.Param("instanceId")

	var request struct {
		CallerID string `json:"caller_id" binding:"required"`
		Message  string `json:"message,omitempty"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, WebhookResponse{
			Success: false,
			Message: "Datos de rechazo inválidos: " + err.Error(),
		})
		return
	}

	// Obtener configuración de rechazo
	var config CallRejectConfig
	wc.db.Where("instance_id = ?", instanceID).First(&config)

	// Mensaje por defecto si no se especifica
	message := request.Message
	if message == "" {
		if config.CustomMessages != "" {
			var customMessages map[string]string
			json.Unmarshal([]byte(config.CustomMessages), &customMessages)
			if defaultMsg, exists := customMessages["default"]; exists {
				message = defaultMsg
			} else {
				message = "Lo siento, no puedo atender llamadas en este momento. Por favor envía un mensaje de texto."
			}
		} else {
			message = "Lo siento, no puedo atender llamadas en este momento. Por favor envía un mensaje de texto."
		}
	}

	// TODO: Integrar con WhatsMeow para rechazar llamada real
	// whatsappService.RejectCall(instanceID, request.CallerID, message)

	wc.logger.Info("Call rejected", 
		zap.String("instanceId", instanceID),
		zap.String("callerId", request.CallerID),
		zap.String("message", message))

	c.JSON(http.StatusOK, WebhookResponse{
		Success: true,
		Message: "Llamada rechazada exitosamente",
		Data: gin.H{
			"caller_id": request.CallerID,
			"message":   message,
			"timestamp": time.Now(),
		},
	})
}

// GetCallSettings - GET /calls/{instanceId}/settings
// Obtener configuración de auto-rechazo de llamadas
func (wc *WebhookController) GetCallSettings(c *gin.Context) {
	instanceID := c.Param("instanceId")

	var config CallRejectConfig
	if err := wc.db.Where("instance_id = ?", instanceID).First(&config).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Devolver configuración por defecto
			c.JSON(http.StatusOK, WebhookResponse{
				Success: true,
				Message: "Configuración por defecto (sin configuración previa)",
				Data: gin.H{
					"auto_reject":        false,
					"whitelist_numbers":  []string{},
					"custom_messages":    map[string]string{},
					"schedule_enabled":   false,
					"schedule_config":    map[string]interface{}{},
				},
			})
			return
		}
		c.JSON(http.StatusInternalServerError, WebhookResponse{
			Success: false,
			Message: "Error al obtener configuración",
		})
		return
	}

	// Deserializar configuraciones JSON
	var whitelistNumbers []string
	var customMessages map[string]string
	var scheduleConfig map[string]interface{}

	if config.WhitelistNumbers != "" {
		json.Unmarshal([]byte(config.WhitelistNumbers), &whitelistNumbers)
	}
	if config.CustomMessages != "" {
		json.Unmarshal([]byte(config.CustomMessages), &customMessages)
	}
	if config.ScheduleConfig != "" {
		json.Unmarshal([]byte(config.ScheduleConfig), &scheduleConfig)
	}

	c.JSON(http.StatusOK, WebhookResponse{
		Success: true,
		Message: "Configuración obtenida exitosamente",
		Data: gin.H{
			"auto_reject":        config.AutoReject,
			"whitelist_numbers":  whitelistNumbers,
			"custom_messages":    customMessages,
			"schedule_enabled":   config.ScheduleEnabled,
			"schedule_config":    scheduleConfig,
			"updated_at":         config.UpdatedAt,
		},
	})
}

// UpdateCallSettings - PUT /calls/{instanceId}/settings
// Actualizar configuración de llamadas
func (wc *WebhookController) UpdateCallSettings(c *gin.Context) {
	instanceID := c.Param("instanceId")

	var request CallRejectRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, WebhookResponse{
			Success: false,
			Message: "Datos de configuración inválidos: " + err.Error(),
		})
		return
	}

	// Buscar configuración existente o crear nueva
	var config CallRejectConfig
	isNew := false
	if err := wc.db.Where("instance_id = ?", instanceID).First(&config).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			config = CallRejectConfig{InstanceID: instanceID}
			isNew = true
		} else {
			c.JSON(http.StatusInternalServerError, WebhookResponse{
				Success: false,
				Message: "Error al buscar configuración",
			})
			return
		}
	}

	// Actualizar campos
	config.AutoReject = request.AutoReject

	if request.WhitelistNumbers != nil {
		whitelistJSON, _ := json.Marshal(request.WhitelistNumbers)
		config.WhitelistNumbers = string(whitelistJSON)
	}

	if request.CustomMessages != nil {
		messagesJSON, _ := json.Marshal(request.CustomMessages)
		config.CustomMessages = string(messagesJSON)
	}

	config.ScheduleEnabled = request.ScheduleEnabled

	if request.ScheduleConfig != nil {
		scheduleJSON, _ := json.Marshal(request.ScheduleConfig)
		config.ScheduleConfig = string(scheduleJSON)
	}

	// Guardar configuración
	var err error
	if isNew {
		err = wc.db.Create(&config).Error
	} else {
		err = wc.db.Save(&config).Error
	}

	if err != nil {
		wc.logger.Error("Error saving call reject configuration", 
			zap.String("instanceId", instanceID), 
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, WebhookResponse{
			Success: false,
			Message: "Error al guardar configuración",
		})
		return
	}

	wc.logger.Info("Call reject configuration updated", 
		zap.String("instanceId", instanceID),
		zap.Bool("autoReject", config.AutoReject))

	c.JSON(http.StatusOK, WebhookResponse{
		Success: true,
		Message: "Configuración actualizada exitosamente",
		Data: gin.H{
			"instance_id":  instanceID,
			"auto_reject":  config.AutoReject,
			"updated_at":   config.UpdatedAt,
		},
	})
}

// =============================================================================
// FUNCIONES AUXILIARES
// =============================================================================

// generateSecret - Genera un secret aleatorio para webhook
func (wc *WebhookController) generateSecret() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// sendWebhookEvent - Envía evento a webhook con firma HMAC
func (wc *WebhookController) sendWebhookEvent(webhook WebhookConfig, payload interface{}) (bool, int, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return false, 0, fmt.Errorf("error marshaling payload: %v", err)
	}

	// Generar firma HMAC-SHA256
	mac := hmac.New(sha256.New, []byte(webhook.Secret))
	mac.Write(jsonData)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	// Crear request HTTP
	req, err := http.NewRequest("POST", webhook.URL, bytes.NewBuffer(jsonData))
	if err != nil {
		return false, 0, fmt.Errorf("error creating request: %v", err)
	}

	// Headers requeridos
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", signature)
	req.Header.Set("User-Agent", "WhatsApp-API-Go/1.0")

	// Cliente HTTP con timeout
	client := &http.Client{
		Timeout: time.Duration(webhook.Timeout) * time.Second,
	}

	// Enviar request
	resp, err := client.Do(req)
	if err != nil {
		return false, 0, fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	// Verificar status code exitoso (2xx)
	success := resp.StatusCode >= 200 && resp.StatusCode < 300

	return success, resp.StatusCode, nil
}

// updateWebhookMetrics - Actualiza métricas del webhook
func (wc *WebhookController) updateWebhookMetrics(webhookID, instanceID string, success bool, responseTime int64) {
	var metrics WebhookMetrics
	wc.db.Where("webhook_id = ? AND instance_id = ?", webhookID, instanceID).First(&metrics)

	metrics.TotalSent++
	if success {
		metrics.TotalSuccess++
	} else {
		metrics.TotalFailed++
	}

	// Calcular tasa de éxito
	if metrics.TotalSent > 0 {
		metrics.SuccessRate = (float64(metrics.TotalSuccess) / float64(metrics.TotalSent)) * 100
	}

	// Actualizar tiempo promedio de respuesta
	if metrics.AvgResponseTime == 0 {
		metrics.AvgResponseTime = float64(responseTime)
	} else {
		metrics.AvgResponseTime = (metrics.AvgResponseTime + float64(responseTime)) / 2
	}

	metrics.LastEvent = time.Now()
	wc.db.Save(&metrics)
}

// createWebhookLog - Crea log de evento de webhook
func (wc *WebhookController) createWebhookLog(webhookID, instanceID, eventID, eventType string, payload interface{}, statusCode int, responseTime int64, success bool, errorMsg string) {
	payloadJSON, _ := json.Marshal(payload)

	log := WebhookLog{
		WebhookID:    webhookID,
		InstanceID:   instanceID,
		EventID:      eventID,
		EventType:    eventType,
		Payload:      string(payloadJSON),
		StatusCode:   statusCode,
		ResponseTime: int(responseTime),
		AttemptCount: 1,
		IsSuccess:    success,
		ErrorMessage: errorMsg,
	}

	if !success {
		// Calcular próximo retry con backoff exponencial
		log.NextRetry = time.Now().Add(time.Second * 2) // Primer retry en 2 segundos
	}

	wc.db.Create(&log)
}
