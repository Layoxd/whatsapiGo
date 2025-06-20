package controllers

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	waLog "go.mau.fi/whatsmeow/util/log"
	"github.com/google/uuid"
)

// InstanceController - Controlador para manejar instancias de WhatsApp
type InstanceController struct {
	store         *sqlstore.Container // Almacén de sesiones de WhatsMeow
	instances     map[string]*WhatsAppInstance // Mapa de instancias activas
	logger        waLog.Logger // Logger de WhatsMeow
}

// WhatsAppInstance - Estructura que representa una instancia de WhatsApp
type WhatsAppInstance struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Phone       string              `json:"phone,omitempty"`
	Status      string              `json:"status"` // disconnected, connecting, connected, qr_pending
	QRCode      string              `json:"qr_code,omitempty"`
	Client      *whatsmeow.Client   `json:"-"`
	Device      *store.Device       `json:"-"`
	CreatedAt   time.Time           `json:"created_at"`
	UpdatedAt   time.Time           `json:"updated_at"`
	LastSeen    *time.Time          `json:"last_seen,omitempty"`
	UserJID     *types.JID          `json:"user_jid,omitempty"`
}

// CreateInstanceRequest - Estructura para crear una nueva instancia
type CreateInstanceRequest struct {
	Name        string `json:"name" binding:"required"`
	Webhook     string `json:"webhook,omitempty"`
	WebhookBase64 bool `json:"webhook_base64,omitempty"`
}

// InstanceResponse - Respuesta estándar para operaciones de instancia
type InstanceResponse struct {
	Success bool                `json:"success"`
	Message string              `json:"message"`
	Data    *WhatsAppInstance   `json:"data,omitempty"`
}

// InstanceListResponse - Respuesta para listar instancias
type InstanceListResponse struct {
	Success   bool                 `json:"success"`
	Message   string               `json:"message"`
	Instances []*WhatsAppInstance  `json:"instances"`
	Total     int                  `json:"total"`
}

// QRResponse - Respuesta específica para QR codes
type QRResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	QRCode  string `json:"qr_code,omitempty"`
	Base64  string `json:"base64,omitempty"`
}

// NewInstanceController - Constructor del controlador de instancias
func NewInstanceController(store *sqlstore.Container, logger waLog.Logger) *InstanceController {
	return &InstanceController{
		store:     store,
		instances: make(map[string]*WhatsAppInstance),
		logger:    logger,
	}
}

// CreateInstance - POST /instances - Crear nueva instancia de WhatsApp
func (ic *InstanceController) CreateInstance(c *gin.Context) {
	var req CreateInstanceRequest
	
	// Validar el JSON de la petición
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, InstanceResponse{
			Success: false,
			Message: fmt.Sprintf("Error en los datos enviados: %v", err),
		})
		return
	}

	// Generar ID único para la instancia
	instanceID := uuid.New().String()
	
	// Crear dispositivo en el store de WhatsMeow
	device, err := ic.store.GetFirstDevice()
	if err != nil {
		ic.logger.Errorf("Error al obtener dispositivo del store: %v", err)
		c.JSON(http.StatusInternalServerError, InstanceResponse{
			Success: false,
			Message: "Error interno al crear la instancia",
		})
		return
	}

	// Si no hay dispositivo, crear uno nuevo
	if device == nil {
		device = ic.store.NewDevice()
	}

	// Crear cliente de WhatsMeow
	client := whatsmeow.NewClient(device, ic.logger)
	
	// Crear instancia
	instance := &WhatsAppInstance{
		ID:        instanceID,
		Name:      req.Name,
		Status:    "disconnected",
		Client:    client,
		Device:    device,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Configurar event handlers para la instancia
	ic.setupEventHandlers(instance)

	// Guardar instancia en memoria
	ic.instances[instanceID] = instance

	ic.logger.Infof("Nueva instancia creada: %s (%s)", instanceID, req.Name)

	c.JSON(http.StatusCreated, InstanceResponse{
		Success: true,
		Message: "Instancia creada exitosamente",
		Data:    instance,
	})
}

// GetInstances - GET /instances - Listar todas las instancias
func (ic *InstanceController) GetInstances(c *gin.Context) {
	instances := make([]*WhatsAppInstance, 0, len(ic.instances))
	
	// Convertir mapa a slice para la respuesta
	for _, instance := range ic.instances {
		// Actualizar estado actual antes de responder
		ic.updateInstanceStatus(instance)
		instances = append(instances, instance)
	}

	c.JSON(http.StatusOK, InstanceListResponse{
		Success:   true,
		Message:   "Instancias obtenidas exitosamente",
		Instances: instances,
		Total:     len(instances),
	})
}

// GetInstance - GET /instances/:id - Obtener instancia específica
func (ic *InstanceController) GetInstance(c *gin.Context) {
	instanceID := c.Param("id")
	
	instance, exists := ic.instances[instanceID]
	if !exists {
		c.JSON(http.StatusNotFound, InstanceResponse{
			Success: false,
			Message: "Instancia no encontrada",
		})
		return
	}

	// Actualizar estado actual
	ic.updateInstanceStatus(instance)

	c.JSON(http.StatusOK, InstanceResponse{
		Success: true,
		Message: "Instancia obtenida exitosamente",
		Data:    instance,
	})
}

// DeleteInstance - DELETE /instances/:id - Eliminar instancia
func (ic *InstanceController) DeleteInstance(c *gin.Context) {
	instanceID := c.Param("id")
	
	instance, exists := ic.instances[instanceID]
	if !exists {
		c.JSON(http.StatusNotFound, InstanceResponse{
			Success: false,
			Message: "Instancia no encontrada",
		})
		return
	}

	// Desconectar cliente si está conectado
	if instance.Client.IsConnected() {
		instance.Client.Disconnect()
	}

	// Eliminar instancia del mapa
	delete(ic.instances, instanceID)

	ic.logger.Infof("Instancia eliminada: %s", instanceID)

	c.JSON(http.StatusOK, InstanceResponse{
		Success: true,
		Message: "Instancia eliminada exitosamente",
	})
}

// GetQRCode - GET /instances/:id/qr - Generar código QR para conexión
func (ic *InstanceController) GetQRCode(c *gin.Context) {
	instanceID := c.Param("id")
	
	instance, exists := ic.instances[instanceID]
	if !exists {
		c.JSON(http.StatusNotFound, QRResponse{
			Success: false,
			Message: "Instancia no encontrada",
		})
		return
	}

	// Verificar si ya está conectado
	if instance.Client.IsConnected() {
		c.JSON(http.StatusBadRequest, QRResponse{
			Success: false,
			Message: "La instancia ya está conectada",
		})
		return
	}

	// Iniciar proceso de conexión
	qrChan, err := instance.Client.GetQRChannel(context.Background())
	if err != nil {
		ic.logger.Errorf("Error al obtener canal QR: %v", err)
		c.JSON(http.StatusInternalServerError, QRResponse{
			Success: false,
			Message: "Error al generar código QR",
		})
		return
	}

	// Conectar cliente
	err = instance.Client.Connect()
	if err != nil {
		ic.logger.Errorf("Error al conectar cliente: %v", err)
		c.JSON(http.StatusInternalServerError, QRResponse{
			Success: false,
			Message: "Error al conectar con WhatsApp",
		})
		return
	}

	// Actualizar estado
	instance.Status = "qr_pending"
	instance.UpdatedAt = time.Now()

	// Configurar timeout para QR
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Esperar por QR code
	select {
	case evt := <-qrChan:
		switch evt.Event {
		case "code":
			// Generar QR code en base64
			qrBase64 := base64.StdEncoding.EncodeToString([]byte(evt.Code))
			
			instance.QRCode = evt.Code
			instance.UpdatedAt = time.Now()

			ic.logger.Infof("QR generado para instancia: %s", instanceID)

			c.JSON(http.StatusOK, QRResponse{
				Success: true,
				Message: "Código QR generado exitosamente",
				QRCode:  evt.Code,
				Base64:  qrBase64,
			})
			return

		case "success":
			instance.Status = "connected"
			instance.QRCode = ""
			instance.UpdatedAt = time.Now()
			
			c.JSON(http.StatusOK, QRResponse{
				Success: true,
				Message: "Conectado exitosamente",
			})
			return

		default:
			ic.logger.Warnf("Evento QR no manejado: %s", evt.Event)
		}

	case <-ctx.Done():
		instance.Status = "disconnected"
		instance.UpdatedAt = time.Now()
		
		c.JSON(http.StatusRequestTimeout, QRResponse{
			Success: false,
			Message: "Timeout: No se pudo generar el código QR",
		})
		return
	}
}

// LogoutInstance - POST /instances/:id/logout - Desconectar instancia
func (ic *InstanceController) LogoutInstance(c *gin.Context) {
	instanceID := c.Param("id")
	
	instance, exists := ic.instances[instanceID]
	if !exists {
		c.JSON(http.StatusNotFound, InstanceResponse{
			Success: false,
			Message: "Instancia no encontrada",
		})
		return
	}

	// Verificar si está conectado
	if !instance.Client.IsConnected() {
		c.JSON(http.StatusBadRequest, InstanceResponse{
			Success: false,
			Message: "La instancia no está conectada",
		})
		return
	}

	// Desconectar
	instance.Client.Disconnect()
	
	// Actualizar estado
	instance.Status = "disconnected"
	instance.QRCode = ""
	instance.Phone = ""
	instance.UserJID = nil
	instance.UpdatedAt = time.Now()

	ic.logger.Infof("Instancia desconectada: %s", instanceID)

	c.JSON(http.StatusOK, InstanceResponse{
		Success: true,
		Message: "Instancia desconectada exitosamente",
		Data:    instance,
	})
}

// setupEventHandlers - Configurar manejadores de eventos para la instancia
func (ic *InstanceController) setupEventHandlers(instance *WhatsAppInstance) {
	instance.Client.AddEventHandler(func(evt interface{}) {
		switch v := evt.(type) {
		case *events.Connected:
			// Cliente conectado exitosamente
			instance.Status = "connected"
			instance.UpdatedAt = time.Now()
			instance.LastSeen = &instance.UpdatedAt
			
			// Obtener información del usuario
			if v.JID != nil {
				instance.UserJID = v.JID
				instance.Phone = v.JID.User
			}
			
			ic.logger.Infof("Instancia conectada: %s (%s)", instance.ID, instance.Phone)

		case *events.Disconnected:
			// Cliente desconectado
			instance.Status = "disconnected"
			instance.UpdatedAt = time.Now()
			ic.logger.Infof("Instancia desconectada: %s", instance.ID)

		case *events.LoggedOut:
			// Sesión cerrada
			instance.Status = "disconnected"
			instance.Phone = ""
			instance.UserJID = nil
			instance.QRCode = ""
			instance.UpdatedAt = time.Now()
			ic.logger.Infof("Sesión cerrada para instancia: %s", instance.ID)
		}
	})
}

// updateInstanceStatus - Actualizar estado actual de la instancia
func (ic *InstanceController) updateInstanceStatus(instance *WhatsAppInstance) {
	if instance.Client.IsConnected() {
		if instance.Status != "connected" {
			instance.Status = "connected"
			instance.UpdatedAt = time.Now()
			now := time.Now()
			instance.LastSeen = &now
		}
	} else {
		if instance.Status == "connected" {
			instance.Status = "disconnected"
			instance.UpdatedAt = time.Now()
		}
	}
}// Archivo base: instance_controller.go
