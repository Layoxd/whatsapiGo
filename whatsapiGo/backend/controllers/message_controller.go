package controllers

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
)

// MessageController - Controlador para manejar envío y recepción de mensajes
type MessageController struct {
	instanceController *InstanceController // Referencia al controlador de instancias
	logger             waLog.Logger        // Logger de WhatsMeow
}

// TextMessageRequest - Estructura para envío de mensajes de texto
type TextMessageRequest struct {
	InstanceID string `json:"instance_id" binding:"required"`
	Phone      string `json:"phone" binding:"required"`      // Número de destino
	IsGroup    bool   `json:"is_group,omitempty"`           // True si es un grupo
	Message    string `json:"message" binding:"required"`    // Contenido del mensaje
	QuotedMsgID string `json:"quoted_msg_id,omitempty"`     // ID del mensaje a citar
}

// MediaMessageRequest - Estructura para envío de mensajes multimedia
type MediaMessageRequest struct {
	InstanceID string `json:"instance_id" binding:"required"`
	Phone      string `json:"phone" binding:"required"`
	IsGroup    bool   `json:"is_group,omitempty"`
	Caption    string `json:"caption,omitempty"`           // Caption para el archivo
	FileName   string `json:"filename,omitempty"`          // Nombre del archivo
	MimeType   string `json:"mimetype,omitempty"`          // Tipo MIME
	MediaData  string `json:"media_data" binding:"required"` // Datos en base64
	QuotedMsgID string `json:"quoted_msg_id,omitempty"`
}

// LocationMessageRequest - Estructura para envío de ubicaciones
type LocationMessageRequest struct {
	InstanceID string  `json:"instance_id" binding:"required"`
	Phone      string  `json:"phone" binding:"required"`
	IsGroup    bool    `json:"is_group,omitempty"`
	Latitude   float64 `json:"latitude" binding:"required"`
	Longitude  float64 `json:"longitude" binding:"required"`
	Name       string  `json:"name,omitempty"`        // Nombre del lugar
	Address    string  `json:"address,omitempty"`     // Dirección
	QuotedMsgID string `json:"quoted_msg_id,omitempty"`
}

// ContactMessageRequest - Estructura para envío de contactos
type ContactMessageRequest struct {
	InstanceID string `json:"instance_id" binding:"required"`
	Phone      string `json:"phone" binding:"required"`
	IsGroup    bool   `json:"is_group,omitempty"`
	ContactVCard string `json:"contact_vcard" binding:"required"` // vCard del contacto
	QuotedMsgID string `json:"quoted_msg_id,omitempty"`
}

// ForwardMessageRequest - Estructura para reenvío de mensajes
type ForwardMessageRequest struct {
	InstanceID    string `json:"instance_id" binding:"required"`
	Phone         string `json:"phone" binding:"required"`
	IsGroup       bool   `json:"is_group,omitempty"`
	MessageID     string `json:"message_id" binding:"required"`     // ID del mensaje a reenviar
	OriginalChat  string `json:"original_chat" binding:"required"`  // Chat original
	ForwardCaption string `json:"forward_caption,omitempty"`        // Caption adicional
}

// MessageHistoryRequest - Estructura para obtener historial
type MessageHistoryRequest struct {
	InstanceID string `json:"instance_id" binding:"required"`
	Phone      string `json:"phone" binding:"required"`
	IsGroup    bool   `json:"is_group,omitempty"`
	Limit      int    `json:"limit,omitempty"`      // Límite de mensajes (default: 50)
	Offset     int    `json:"offset,omitempty"`     // Offset para paginación
	FromTime   string `json:"from_time,omitempty"`  // Fecha desde (ISO 8601)
	ToTime     string `json:"to_time,omitempty"`    // Fecha hasta (ISO 8601)
}

// MessageResponse - Respuesta estándar para operaciones de mensajes
type MessageResponse struct {
	Success   bool                 `json:"success"`
	Message   string               `json:"message"`
	Data      *SentMessageInfo     `json:"data,omitempty"`
	MessageID string               `json:"message_id,omitempty"`
	Timestamp int64                `json:"timestamp,omitempty"`
}

// SentMessageInfo - Información del mensaje enviado
type SentMessageInfo struct {
	ID        string    `json:"id"`
	Phone     string    `json:"phone"`
	IsGroup   bool      `json:"is_group"`
	Type      string    `json:"type"`      // text, image, video, audio, document, location, contact
	Content   string    `json:"content,omitempty"`
	Caption   string    `json:"caption,omitempty"`
	Status    string    `json:"status"`    // sent, delivered, read
	Timestamp time.Time `json:"timestamp"`
}

// MessageHistoryResponse - Respuesta para historial de mensajes
type MessageHistoryResponse struct {
	Success  bool               `json:"success"`
	Message  string             `json:"message"`
	Messages []*SentMessageInfo `json:"messages"`
	Total    int                `json:"total"`
	HasMore  bool               `json:"has_more"`
}

// NewMessageController - Constructor del controlador de mensajes
func NewMessageController(instanceController *InstanceController, logger waLog.Logger) *MessageController {
	return &MessageController{
		instanceController: instanceController,
		logger:             logger,
	}
}

// SendTextMessage - POST /messages/text - Enviar mensaje de texto
func (mc *MessageController) SendTextMessage(c *gin.Context) {
	var req TextMessageRequest
	
	// Validar el JSON de la petición
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, MessageResponse{
			Success: false,
			Message: fmt.Sprintf("Error en los datos enviados: %v", err),
		})
		return
	}

	// Obtener la instancia
	instance, exists := mc.instanceController.instances[req.InstanceID]
	if !exists {
		c.JSON(http.StatusNotFound, MessageResponse{
			Success: false,
			Message: "Instancia no encontrada",
		})
		return
	}

	// Verificar que la instancia esté conectada
	if !instance.Client.IsConnected() {
		c.JSON(http.StatusBadRequest, MessageResponse{
			Success: false,
			Message: "La instancia no está conectada a WhatsApp",
		})
		return
	}

	// Construir JID del destinatario
	recipientJID, err := mc.buildRecipientJID(req.Phone, req.IsGroup)
	if err != nil {
		c.JSON(http.StatusBadRequest, MessageResponse{
			Success: false,
			Message: fmt.Sprintf("Error en el número de teléfono: %v", err),
		})
		return
	}

	// Crear mensaje de texto
	msgContent := &waE2E.Message{
		Conversation: proto.String(req.Message),
	}

	// Si hay un mensaje citado, agregarlo
	if req.QuotedMsgID != "" {
		msgContent.ExtendedTextMessage = &waE2E.ExtendedTextMessage{
			Text: proto.String(req.Message),
			ContextInfo: &waE2E.ContextInfo{
				StanzaId:    proto.String(req.QuotedMsgID),
				Participant: proto.String(req.Phone + "@s.whatsapp.net"),
				QuotedMessage: &waE2E.Message{
					Conversation: proto.String(""),
				},
			},
		}
		msgContent.Conversation = nil
	}

	// Enviar mensaje
	sentMsg, err := instance.Client.SendMessage(context.Background(), recipientJID, msgContent)
	if err != nil {
		mc.logger.Errorf("Error enviando mensaje de texto: %v", err)
		c.JSON(http.StatusInternalServerError, MessageResponse{
			Success: false,
			Message: "Error al enviar el mensaje",
		})
		return
	}

	// Crear información del mensaje enviado
	messageInfo := &SentMessageInfo{
		ID:        sentMsg.ID,
		Phone:     req.Phone,
		IsGroup:   req.IsGroup,
		Type:      "text",
		Content:   req.Message,
		Status:    "sent",
		Timestamp: sentMsg.Timestamp,
	}

	mc.logger.Infof("Mensaje de texto enviado: %s -> %s", req.InstanceID, req.Phone)

	c.JSON(http.StatusOK, MessageResponse{
		Success:   true,
		Message:   "Mensaje de texto enviado exitosamente",
		Data:      messageInfo,
		MessageID: sentMsg.ID,
		Timestamp: sentMsg.Timestamp.Unix(),
	})
}

// SendImageMessage - POST /messages/image - Enviar imagen
func (mc *MessageController) SendImageMessage(c *gin.Context) {
	var req MediaMessageRequest
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, MessageResponse{
			Success: false,
			Message: fmt.Sprintf("Error en los datos enviados: %v", err),
		})
		return
	}

	// Validar que es una imagen
	if !mc.isValidImageType(req.MimeType) {
		c.JSON(http.StatusBadRequest, MessageResponse{
			Success: false,
			Message: "Tipo de archivo no válido. Solo se permiten imágenes (JPEG, PNG, GIF, WebP)",
		})
		return
	}

	sentMsg, err := mc.sendMediaMessage(req, "image")
	if err != nil {
		c.JSON(http.StatusInternalServerError, MessageResponse{
			Success: false,
			Message: fmt.Sprintf("Error al enviar imagen: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, MessageResponse{
		Success:   true,
		Message:   "Imagen enviada exitosamente",
		Data:      sentMsg,
		MessageID: sentMsg.ID,
		Timestamp: time.Now().Unix(),
	})
}

// SendVideoMessage - POST /messages/video - Enviar video
func (mc *MessageController) SendVideoMessage(c *gin.Context) {
	var req MediaMessageRequest
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, MessageResponse{
			Success: false,
			Message: fmt.Sprintf("Error en los datos enviados: %v", err),
		})
		return
	}

	// Validar que es un video
	if !mc.isValidVideoType(req.MimeType) {
		c.JSON(http.StatusBadRequest, MessageResponse{
			Success: false,
			Message: "Tipo de archivo no válido. Solo se permiten videos (MP4, 3GP, MOV, AVI)",
		})
		return
	}

	sentMsg, err := mc.sendMediaMessage(req, "video")
	if err != nil {
		c.JSON(http.StatusInternalServerError, MessageResponse{
			Success: false,
			Message: fmt.Sprintf("Error al enviar video: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, MessageResponse{
		Success:   true,
		Message:   "Video enviado exitosamente",
		Data:      sentMsg,
		MessageID: sentMsg.ID,
		Timestamp: time.Now().Unix(),
	})
}

// SendAudioMessage - POST /messages/audio - Enviar audio/nota de voz
func (mc *MessageController) SendAudioMessage(c *gin.Context) {
	var req MediaMessageRequest
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, MessageResponse{
			Success: false,
			Message: fmt.Sprintf("Error en los datos enviados: %v", err),
		})
		return
	}

	// Validar que es un audio
	if !mc.isValidAudioType(req.MimeType) {
		c.JSON(http.StatusBadRequest, MessageResponse{
			Success: false,
			Message: "Tipo de archivo no válido. Solo se permiten audios (MP3, AAC, OGG, OPUS, M4A)",
		})
		return
	}

	sentMsg, err := mc.sendMediaMessage(req, "audio")
	if err != nil {
		c.JSON(http.StatusInternalServerError, MessageResponse{
			Success: false,
			Message: fmt.Sprintf("Error al enviar audio: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, MessageResponse{
		Success:   true,
		Message:   "Audio enviado exitosamente",
		Data:      sentMsg,
		MessageID: sentMsg.ID,
		Timestamp: time.Now().Unix(),
	})
}

// SendDocumentMessage - POST /messages/document - Enviar documento
func (mc *MessageController) SendDocumentMessage(c *gin.Context) {
	var req MediaMessageRequest
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, MessageResponse{
			Success: false,
			Message: fmt.Sprintf("Error en los datos enviados: %v", err),
		})
		return
	}

	// Validar nombre de archivo
	if req.FileName == "" {
		c.JSON(http.StatusBadRequest, MessageResponse{
			Success: false,
			Message: "El nombre del archivo es requerido para documentos",
		})
		return
	}

	sentMsg, err := mc.sendMediaMessage(req, "document")
	if err != nil {
		c.JSON(http.StatusInternalServerError, MessageResponse{
			Success: false,
			Message: fmt.Sprintf("Error al enviar documento: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, MessageResponse{
		Success:   true,
		Message:   "Documento enviado exitosamente",
		Data:      sentMsg,
		MessageID: sentMsg.ID,
		Timestamp: time.Now().Unix(),
	})
}

// SendLocationMessage - POST /messages/location - Enviar ubicación
func (mc *MessageController) SendLocationMessage(c *gin.Context) {
	var req LocationMessageRequest
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, MessageResponse{
			Success: false,
			Message: fmt.Sprintf("Error en los datos enviados: %v", err),
		})
		return
	}

	// Obtener la instancia
	instance, exists := mc.instanceController.instances[req.InstanceID]
	if !exists {
		c.JSON(http.StatusNotFound, MessageResponse{
			Success: false,
			Message: "Instancia no encontrada",
		})
		return
	}

	if !instance.Client.IsConnected() {
		c.JSON(http.StatusBadRequest, MessageResponse{
			Success: false,
			Message: "La instancia no está conectada a WhatsApp",
		})
		return
	}

	// Construir JID del destinatario
	recipientJID, err := mc.buildRecipientJID(req.Phone, req.IsGroup)
	if err != nil {
		c.JSON(http.StatusBadRequest, MessageResponse{
			Success: false,
			Message: fmt.Sprintf("Error en el número de teléfono: %v", err),
		})
		return
	}

	// Crear mensaje de ubicación
	msgContent := &waE2E.Message{
		LocationMessage: &waE2E.LocationMessage{
			DegreesLatitude:  proto.Float64(req.Latitude),
			DegreesLongitude: proto.Float64(req.Longitude),
			Name:             proto.String(req.Name),
			Address:          proto.String(req.Address),
		},
	}

	// Enviar mensaje
	sentMsg, err := instance.Client.SendMessage(context.Background(), recipientJID, msgContent)
	if err != nil {
		mc.logger.Errorf("Error enviando ubicación: %v", err)
		c.JSON(http.StatusInternalServerError, MessageResponse{
			Success: false,
			Message: "Error al enviar la ubicación",
		})
		return
	}

	messageInfo := &SentMessageInfo{
		ID:        sentMsg.ID,
		Phone:     req.Phone,
		IsGroup:   req.IsGroup,
		Type:      "location",
		Content:   fmt.Sprintf("Lat: %f, Lng: %f", req.Latitude, req.Longitude),
		Status:    "sent",
		Timestamp: sentMsg.Timestamp,
	}

	mc.logger.Infof("Ubicación enviada: %s -> %s", req.InstanceID, req.Phone)

	c.JSON(http.StatusOK, MessageResponse{
		Success:   true,
		Message:   "Ubicación enviada exitosamente",
		Data:      messageInfo,
		MessageID: sentMsg.ID,
		Timestamp: sentMsg.Timestamp.Unix(),
	})
}

// SendContactMessage - POST /messages/contact - Enviar contacto
func (mc *MessageController) SendContactMessage(c *gin.Context) {
	var req ContactMessageRequest
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, MessageResponse{
			Success: false,
			Message: fmt.Sprintf("Error en los datos enviados: %v", err),
		})
		return
	}

	// Obtener la instancia
	instance, exists := mc.instanceController.instances[req.InstanceID]
	if !exists {
		c.JSON(http.StatusNotFound, MessageResponse{
			Success: false,
			Message: "Instancia no encontrada",
		})
		return
	}

	if !instance.Client.IsConnected() {
		c.JSON(http.StatusBadRequest, MessageResponse{
			Success: false,
			Message: "La instancia no está conectada a WhatsApp",
		})
		return
	}

	// Construir JID del destinatario
	recipientJID, err := mc.buildRecipientJID(req.Phone, req.IsGroup)
	if err != nil {
		c.JSON(http.StatusBadRequest, MessageResponse{
			Success: false,
			Message: fmt.Sprintf("Error en el número de teléfono: %v", err),
		})
		return
	}

	// Crear mensaje de contacto
	msgContent := &waE2E.Message{
		ContactMessage: &waE2E.ContactMessage{
			DisplayName: proto.String("Contacto Compartido"),
			Vcard:       proto.String(req.ContactVCard),
		},
	}

	// Enviar mensaje
	sentMsg, err := instance.Client.SendMessage(context.Background(), recipientJID, msgContent)
	if err != nil {
		mc.logger.Errorf("Error enviando contacto: %v", err)
		c.JSON(http.StatusInternalServerError, MessageResponse{
			Success: false,
			Message: "Error al enviar el contacto",
		})
		return
	}

	messageInfo := &SentMessageInfo{
		ID:        sentMsg.ID,
		Phone:     req.Phone,
		IsGroup:   req.IsGroup,
		Type:      "contact",
		Content:   "Contacto compartido",
		Status:    "sent",
		Timestamp: sentMsg.Timestamp,
	}

	mc.logger.Infof("Contacto enviado: %s -> %s", req.InstanceID, req.Phone)

	c.JSON(http.StatusOK, MessageResponse{
		Success:   true,
		Message:   "Contacto enviado exitosamente",
		Data:      messageInfo,
		MessageID: sentMsg.ID,
		Timestamp: sentMsg.Timestamp.Unix(),
	})
}

// sendMediaMessage - Función auxiliar para enviar mensajes multimedia
func (mc *MessageController) sendMediaMessage(req MediaMessageRequest, mediaType string) (*SentMessageInfo, error) {
	// Obtener la instancia
	instance, exists := mc.instanceController.instances[req.InstanceID]
	if !exists {
		return nil, fmt.Errorf("instancia no encontrada")
	}

	if !instance.Client.IsConnected() {
		return nil, fmt.Errorf("la instancia no está conectada a WhatsApp")
	}

	// Construir JID del destinatario
	recipientJID, err := mc.buildRecipientJID(req.Phone, req.IsGroup)
	if err != nil {
		return nil, fmt.Errorf("error en el número de teléfono: %v", err)
	}

	// Decodificar datos base64
	mediaData, err := base64.StdEncoding.DecodeString(req.MediaData)
	if err != nil {
		return nil, fmt.Errorf("error decodificando datos base64: %v", err)
	}

	// Detectar tipo MIME si no se proporcionó
	if req.MimeType == "" {
		req.MimeType = http.DetectContentType(mediaData)
	}

	// Subir archivo a WhatsApp
	uploaded, err := instance.Client.Upload(context.Background(), mediaData, whatsmeow.MediaType(mediaType))
	if err != nil {
		return nil, fmt.Errorf("error subiendo archivo: %v", err)
	}

	// Crear mensaje según el tipo
	var msgContent *waE2E.Message
	switch mediaType {
	case "image":
		msgContent = &waE2E.Message{
			ImageMessage: &waE2E.ImageMessage{
				Caption:       proto.String(req.Caption),
				Url:           proto.String(uploaded.URL),
				DirectPath:    proto.String(uploaded.DirectPath),
				MediaKey:      uploaded.MediaKey,
				Mimetype:      proto.String(req.MimeType),
				FileEncSha256: uploaded.FileEncSHA256,
				FileSha256:    uploaded.FileSHA256,
				FileLength:    proto.Uint64(uint64(len(mediaData))),
			},
		}
	case "video":
		msgContent = &waE2E.Message{
			VideoMessage: &waE2E.VideoMessage{
				Caption:       proto.String(req.Caption),
				Url:           proto.String(uploaded.URL),
				DirectPath:    proto.String(uploaded.DirectPath),
				MediaKey:      uploaded.MediaKey,
				Mimetype:      proto.String(req.MimeType),
				FileEncSha256: uploaded.FileEncSHA256,
				FileSha256:    uploaded.FileSHA256,
				FileLength:    proto.Uint64(uint64(len(mediaData))),
			},
		}
	case "audio":
		msgContent = &waE2E.Message{
			AudioMessage: &waE2E.AudioMessage{
				Url:           proto.String(uploaded.URL),
				DirectPath:    proto.String(uploaded.DirectPath),
				MediaKey:      uploaded.MediaKey,
				Mimetype:      proto.String(req.MimeType),
				FileEncSha256: uploaded.FileEncSHA256,
				FileSha256:    uploaded.FileSHA256,
				FileLength:    proto.Uint64(uint64(len(mediaData))),
				Ptt:           proto.Bool(strings.Contains(req.MimeType, "ogg")), // Nota de voz si es OGG
			},
		}
	case "document":
		msgContent = &waE2E.Message{
			DocumentMessage: &waE2E.DocumentMessage{
				Caption:       proto.String(req.Caption),
				Url:           proto.String(uploaded.URL),
				DirectPath:    proto.String(uploaded.DirectPath),
				MediaKey:      uploaded.MediaKey,
				Mimetype:      proto.String(req.MimeType),
				FileEncSha256: uploaded.FileEncSHA256,
				FileSha256:    uploaded.FileSHA256,
				FileLength:    proto.Uint64(uint64(len(mediaData))),
				FileName:      proto.String(req.FileName),
			},
		}
	}

	// Enviar mensaje
	sentMsg, err := instance.Client.SendMessage(context.Background(), recipientJID, msgContent)
	if err != nil {
		return nil, fmt.Errorf("error enviando mensaje: %v", err)
	}

	messageInfo := &SentMessageInfo{
		ID:        sentMsg.ID,
		Phone:     req.Phone,
		IsGroup:   req.IsGroup,
		Type:      mediaType,
		Caption:   req.Caption,
		Status:    "sent",
		Timestamp: sentMsg.Timestamp,
	}

	mc.logger.Infof("Mensaje %s enviado: %s -> %s", mediaType, req.InstanceID, req.Phone)
	return messageInfo, nil
}

// buildRecipientJID - Construir JID del destinatario
func (mc *MessageController) buildRecipientJID(phone string, isGroup bool) (types.JID, error) {
	// Limpiar número de teléfono
	phone = strings.ReplaceAll(phone, "+", "")
	phone = strings.ReplaceAll(phone, " ", "")
	phone = strings.ReplaceAll(phone, "-", "")

	if isGroup {
		// Para grupos, el formato es diferente
		if !strings.Contains(phone, "@") {
			phone = phone + "@g.us"
		}
	} else {
		// Para contactos individuales
		if !strings.Contains(phone, "@") {
			phone = phone + "@s.whatsapp.net"
		}
	}

	jid, err := types.ParseJID(phone)
	if err != nil {
		return jid, fmt.Errorf("formato de número inválido: %v", err)
	}

	return jid, nil
}

// Funciones de validación de tipos de archivo
func (mc *MessageController) isValidImageType(mimeType string) bool {
	validTypes := []string{
		"image/jpeg", "image/jpg", "image/png", 
		"image/gif", "image/webp", "image/bmp",
	}
	for _, validType := range validTypes {
		if mimeType == validType {
			return true
		}
	}
	return false
}

func (mc *MessageController) isValidVideoType(mimeType string) bool {
	validTypes := []string{
		"video/mp4", "video/3gpp", "video/quicktime", 
		"video/avi", "video/mkv", "video/webm",
	}
	for _, validType := range validTypes {
		if mimeType == validType {
			return true
		}
	}
	return false
}

func (mc *MessageController) isValidAudioType(mimeType string) bool {
	validTypes := []string{
		"audio/mpeg", "audio/mp3", "audio/aac", 
		"audio/ogg", "audio/opus", "audio/m4a", 
		"audio/wav", "audio/amr",
	}
	for _, validType := range validTypes {
		if mimeType == validType {
			return true
		}
	}
	return false
}// Archivo base: message_controller.go
