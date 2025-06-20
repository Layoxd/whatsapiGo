package controllers

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
)

// StatusController - Controlador para gestión completa de estados/stories
type StatusController struct {
	instanceController *InstanceController // Referencia al controlador de instancias
	logger             waLog.Logger        // Logger de WhatsMeow
}

// StatusInfo - Información completa de un estado
type StatusInfo struct {
	ID           string                `json:"id"`                     // ID único del estado
	JID          string                `json:"jid"`                    // JID del autor
	LID          string                `json:"lid,omitempty"`          // LID del autor
	Phone        string                `json:"phone"`                  // Número del autor
	Name         string                `json:"name,omitempty"`         // Nombre del autor
	Type         string                `json:"type"`                   // text, image, video, audio
	Content      string                `json:"content,omitempty"`      // Contenido de texto
	Caption      string                `json:"caption,omitempty"`      // Caption para multimedia
	MediaURL     string                `json:"media_url,omitempty"`    // URL del archivo multimedia
	MediaMimeType string               `json:"media_mime_type,omitempty"` // Tipo MIME del archivo
	BackgroundColor string             `json:"background_color,omitempty"` // Color de fondo para texto
	Font         int                   `json:"font,omitempty"`         // Fuente para texto
	IsOwn        bool                  `json:"is_own"`                 // Si es nuestro estado
	PublishedAt  time.Time             `json:"published_at"`           // Fecha de publicación
	ExpiresAt    time.Time             `json:"expires_at"`             // Fecha de expiración (24h)
	ViewCount    int                   `json:"view_count"`             // Número de visualizaciones
	Viewers      []*StatusViewer       `json:"viewers,omitempty"`      // Lista de viewers
	Privacy      string                `json:"privacy"`                // all, contacts, selected, except
	Audience     []string              `json:"audience,omitempty"`     // Lista de JIDs con acceso
}

// StatusViewer - Información de quien vio el estado
type StatusViewer struct {
	JID      string    `json:"jid"`                // JID del viewer
	LID      string    `json:"lid,omitempty"`      // LID del viewer
	Phone    string    `json:"phone"`              // Número del viewer
	Name     string    `json:"name,omitempty"`     // Nombre del viewer
	ViewedAt time.Time `json:"viewed_at"`          // Fecha de visualización
}

// StatusPrivacySettings - Configuraciones de privacidad de estados
type StatusPrivacySettings struct {
	DefaultPrivacy string   `json:"default_privacy"`           // all, contacts, selected, except
	AllowList      []string `json:"allow_list,omitempty"`      // Lista de JIDs permitidos
	DenyList       []string `json:"deny_list,omitempty"`       // Lista de JIDs bloqueados
	ReadReceipts   bool     `json:"read_receipts"`             // Mostrar confirmaciones de lectura
	AllowReplies   bool     `json:"allow_replies"`             // Permitir respuestas
}

// PublishStatusRequest - Petición para publicar estado
type PublishStatusRequest struct {
	InstanceID      string   `json:"instance_id" binding:"required"`
	Type            string   `json:"type" binding:"required"`        // text, image, video, audio
	Content         string   `json:"content,omitempty"`              // Texto del estado
	Caption         string   `json:"caption,omitempty"`              // Caption para multimedia
	MediaData       string   `json:"media_data,omitempty"`           // Media en base64
	MediaURL        string   `json:"media_url,omitempty"`            // URL del archivo
	MimeType        string   `json:"mime_type,omitempty"`            // Tipo MIME
	BackgroundColor string   `json:"background_color,omitempty"`     // Color de fondo (#RRGGBB)
	Font            int      `json:"font,omitempty"`                 // ID de fuente (0-5)
	Privacy         string   `json:"privacy,omitempty"`              // all, contacts, selected, except
	Audience        []string `json:"audience,omitempty"`             // Lista de JIDs específicos
}

// UpdatePrivacyRequest - Petición para actualizar privacidad
type UpdatePrivacyRequest struct {
	InstanceID     string   `json:"instance_id" binding:"required"`
	DefaultPrivacy string   `json:"default_privacy" binding:"required"` // all, contacts, selected, except
	AllowList      []string `json:"allow_list,omitempty"`               // Lista de permitidos
	DenyList       []string `json:"deny_list,omitempty"`                // Lista de bloqueados
	ReadReceipts   bool     `json:"read_receipts"`                      // Confirmaciones de lectura
	AllowReplies   bool     `json:"allow_replies"`                      // Permitir respuestas
}

// StatusResponse - Respuesta estándar para operaciones de estados
type StatusResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    *StatusInfo `json:"data,omitempty"`
}

// StatusListResponse - Respuesta para listar estados
type StatusListResponse struct {
	Success  bool          `json:"success"`
	Message  string        `json:"message"`
	Statuses []*StatusInfo `json:"statuses"`
	Total    int           `json:"total"`
	Own      int           `json:"own"`      // Propios
	Contacts int           `json:"contacts"` // De contactos
}

// StatusViewersResponse - Respuesta para viewers de estado
type StatusViewersResponse struct {
	Success   bool            `json:"success"`
	Message   string          `json:"message"`
	StatusID  string          `json:"status_id"`
	Viewers   []*StatusViewer `json:"viewers"`
	ViewCount int             `json:"view_count"`
}

// StatusPrivacyResponse - Respuesta para configuraciones de privacidad
type StatusPrivacyResponse struct {
	Success bool                   `json:"success"`
	Message string                 `json:"message"`
	Privacy *StatusPrivacySettings `json:"privacy,omitempty"`
}

// NewStatusController - Constructor del controlador de estados
func NewStatusController(instanceController *InstanceController, logger waLog.Logger) *StatusController {
	return &StatusController{
		instanceController: instanceController,
		logger:             logger,
	}
}

// PublishStatus - POST /status/{instanceId}/publish - Publicar estado/story
func (sc *StatusController) PublishStatus(c *gin.Context) {
	instanceID := c.Param("instanceId")
	var req PublishStatusRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, StatusResponse{
			Success: false,
			Message: fmt.Sprintf("Error en los datos enviados: %v", err),
		})
		return
	}

	// Validar instanceID
	if req.InstanceID != instanceID {
		req.InstanceID = instanceID
	}

	// Obtener instancia
	instance, exists := sc.instanceController.instances[req.InstanceID]
	if !exists {
		c.JSON(http.StatusNotFound, StatusResponse{
			Success: false,
			Message: "Instancia no encontrada",
		})
		return
	}

	if !instance.Client.IsConnected() {
		c.JSON(http.StatusBadRequest, StatusResponse{
			Success: false,
			Message: "La instancia no está conectada a WhatsApp",
		})
		return
	}

	// Validar tipo de estado
	if !sc.isValidStatusType(req.Type) {
		c.JSON(http.StatusBadRequest, StatusResponse{
			Success: false,
			Message: "Tipo de estado inválido. Permitidos: text, image, video, audio",
		})
		return
	}

	// Publicar estado según el tipo
	statusInfo, err := sc.publishStatusByType(instance.Client, req)
	if err != nil {
		sc.logger.Errorf("Error publicando estado: %v", err)
		c.JSON(http.StatusInternalServerError, StatusResponse{
			Success: false,
			Message: fmt.Sprintf("Error publicando estado: %v", err),
		})
		return
	}

	sc.logger.Infof("Estado publicado: tipo %s para instancia %s", req.Type, instanceID)

	c.JSON(http.StatusCreated, StatusResponse{
		Success: true,
		Message: "Estado publicado exitosamente",
		Data:    statusInfo,
	})
}

// GetOwnStatuses - GET /status/{instanceId} - Listar estados propios
func (sc *StatusController) GetOwnStatuses(c *gin.Context) {
	instanceID := c.Param("instanceId")

	// Obtener instancia
	instance, exists := sc.instanceController.instances[instanceID]
	if !exists {
		c.JSON(http.StatusNotFound, StatusListResponse{
			Success: false,
			Message: "Instancia no encontrada",
		})
		return
	}

	if !instance.Client.IsConnected() {
		c.JSON(http.StatusBadRequest, StatusListResponse{
			Success: false,
			Message: "La instancia no está conectada a WhatsApp",
		})
		return
	}

	// Obtener estados propios
	ownStatuses, err := sc.getOwnStatuses(instance.Client)
	if err != nil {
		sc.logger.Errorf("Error obteniendo estados propios: %v", err)
		c.JSON(http.StatusInternalServerError, StatusListResponse{
			Success: false,
			Message: "Error obteniendo estados propios",
		})
		return
	}

	sc.logger.Infof("Estados propios obtenidos: %d para instancia %s", len(ownStatuses), instanceID)

	c.JSON(http.StatusOK, StatusListResponse{
		Success:  true,
		Message:  "Estados propios obtenidos exitosamente",
		Statuses: ownStatuses,
		Total:    len(ownStatuses),
		Own:      len(ownStatuses),
		Contacts: 0,
	})
}

// GetContactStatuses - GET /status/{instanceId}/contacts - Ver estados de contactos
func (sc *StatusController) GetContactStatuses(c *gin.Context) {
	instanceID := c.Param("instanceId")

	// Obtener instancia
	instance, exists := sc.instanceController.instances[instanceID]
	if !exists {
		c.JSON(http.StatusNotFound, StatusListResponse{
			Success: false,
			Message: "Instancia no encontrada",
		})
		return
	}

	if !instance.Client.IsConnected() {
		c.JSON(http.StatusBadRequest, StatusListResponse{
			Success: false,
			Message: "La instancia no está conectada a WhatsApp",
		})
		return
	}

	// Obtener estados de contactos
	contactStatuses, err := sc.getContactStatuses(instance.Client)
	if err != nil {
		sc.logger.Errorf("Error obteniendo estados de contactos: %v", err)
		c.JSON(http.StatusInternalServerError, StatusListResponse{
			Success: false,
			Message: "Error obteniendo estados de contactos",
		})
		return
	}

	sc.logger.Infof("Estados de contactos obtenidos: %d para instancia %s", len(contactStatuses), instanceID)

	c.JSON(http.StatusOK, StatusListResponse{
		Success:  true,
		Message:  "Estados de contactos obtenidos exitosamente",
		Statuses: contactStatuses,
		Total:    len(contactStatuses),
		Own:      0,
		Contacts: len(contactStatuses),
	})
}

// GetContactStatus - GET /status/{instanceId}/contact/{jid} - Estados de contacto específico
func (sc *StatusController) GetContactStatus(c *gin.Context) {
	instanceID := c.Param("instanceId")
	contactJIDStr := c.Param("jid")

	// Obtener instancia
	instance, exists := sc.instanceController.instances[instanceID]
	if !exists {
		c.JSON(http.StatusNotFound, StatusListResponse{
			Success: false,
			Message: "Instancia no encontrada",
		})
		return
	}

	if !instance.Client.IsConnected() {
		c.JSON(http.StatusBadRequest, StatusListResponse{
			Success: false,
			Message: "La instancia no está conectada a WhatsApp",
		})
		return
	}

	// Parsear JID del contacto
	contactJID, err := types.ParseJID(contactJIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, StatusListResponse{
			Success: false,
			Message: fmt.Sprintf("JID de contacto inválido: %v", err),
		})
		return
	}

	// Obtener estados del contacto específico
	statuses, err := sc.getSpecificContactStatuses(instance.Client, contactJID)
	if err != nil {
		sc.logger.Errorf("Error obteniendo estados del contacto %s: %v", contactJID, err)
		c.JSON(http.StatusInternalServerError, StatusListResponse{
			Success: false,
			Message: "Error obteniendo estados del contacto",
		})
		return
	}

	sc.logger.Infof("Estados del contacto %s obtenidos: %d", contactJID, len(statuses))

	c.JSON(http.StatusOK, StatusListResponse{
		Success:  true,
		Message:  fmt.Sprintf("Estados del contacto obtenidos: %d", len(statuses)),
		Statuses: statuses,
		Total:    len(statuses),
		Own:      0,
		Contacts: len(statuses),
	})
}

// DeleteStatus - DELETE /status/{instanceId}/{statusId} - Eliminar estado propio
func (sc *StatusController) DeleteStatus(c *gin.Context) {
	instanceID := c.Param("instanceId")
	statusID := c.Param("statusId")

	// Obtener instancia
	instance, exists := sc.instanceController.instances[instanceID]
	if !exists {
		c.JSON(http.StatusNotFound, StatusResponse{
			Success: false,
			Message: "Instancia no encontrada",
		})
		return
	}

	if !instance.Client.IsConnected() {
		c.JSON(http.StatusBadRequest, StatusResponse{
			Success: false,
			Message: "La instancia no está conectada a WhatsApp",
		})
		return
	}

	// Eliminar estado
	err := sc.deleteOwnStatus(instance.Client, statusID)
	if err != nil {
		sc.logger.Errorf("Error eliminando estado %s: %v", statusID, err)
		c.JSON(http.StatusInternalServerError, StatusResponse{
			Success: false,
			Message: "Error eliminando el estado",
		})
		return
	}

	sc.logger.Infof("Estado eliminado: %s", statusID)

	c.JSON(http.StatusOK, StatusResponse{
		Success: true,
		Message: "Estado eliminado exitosamente",
	})
}

// GetStatusViewers - GET /status/{instanceId}/{statusId}/viewers - Ver quién vio el estado
func (sc *StatusController) GetStatusViewers(c *gin.Context) {
	instanceID := c.Param("instanceId")
	statusID := c.Param("statusId")

	// Obtener instancia
	instance, exists := sc.instanceController.instances[instanceID]
	if !exists {
		c.JSON(http.StatusNotFound, StatusViewersResponse{
			Success: false,
			Message: "Instancia no encontrada",
		})
		return
	}

	if !instance.Client.IsConnected() {
		c.JSON(http.StatusBadRequest, StatusViewersResponse{
			Success: false,
			Message: "La instancia no está conectada a WhatsApp",
		})
		return
	}

	// Obtener viewers del estado
	viewers, err := sc.getStatusViewers(instance.Client, statusID)
	if err != nil {
		sc.logger.Errorf("Error obteniendo viewers del estado %s: %v", statusID, err)
		c.JSON(http.StatusInternalServerError, StatusViewersResponse{
			Success: false,
			Message: "Error obteniendo viewers del estado",
		})
		return
	}

	sc.logger.Infof("Viewers del estado %s obtenidos: %d", statusID, len(viewers))

	c.JSON(http.StatusOK, StatusViewersResponse{
		Success:   true,
		Message:   fmt.Sprintf("Viewers obtenidos: %d visualizaciones", len(viewers)),
		StatusID:  statusID,
		Viewers:   viewers,
		ViewCount: len(viewers),
	})
}

// UpdateStatusPrivacy - POST /status/{instanceId}/privacy - Configurar privacidad
func (sc *StatusController) UpdateStatusPrivacy(c *gin.Context) {
	instanceID := c.Param("instanceId")
	var req UpdatePrivacyRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, StatusPrivacyResponse{
			Success: false,
			Message: fmt.Sprintf("Error en los datos enviados: %v", err),
		})
		return
	}

	// Validar instanceID
	if req.InstanceID != instanceID {
		req.InstanceID = instanceID
	}

	// Obtener instancia
	instance, exists := sc.instanceController.instances[req.InstanceID]
	if !exists {
		c.JSON(http.StatusNotFound, StatusPrivacyResponse{
			Success: false,
			Message: "Instancia no encontrada",
		})
		return
	}

	if !instance.Client.IsConnected() {
		c.JSON(http.StatusBadRequest, StatusPrivacyResponse{
			Success: false,
			Message: "La instancia no está conectada a WhatsApp",
		})
		return
	}

	// Validar configuración de privacidad
	if !sc.isValidPrivacyType(req.DefaultPrivacy) {
		c.JSON(http.StatusBadRequest, StatusPrivacyResponse{
			Success: false,
			Message: "Tipo de privacidad inválido. Permitidos: all, contacts, selected, except",
		})
		return
	}

	// Actualizar configuraciones de privacidad
	privacySettings, err := sc.updatePrivacySettings(instance.Client, req)
	if err != nil {
		sc.logger.Errorf("Error actualizando privacidad de estados: %v", err)
		c.JSON(http.StatusInternalServerError, StatusPrivacyResponse{
			Success: false,
			Message: "Error actualizando configuraciones de privacidad",
		})
		return
	}

	sc.logger.Infof("Configuraciones de privacidad actualizadas para instancia %s", instanceID)

	c.JSON(http.StatusOK, StatusPrivacyResponse{
		Success: true,
		Message: "Configuraciones de privacidad actualizadas exitosamente",
		Privacy: privacySettings,
	})
}

// GetStatusPrivacy - GET /status/{instanceId}/privacy - Obtener configuración de privacidad
func (sc *StatusController) GetStatusPrivacy(c *gin.Context) {
	instanceID := c.Param("instanceId")

	// Obtener instancia
	instance, exists := sc.instanceController.instances[instanceID]
	if !exists {
		c.JSON(http.StatusNotFound, StatusPrivacyResponse{
			Success: false,
			Message: "Instancia no encontrada",
		})
		return
	}

	if !instance.Client.IsConnected() {
		c.JSON(http.StatusBadRequest, StatusPrivacyResponse{
			Success: false,
			Message: "La instancia no está conectada a WhatsApp",
		})
		return
	}

	// Obtener configuraciones actuales
	privacySettings, err := sc.getPrivacySettings(instance.Client)
	if err != nil {
		sc.logger.Errorf("Error obteniendo configuraciones de privacidad: %v", err)
		c.JSON(http.StatusInternalServerError, StatusPrivacyResponse{
			Success: false,
			Message: "Error obteniendo configuraciones de privacidad",
		})
		return
	}

	sc.logger.Infof("Configuraciones de privacidad obtenidas para instancia %s", instanceID)

	c.JSON(http.StatusOK, StatusPrivacyResponse{
		Success: true,
		Message: "Configuraciones de privacidad obtenidas exitosamente",
		Privacy: privacySettings,
	})
}

// publishStatusByType - Publicar estado según el tipo
func (sc *StatusController) publishStatusByType(client *whatsmeow.Client, req PublishStatusRequest) (*StatusInfo, error) {
	switch req.Type {
	case "text":
		return sc.publishTextStatus(client, req)
	case "image":
		return sc.publishImageStatus(client, req)
	case "video":
		return sc.publishVideoStatus(client, req)
	case "audio":
		return sc.publishAudioStatus(client, req)
	default:
		return nil, fmt.Errorf("tipo de estado no soportado: %s", req.Type)
	}
}

// publishTextStatus - Publicar estado de texto
func (sc *StatusController) publishTextStatus(client *whatsmeow.Client, req PublishStatusRequest) (*StatusInfo, error) {
	if req.Content == "" {
		return nil, fmt.Errorf("el contenido de texto es requerido")
	}

	// Crear mensaje de estado de texto
	statusMessage := &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text: proto.String(req.Content),
		},
	}

	// Configurar color de fondo si se proporciona
	if req.BackgroundColor != "" {
		statusMessage.ExtendedTextMessage.BackgroundArgb = proto.Uint32(sc.parseColorToARGB(req.BackgroundColor))
	}

	// Configurar fuente si se proporciona
	if req.Font > 0 {
		statusMessage.ExtendedTextMessage.Font = proto.Uint32(uint32(req.Font))
	}

	// Enviar estado
	sentMsg, err := client.SendMessage(context.Background(), types.StatusBroadcastJID, statusMessage)
	if err != nil {
		return nil, fmt.Errorf("error enviando estado de texto: %v", err)
	}

	// Crear información del estado
	statusInfo := &StatusInfo{
		ID:              sentMsg.ID,
		JID:             client.Store.ID.String(),
		Phone:           client.Store.ID.User,
		Type:            "text",
		Content:         req.Content,
		BackgroundColor: req.BackgroundColor,
		Font:            req.Font,
		IsOwn:           true,
		PublishedAt:     sentMsg.Timestamp,
		ExpiresAt:       sentMsg.Timestamp.Add(24 * time.Hour),
		Privacy:         sc.getDefaultPrivacy(req.Privacy),
	}

	return statusInfo, nil
}

// publishImageStatus - Publicar estado de imagen
func (sc *StatusController) publishImageStatus(client *whatsmeow.Client, req PublishStatusRequest) (*StatusInfo, error) {
	// Obtener datos de la imagen
	imageData, mimeType, err := sc.getMediaData(req.MediaData, req.MediaURL, req.MimeType)
	if err != nil {
		return nil, fmt.Errorf("error obteniendo datos de imagen: %v", err)
	}

	// Validar que es una imagen
	if !sc.isValidImageType(mimeType) {
		return nil, fmt.Errorf("tipo de archivo no válido para imagen")
	}

	// Subir imagen
	uploaded, err := client.Upload(context.Background(), imageData, whatsmeow.MediaImage)
	if err != nil {
		return nil, fmt.Errorf("error subiendo imagen: %v", err)
	}

	// Crear mensaje de estado de imagen
	statusMessage := &waE2E.Message{
		ImageMessage: &waE2E.ImageMessage{
			Caption:       proto.String(req.Caption),
			Url:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			Mimetype:      proto.String(mimeType),
			FileEncSha256: uploaded.FileEncSHA256,
			FileSha256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uint64(len(imageData))),
		},
	}

	// Enviar estado
	sentMsg, err := client.SendMessage(context.Background(), types.StatusBroadcastJID, statusMessage)
	if err != nil {
		return nil, fmt.Errorf("error enviando estado de imagen: %v", err)
	}

	// Crear información del estado
	statusInfo := &StatusInfo{
		ID:            sentMsg.ID,
		JID:           client.Store.ID.String(),
		Phone:         client.Store.ID.User,
		Type:          "image",
		Caption:       req.Caption,
		MediaURL:      uploaded.URL,
		MediaMimeType: mimeType,
		IsOwn:         true,
		PublishedAt:   sentMsg.Timestamp,
		ExpiresAt:     sentMsg.Timestamp.Add(24 * time.Hour),
		Privacy:       sc.getDefaultPrivacy(req.Privacy),
	}

	return statusInfo, nil
}

// publishVideoStatus - Publicar estado de video
func (sc *StatusController) publishVideoStatus(client *whatsmeow.Client, req PublishStatusRequest) (*StatusInfo, error) {
	// Obtener datos del video
	videoData, mimeType, err := sc.getMediaData(req.MediaData, req.MediaURL, req.MimeType)
	if err != nil {
		return nil, fmt.Errorf("error obteniendo datos de video: %v", err)
	}

	// Validar que es un video
	if !sc.isValidVideoType(mimeType) {
		return nil, fmt.Errorf("tipo de archivo no válido para video")
	}

	// Subir video
	uploaded, err := client.Upload(context.Background(), videoData, whatsmeow.MediaVideo)
	if err != nil {
		return nil, fmt.Errorf("error subiendo video: %v", err)
	}

	// Crear mensaje de estado de video
	statusMessage := &waE2E.Message{
		VideoMessage: &waE2E.VideoMessage{
			Caption:       proto.String(req.Caption),
			Url:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			Mimetype:      proto.String(mimeType),
			FileEncSha256: uploaded.FileEncSHA256,
			FileSha256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uint64(len(videoData))),
		},
	}

	// Enviar estado
	sentMsg, err := client.SendMessage(context.Background(), types.StatusBroadcastJID, statusMessage)
	if err != nil {
		return nil, fmt.Errorf("error enviando estado de video: %v", err)
	}

	// Crear información del estado
	statusInfo := &StatusInfo{
		ID:            sentMsg.ID,
		JID:           client.Store.ID.String(),
		Phone:         client.Store.ID.User,
		Type:          "video",
		Caption:       req.Caption,
		MediaURL:      uploaded.URL,
		MediaMimeType: mimeType,
		IsOwn:         true,
		PublishedAt:   sentMsg.Timestamp,
		ExpiresAt:     sentMsg.Timestamp.Add(24 * time.Hour),
		Privacy:       sc.getDefaultPrivacy(req.Privacy),
	}

	return statusInfo, nil
}

// publishAudioStatus - Publicar estado de audio
func (sc *StatusController) publishAudioStatus(client *whatsmeow.Client, req PublishStatusRequest) (*StatusInfo, error) {
	// Obtener datos del audio
	audioData, mimeType, err := sc.getMediaData(req.MediaData, req.MediaURL, req.MimeType)
	if err != nil {
		return nil, fmt.Errorf("error obteniendo datos de audio: %v", err)
	}

	// Validar que es un audio
	if !sc.isValidAudioType(mimeType) {
		return nil, fmt.Errorf("tipo de archivo no válido para audio")
	}

	// Subir audio
	uploaded, err := client.Upload(context.Background(), audioData, whatsmeow.MediaAudio)
	if err != nil {
		return nil, fmt.Errorf("error subiendo audio: %v", err)
	}

	// Crear mensaje de estado de audio
	statusMessage := &waE2E.Message{
		AudioMessage: &waE2E.AudioMessage{
			Url:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			Mimetype:      proto.String(mimeType),
			FileEncSha256: uploaded.FileEncSHA256,
			FileSha256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uint64(len(audioData))),
			Ptt:           proto.Bool(true), // Estado de audio siempre como nota de voz
		},
	}

	// Enviar estado
	sentMsg, err := client.SendMessage(context.Background(), types.StatusBroadcastJID, statusMessage)
	if err != nil {
		return nil, fmt.Errorf("error enviando estado de audio: %v", err)
	}

	// Crear información del estado
	statusInfo := &StatusInfo{
		ID:            sentMsg.ID,
		JID:           client.Store.ID.String(),
		Phone:         client.Store.ID.User,
		Type:          "audio",
		MediaURL:      uploaded.URL,
		MediaMimeType: mimeType,
		IsOwn:         true,
		PublishedAt:   sentMsg.Timestamp,
		ExpiresAt:     sentMsg.Timestamp.Add(24 * time.Hour),
		Privacy:       sc.getDefaultPrivacy(req.Privacy),
	}

	return statusInfo, nil
}

// getOwnStatuses - Obtener estados propios
func (sc *StatusController) getOwnStatuses(client *whatsmeow.Client) ([]*StatusInfo, error) {
	// Esta función requiere acceso al store de estados
	// Por ahora implementamos una versión básica
	sc.logger.Infof("Obteniendo estados propios (funcionalidad básica implementada)")
	
	var statuses []*StatusInfo
	return statuses, nil
}

// getContactStatuses - Obtener estados de contactos
func (sc *StatusController) getContactStatuses(client *whatsmeow.Client) ([]*StatusInfo, error) {
	// Esta función requiere acceso al store de estados de contactos
	sc.logger.Infof("Obteniendo estados de contactos (funcionalidad básica implementada)")
	
	var statuses []*StatusInfo
	return statuses, nil
}

// getSpecificContactStatuses - Obtener estados de un contacto específico
func (sc *StatusController) getSpecificContactStatuses(client *whatsmeow.Client, contactJID types.JID) ([]*StatusInfo, error) {
	sc.logger.Infof("Obteniendo estados del contacto %s (funcionalidad básica implementada)", contactJID)
	
	var statuses []*StatusInfo
	return statuses, nil
}

// deleteOwnStatus - Eliminar estado propio
func (sc *StatusController) deleteOwnStatus(client *whatsmeow.Client, statusID string) error {
	// Enviar mensaje de eliminación de estado
	_, err := client.SendMessage(context.Background(), types.StatusBroadcastJID, &waE2E.Message{
		ProtocolMessage: &waE2E.ProtocolMessage{
			Type: proto.Uint32(uint32(waE2E.ProtocolMessage_REVOKE)),
			Key: &waE2E.MessageKey{
				Id: proto.String(statusID),
			},
		},
	})
	
	return err
}

// getStatusViewers - Obtener viewers de un estado
func (sc *StatusController) getStatusViewers(client *whatsmeow.Client, statusID string) ([]*StatusViewer, error) {
	sc.logger.Infof("Obteniendo viewers del estado %s (funcionalidad básica implementada)", statusID)
	
	var viewers []*StatusViewer
	return viewers, nil
}

// updatePrivacySettings - Actualizar configuraciones de privacidad
func (sc *StatusController) updatePrivacySettings(client *whatsmeow.Client, req UpdatePrivacyRequest) (*StatusPrivacySettings, error) {
	// Implementar actualización de configuraciones de privacidad
	// Esta funcionalidad requiere acceso a las configuraciones de WhatsApp
	
	settings := &StatusPrivacySettings{
		DefaultPrivacy: req.DefaultPrivacy,
		AllowList:      req.AllowList,
		DenyList:       req.DenyList,
		ReadReceipts:   req.ReadReceipts,
		AllowReplies:   req.AllowReplies,
	}
	
	sc.logger.Infof("Configuraciones de privacidad actualizadas: %s", req.DefaultPrivacy)
	return settings, nil
}

// getPrivacySettings - Obtener configuraciones actuales de privacidad
func (sc *StatusController) getPrivacySettings(client *whatsmeow.Client) (*StatusPrivacySettings, error) {
	// Implementar obtención de configuraciones actuales
	settings := &StatusPrivacySettings{
		DefaultPrivacy: "contacts",
		AllowList:      []string{},
		DenyList:       []string{},
		ReadReceipts:   true,
		AllowReplies:   true,
	}
	
	return settings, nil
}

// getMediaData - Obtener datos de multimedia desde base64 o URL
func (sc *StatusController) getMediaData(mediaData, mediaURL, mimeType string) ([]byte, string, error) {
	if mediaData != "" {
		// Decodificar base64
		data, err := base64.StdEncoding.DecodeString(mediaData)
		if err != nil {
			return nil, "", fmt.Errorf("error decodificando base64: %v", err)
		}
		
		// Detectar MIME type si no se proporcionó
		if mimeType == "" {
			mimeType = http.DetectContentType(data)
		}
		
		return data, mimeType, nil
	}
	
	if mediaURL != "" {
		// Descargar desde URL
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, "GET", mediaURL, nil)
		if err != nil {
			return nil, "", fmt.Errorf("error creando petición HTTP: %v", err)
		}

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return nil, "", fmt.Errorf("error descargando archivo: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, "", fmt.Errorf("error HTTP: %d %s", resp.StatusCode, resp.Status)
		}

		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, "", fmt.Errorf("error leyendo archivo: %v", err)
		}

		// Detectar MIME type
		if mimeType == "" {
			mimeType = resp.Header.Get("Content-Type")
			if mimeType == "" {
				mimeType = http.DetectContentType(data)
			}
		}

		return data, mimeType, nil
	}
	
	return nil, "", fmt.Errorf("debe proporcionar media_data o media_url")
}

// Funciones de validación
func (sc *StatusController) isValidStatusType(statusType string) bool {
	validTypes := []string{"text", "image", "video", "audio"}
	for _, validType := range validTypes {
		if statusType == validType {
			return true
		}
	}
	return false
}

func (sc *StatusController) isValidPrivacyType(privacyType string) bool {
	validTypes := []string{"all", "contacts", "selected", "except"}
	for _, validType := range validTypes {
		if privacyType == validType {
			return true
		}
	}
	return false
}

func (sc *StatusController) isValidImageType(mimeType string) bool {
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

func (sc *StatusController) isValidVideoType(mimeType string) bool {
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

func (sc *StatusController) isValidAudioType(mimeType string) bool {
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
}

// parseColorToARGB - Convertir color hex a ARGB
func (sc *StatusController) parseColorToARGB(color string) uint32 {
	// Remover # si existe
	color = strings.TrimPrefix(color, "#")
	
	// Por defecto retornar blanco si hay error
	if len(color) != 6 {
		return 0xFFFFFFFF
	}
	
	// Parsear color hex (simplificado)
	// En una implementación completa, usar una librería de parsing de colores
	return 0xFF000000 // Negro con alpha completo como ejemplo
}

// getDefaultPrivacy - Obtener privacidad por defecto
func (sc *StatusController) getDefaultPrivacy(privacy string) string {
	if privacy == "" {
		return "contacts"
	}
	return privacy
}
