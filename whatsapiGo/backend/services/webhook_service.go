package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Layoxd/whatsapiGo/models"
	"github.com/google/uuid"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// WebhookService - Servicio para gestión de webhooks y eventos de WhatsMeow
type WebhookService struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewWebhookService - Constructor para WebhookService
func NewWebhookService(db *gorm.DB, logger *zap.Logger) *WebhookService {
	return &WebhookService{
		db:     db,
		logger: logger,
	}
}

// EventHandler - Manejador de eventos de WhatsMeow para webhooks
func (ws *WebhookService) EventHandler(instanceID string) func(interface{}) {
	return func(evt interface{}) {
		ws.processEvent(instanceID, evt)
	}
}

// processEvent - Procesa eventos de WhatsMeow y los envía a webhooks configurados
func (ws *WebhookService) processEvent(instanceID string, evt interface{}) {
	var eventType string
	var eventData map[string]interface{}

	// Identificar tipo de evento y extraer datos
	switch v := evt.(type) {
	case *events.Message:
		eventType = "message.received"
		eventData = ws.extractMessageData(v)
	case *events.Receipt:
		eventType = "message.receipt"
		eventData = ws.extractReceiptData(v)
	case *events.CallOffer:
		eventType = "call.incoming"
		eventData = ws.extractCallOfferData(v)
		// Verificar auto-rechazo de llamadas
		ws.handleAutoRejectCall(instanceID, v)
	case *events.GroupInfo:
		eventType = "group.info"
		eventData = ws.extractGroupInfoData(v)
	case *events.JoinedGroup:
		eventType = "group.joined"
		eventData = ws.extractJoinedGroupData(v)
	case *events.Contact:
		eventType = "contact.update"
		eventData = ws.extractContactData(v)
	case *events.PushName:
		eventType = "contact.pushname"
		eventData = ws.extractPushNameData(v)
	case *events.Presence:
		eventType = "contact.presence"
		eventData = ws.extractPresenceData(v)
	case *events.Connected:
		eventType = "instance.connected"
		eventData = map[string]interface{}{
			"timestamp": time.Now().Unix(),
		}
	case *events.Disconnected:
		eventType = "instance.disconnected"
		eventData = map[string]interface{}{
			"timestamp": time.Now().Unix(),
		}
	case *events.QR:
		eventType = "instance.qr"
		eventData = map[string]interface{}{
			"qr_code":   v.Codes,
			"timestamp": time.Now().Unix(),
		}
	case *events.PairSuccess:
		eventType = "instance.paired"
		eventData = map[string]interface{}{
			"id":        v.ID.String(),
			"timestamp": time.Now().Unix(),
		}
	default:
		// Evento no reconocido, salir
		return
	}

	// Obtener webhooks activos para esta instancia
	var webhooks []models.WebhookConfig
	if err := ws.db.Where("instance_id = ? AND is_active = ?", instanceID, true).Find(&webhooks).Error; err != nil {
		ws.logger.Error("Error obteniendo webhooks activos",
			zap.String("instanceId", instanceID),
			zap.Error(err))
		return
	}

	// Enviar evento a cada webhook configurado
	for _, webhook := range webhooks {
		// Verificar si el webhook está configurado para este tipo de evento
		if !ws.isEventConfigured(webhook, eventType) {
			continue
		}

		// Crear payload del evento
		payload := map[string]interface{}{
			"event":       eventType,
			"instance_id": instanceID,
			"timestamp":   time.Now().Unix(),
			"data":        eventData,
		}

		// Enviar evento de forma asíncrona
		go ws.sendWebhookEvent(webhook, payload)
	}
}

// extractMessageData - Extrae datos de evento de mensaje
func (ws *WebhookService) extractMessageData(msg *events.Message) map[string]interface{} {
	data := map[string]interface{}{
		"id":        msg.Info.ID,
		"timestamp": msg.Info.Timestamp.Unix(),
		"from":      msg.Info.Sender.String(),
		"chat":      msg.Info.Chat.String(),
		"type":      "text", // Por defecto
	}

	// Extraer contenido según tipo de mensaje
	if msg.Message.Conversation != nil {
		data["type"] = "text"
		data["text"] = *msg.Message.Conversation
	} else if msg.Message.ImageMessage != nil {
		data["type"] = "image"
		data["caption"] = msg.Message.ImageMessage.GetCaption()
		data["mime_type"] = msg.Message.ImageMessage.GetMimetype()
	} else if msg.Message.VideoMessage != nil {
		data["type"] = "video"
		data["caption"] = msg.Message.VideoMessage.GetCaption()
		data["mime_type"] = msg.Message.VideoMessage.GetMimetype()
	} else if msg.Message.AudioMessage != nil {
		data["type"] = "audio"
		data["mime_type"] = msg.Message.AudioMessage.GetMimetype()
		data["duration"] = msg.Message.AudioMessage.GetSeconds()
	} else if msg.Message.DocumentMessage != nil {
		data["type"] = "document"
		data["filename"] = msg.Message.DocumentMessage.GetFileName()
		data["mime_type"] = msg.Message.DocumentMessage.GetMimetype()
	} else if msg.Message.LocationMessage != nil {
		data["type"] = "location"
		data["latitude"] = msg.Message.LocationMessage.GetDegreesLatitude()
		data["longitude"] = msg.Message.LocationMessage.GetDegreesLongitude()
	}

	return data
}

// extractReceiptData - Extrae datos de recibo de mensaje
func (ws *WebhookService) extractReceiptData(receipt *events.Receipt) map[string]interface{} {
	return map[string]interface{}{
		"message_ids": receipt.MessageIDs,
		"timestamp":   receipt.Timestamp.Unix(),
		"chat":        receipt.Chat.String(),
		"sender":      receipt.Sender.String(),
		"type":        receipt.Type.String(),
	}
}

// extractCallOfferData - Extrae datos de oferta de llamada
func (ws *WebhookService) extractCallOfferData(call *events.CallOffer) map[string]interface{} {
	return map[string]interface{}{
		"call_id":   call.CallID,
		"from":      call.CallCreator.String(),
		"timestamp": call.Timestamp.Unix(),
		"is_video":  call.IsVideo,
	}
}

// extractGroupInfoData - Extrae datos de información de grupo
func (ws *WebhookService) extractGroupInfoData(group *events.GroupInfo) map[string]interface{} {
	data := map[string]interface{}{
		"jid":       group.JID.String(),
		"timestamp": group.Timestamp.Unix(),
	}

	if group.Name != nil {
		data["name"] = group.Name.Name
		data["name_changed_by"] = group.Name.NameSetBy.String()
		data["name_changed_at"] = group.Name.NameSetAt.Unix()
	}

	if group.Topic != nil {
		data["topic"] = group.Topic.Topic
		data["topic_changed_by"] = group.Topic.TopicSetBy.String()
		data["topic_changed_at"] = group.Topic.TopicSetAt.Unix()
	}

	return data
}

// extractJoinedGroupData - Extrae datos de unión a grupo
func (ws *WebhookService) extractJoinedGroupData(joined *events.JoinedGroup) map[string]interface{} {
	return map[string]interface{}{
		"group_jid": joined.GroupInfo.JID.String(),
		"type":      joined.Type.String(),
		"timestamp": time.Now().Unix(),
	}
}

// extractContactData - Extrae datos de contacto
func (ws *WebhookService) extractContactData(contact *events.Contact) map[string]interface{} {
	return map[string]interface{}{
		"jid":       contact.JID.String(),
		"name":      contact.Name,
		"notify":    contact.Notify,
		"timestamp": time.Now().Unix(),
	}
}

// extractPushNameData - Extrae datos de push name
func (ws *WebhookService) extractPushNameData(pushName *events.PushName) map[string]interface{} {
	return map[string]interface{}{
		"jid":       pushName.JID.String(),
		"old_name":  pushName.OldPushName,
		"new_name":  pushName.NewPushName,
		"timestamp": time.Now().Unix(),
	}
}

// extractPresenceData - Extrae datos de presencia
func (ws *WebhookService) extractPresenceData(presence *events.Presence) map[string]interface{} {
	return map[string]interface{}{
		"jid":       presence.From.String(),
		"status":    presence.Presence.String(),
		"timestamp": time.Now().Unix(),
	}
}

// isEventConfigured - Verifica si el webhook está configurado para recibir este tipo de evento
func (ws *WebhookService) isEventConfigured(webhook models.WebhookConfig, eventType string) bool {
	var events []string
	if err := json.Unmarshal([]byte(webhook.Events), &events); err != nil {
		return false
	}

	// Verificar si el evento específico está en la lista
	for _, configuredEvent := range events {
		if configuredEvent == eventType || configuredEvent == "*" {
			return true
		}
	}

	return false
}

// sendWebhookEvent - Envía evento a webhook con retry inteligente
func (ws *WebhookService) sendWebhookEvent(webhook models.WebhookConfig, payload map[string]interface{}) {
	eventID := uuid.New().String()
	
	// Crear log inicial
	payloadJSON, _ := json.Marshal(payload)
	log := models.WebhookLog{
		WebhookID:    webhook.WebhookID,
		InstanceID:   webhook.InstanceID,
		EventID:      eventID,
		EventType:    payload["event"].(string),
		Payload:      string(payloadJSON),
		AttemptCount: 0,
	}

	// Intentar envío con retry
	for attempt := 1; attempt <= webhook.MaxRetries; attempt++ {
		log.AttemptCount = attempt
		
		startTime := time.Now()
		success, statusCode, err := ws.doHTTPRequest(webhook, payload)
		responseTime := time.Since(startTime).Milliseconds()

		log.StatusCode = statusCode
		log.ResponseTime = int(responseTime)
		log.IsSuccess = success

		if err != nil {
			log.ErrorMessage = err.Error()
		}

		// Guardar/actualizar log
		if attempt == 1 {
			ws.db.Create(&log)
		} else {
			ws.db.Save(&log)
		}

		// Actualizar métricas
		ws.updateWebhookMetrics(webhook.WebhookID, webhook.InstanceID, success, responseTime)

		// Si fue exitoso, terminar
		if success {
			ws.logger.Info("Webhook enviado exitosamente",
				zap.String("webhookId", webhook.WebhookID),
				zap.String("eventId", eventID),
				zap.Int("attempt", attempt))
			return
		}

		// Si no fue el último intento, esperar antes del retry
		if attempt < webhook.MaxRetries {
			backoffTime := ws.calculateBackoff(attempt)
			time.Sleep(backoffTime)
		}
	}

	// Todos los intentos fallaron
	ws.logger.Error("Webhook falló después de todos los intentos",
		zap.String("webhookId", webhook.WebhookID),
		zap.String("eventId", eventID),
		zap.Int("maxRetries", webhook.MaxRetries))
}

// doHTTPRequest - Realiza la petición HTTP al webhook (implementación simplificada)
func (ws *WebhookService) doHTTPRequest(webhook models.WebhookConfig, payload map[string]interface{}) (bool, int, error) {
	// TODO: Implementar la lógica real de envío HTTP
	// Esta es una implementación mock para que compile
	// En la implementación real aquí iría el código del WebhookController.sendWebhookEvent()
	
	ws.logger.Info("Simulando envío de webhook",
		zap.String("webhookId", webhook.WebhookID),
		zap.String("url", webhook.URL))
	
	// Simular éxito para desarrollo
	return true, 200, nil
}

// calculateBackoff - Calcula tiempo de espera con backoff exponencial
func (ws *WebhookService) calculateBackoff(attempt int) time.Duration {
	// Backoff exponencial: 1s, 2s, 4s, 8s, 16s
	baseDelay := time.Second
	backoffTime := baseDelay * time.Duration(1<<uint(attempt-1))
	
	// Máximo 32 segundos
	if backoffTime > 32*time.Second {
		backoffTime = 32 * time.Second
	}
	
	return backoffTime
}

// updateWebhookMetrics - Actualiza métricas del webhook
func (ws *WebhookService) updateWebhookMetrics(webhookID, instanceID string, success bool, responseTime int64) {
	var metrics models.WebhookMetrics
	ws.db.Where("webhook_id = ? AND instance_id = ?", webhookID, instanceID).FirstOrCreate(&metrics, models.WebhookMetrics{
		WebhookID:  webhookID,
		InstanceID: instanceID,
	})

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
	ws.db.Save(&metrics)
}

// handleAutoRejectCall - Maneja el auto-rechazo de llamadas
func (ws *WebhookService) handleAutoRejectCall(instanceID string, callOffer *events.CallOffer) {
	var config models.CallRejectConfig
	if err := ws.db.Where("instance_id = ?", instanceID).First(&config).Error; err != nil {
		// No hay configuración de auto-rechazo
		return
	}

	if !config.AutoReject {
		// Auto-rechazo deshabilitado
		return
	}

	// Verificar whitelist
	callerJID := callOffer.CallCreator.String()
	if ws.isNumberWhitelisted(config, callerJID) {
		ws.logger.Info("Llamada permitida por whitelist",
			zap.String("instanceId", instanceID),
			zap.String("caller", callerJID))
		return
	}

	// Verificar horarios
	if config.ScheduleEnabled && !ws.isWithinSchedule(config) {
		ws.logger.Info("Llamada rechazada por horario",
			zap.String("instanceId", instanceID),
			zap.String("caller", callerJID))
		// TODO: Integrar con WhatsMeow para rechazar llamada
		return
	}

	// Rechazar llamada
	ws.logger.Info("Auto-rechazando llamada",
		zap.String("instanceId", instanceID),
		zap.String("caller", callerJID))
	
	// TODO: Implementar rechazo real con WhatsMeow
	// client.RejectCall(callOffer.CallID)
}

// isNumberWhitelisted - Verifica si un número está en la whitelist
func (ws *WebhookService) isNumberWhitelisted(config models.CallRejectConfig, callerJID string) bool {
	if config.WhitelistNumbers == "" {
		return false
	}

	var whitelist []string
	if err := json.Unmarshal([]byte(config.WhitelistNumbers), &whitelist); err != nil {
		return false
	}

	for _, number := range whitelist {
		if number == callerJID {
			return true
		}
	}

	return false
}

// isWithinSchedule - Verifica si está dentro del horario permitido
func (ws *WebhookService) isWithinSchedule(config models.CallRejectConfig) bool {
	if config.ScheduleConfig == "" {
		return true
	}

	var schedule map[string]interface{}
	if err := json.Unmarshal([]byte(config.ScheduleConfig), &schedule); err != nil {
		return true
	}

	// TODO: Implementar lógica de verificación de horarios
	// Por ahora devolver true (permitir)
	return true
}
