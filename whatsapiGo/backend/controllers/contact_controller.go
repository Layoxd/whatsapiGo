// Archivo base: contact_controller.gopackage controllers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
	waLog "go.mau.fi/whatsmeow/util/log"
)

// ContactController - Controlador para gestión de contactos con soporte LID
type ContactController struct {
	instanceController *InstanceController // Referencia al controlador de instancias
	logger             waLog.Logger        // Logger de WhatsMeow
}

// ContactInfo - Información completa de un contacto
type ContactInfo struct {
	JID          string             `json:"jid"`                    // JID tradicional (con número)
	LID          string             `json:"lid,omitempty"`          // LID (Link ID) privado
	Phone        string             `json:"phone"`                  // Número de teléfono limpio
	Name         string             `json:"name,omitempty"`         // Nombre del contacto
	PushName     string             `json:"push_name,omitempty"`    // Nombre de WhatsApp
	BusinessName string             `json:"business_name,omitempty"` // Nombre de negocio
	Status       string             `json:"status,omitempty"`       // Estado/About
	Avatar       *AvatarInfo        `json:"avatar,omitempty"`       // Información del avatar
	VerifiedName *VerifiedNameInfo  `json:"verified_name,omitempty"` // Nombre verificado de empresa
	Devices      []string           `json:"devices,omitempty"`      // Lista de dispositivos
	IsBlocked    bool               `json:"is_blocked"`             // Si está bloqueado
	IsInContacts bool               `json:"is_in_contacts"`         // Si está en contactos
	IsOnWhatsApp bool               `json:"is_on_whatsapp"`         // Si está registrado en WhatsApp
	LastSeen     *time.Time         `json:"last_seen,omitempty"`    // Última conexión
	UpdatedAt    time.Time          `json:"updated_at"`             // Última actualización
}

// AvatarInfo - Información del avatar
type AvatarInfo struct {
	ID       string `json:"id"`                // ID del avatar
	URL      string `json:"url,omitempty"`     // URL del avatar
	Type     string `json:"type,omitempty"`    // Tipo (individual, group)
	Preview  string `json:"preview,omitempty"` // Preview en base64
	FullSize string `json:"full_size,omitempty"` // Avatar completo en base64
}

// VerifiedNameInfo - Información de nombre verificado
type VerifiedNameInfo struct {
	Certificate *VerifiedNameCertificate `json:"certificate,omitempty"`
	Details     *VerifiedNameDetails     `json:"details,omitempty"`
}

type VerifiedNameCertificate struct {
	Details    string `json:"details"`
	Signature  string `json:"signature"`
	ServerSignature string `json:"server_signature"`
}

type VerifiedNameDetails struct {
	Serial            uint64 `json:"serial"`
	Issuer            string `json:"issuer"`
	VerifiedName      string `json:"verified_name"`
	LocalizedNames    []LocalizedName `json:"localized_names,omitempty"`
	IssueTime         uint64 `json:"issue_time"`
}

type LocalizedName struct {
	Locale string `json:"locale"`
	Name   string `json:"name"`
}

// SearchContactsRequest - Petición para buscar contactos
type SearchContactsRequest struct {
	InstanceID string `json:"instance_id" binding:"required"`
	Query      string `json:"query" binding:"required"`      // Término de búsqueda
	Type       string `json:"type,omitempty"`                // phone, name, jid, lid, all
	Limit      int    `json:"limit,omitempty"`               // Límite de resultados (default: 50)
}

// CheckContactsRequest - Petición para verificar contactos en WhatsApp
type CheckContactsRequest struct {
	InstanceID string   `json:"instance_id" binding:"required"`
	Phones     []string `json:"phones" binding:"required"` // Lista de números a verificar
}

// BlockContactRequest - Petición para bloquear/desbloquear contacto
type BlockContactRequest struct {
	InstanceID string `json:"instance_id" binding:"required"`
	JID        string `json:"jid,omitempty"`    // JID del contacto
	LID        string `json:"lid,omitempty"`    // LID del contacto (alternativo a JID)
	Phone      string `json:"phone,omitempty"`  // Número de teléfono (alternativo)
}

// ConvertJIDLIDRequest - Petición para conversión JID ↔ LID
type ConvertJIDLIDRequest struct {
	InstanceID string `json:"instance_id" binding:"required"`
	Identifier string `json:"identifier" binding:"required"` // JID, LID o número
}

// ContactResponse - Respuesta estándar para operaciones de contactos
type ContactResponse struct {
	Success bool         `json:"success"`
	Message string       `json:"message"`
	Data    *ContactInfo `json:"data,omitempty"`
}

// ContactListResponse - Respuesta para listar contactos
type ContactListResponse struct {
	Success  bool           `json:"success"`
	Message  string         `json:"message"`
	Contacts []*ContactInfo `json:"contacts"`
	Total    int            `json:"total"`
	HasMore  bool           `json:"has_more"`
}

// CheckContactsResponse - Respuesta para verificación de contactos
type CheckContactsResponse struct {
	Success bool                          `json:"success"`
	Message string                        `json:"message"`
	Results []types.IsOnWhatsAppResponse `json:"results"`
}

// ConvertJIDLIDResponse - Respuesta para conversión JID ↔ LID
type ConvertJIDLIDResponse struct {
	Success      bool   `json:"success"`
	Message      string `json:"message"`
	OriginalJID  string `json:"original_jid,omitempty"`
	OriginalLID  string `json:"original_lid,omitempty"`
	ConvertedJID string `json:"converted_jid,omitempty"`
	ConvertedLID string `json:"converted_lid,omitempty"`
	Phone        string `json:"phone,omitempty"`
}

// NewContactController - Constructor del controlador de contactos
func NewContactController(instanceController *InstanceController, logger waLog.Logger) *ContactController {
	return &ContactController{
		instanceController: instanceController,
		logger:             logger,
	}
}

// GetContacts - GET /contacts/{instanceId} - Listar todos los contactos
func (cc *ContactController) GetContacts(c *gin.Context) {
	instanceID := c.Param("instanceId")
	
	// Obtener instancia
	instance, exists := cc.instanceController.instances[instanceID]
	if !exists {
		c.JSON(http.StatusNotFound, ContactListResponse{
			Success: false,
			Message: "Instancia no encontrada",
		})
		return
	}

	if !instance.Client.IsConnected() {
		c.JSON(http.StatusBadRequest, ContactListResponse{
			Success: false,
			Message: "La instancia no está conectada a WhatsApp",
		})
		return
	}

	// Obtener contactos desde el store
	contacts := cc.getAllContactsFromStore(instance.Client)
	
	cc.logger.Infof("Contactos obtenidos: %d para instancia %s", len(contacts), instanceID)

	c.JSON(http.StatusOK, ContactListResponse{
		Success:  true,
		Message:  "Contactos obtenidos exitosamente",
		Contacts: contacts,
		Total:    len(contacts),
		HasMore:  false,
	})
}

// SearchContacts - GET /contacts/{instanceId}/search - Buscar contactos
func (cc *ContactController) SearchContacts(c *gin.Context) {
	instanceID := c.Param("instanceId")
	query := c.Query("q")
	searchType := c.DefaultQuery("type", "all")
	
	if query == "" {
		c.JSON(http.StatusBadRequest, ContactListResponse{
			Success: false,
			Message: "El parámetro 'q' (query) es requerido",
		})
		return
	}

	// Obtener instancia
	instance, exists := cc.instanceController.instances[instanceID]
	if !exists {
		c.JSON(http.StatusNotFound, ContactListResponse{
			Success: false,
			Message: "Instancia no encontrada",
		})
		return
	}

	if !instance.Client.IsConnected() {
		c.JSON(http.StatusBadRequest, ContactListResponse{
			Success: false,
			Message: "La instancia no está conectada a WhatsApp",
		})
		return
	}

	// Buscar contactos
	contacts := cc.searchContactsByQuery(instance.Client, query, searchType)
	
	cc.logger.Infof("Búsqueda de contactos '%s' tipo '%s': %d resultados", query, searchType, len(contacts))

	c.JSON(http.StatusOK, ContactListResponse{
		Success:  true,
		Message:  fmt.Sprintf("Búsqueda completada: %d contactos encontrados", len(contacts)),
		Contacts: contacts,
		Total:    len(contacts),
		HasMore:  false,
	})
}

// GetContactInfo - GET /contacts/{instanceId}/info/{jid} - Información completa de contacto
func (cc *ContactController) GetContactInfo(c *gin.Context) {
	instanceID := c.Param("instanceId")
	jidStr := c.Param("jid")
	
	// Obtener instancia
	instance, exists := cc.instanceController.instances[instanceID]
	if !exists {
		c.JSON(http.StatusNotFound, ContactResponse{
			Success: false,
			Message: "Instancia no encontrada",
		})
		return
	}

	if !instance.Client.IsConnected() {
		c.JSON(http.StatusBadRequest, ContactResponse{
			Success: false,
			Message: "La instancia no está conectada a WhatsApp",
		})
		return
	}

	// Parsear JID
	jid, err := types.ParseJID(jidStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ContactResponse{
			Success: false,
			Message: fmt.Sprintf("JID inválido: %v", err),
		})
		return
	}

	// Obtener información completa del contacto
	contactInfo, err := cc.getDetailedContactInfo(instance.Client, jid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ContactResponse{
			Success: false,
			Message: fmt.Sprintf("Error obteniendo información del contacto: %v", err),
		})
		return
	}

	if contactInfo == nil {
		c.JSON(http.StatusNotFound, ContactResponse{
			Success: false,
			Message: "Contacto no encontrado",
		})
		return
	}

	cc.logger.Infof("Información de contacto obtenida: %s", jidStr)

	c.JSON(http.StatusOK, ContactResponse{
		Success: true,
		Message: "Información del contacto obtenida exitosamente",
		Data:    contactInfo,
	})
}

// CheckContacts - POST /contacts/{instanceId}/check - Verificar si números están en WhatsApp
func (cc *ContactController) CheckContacts(c *gin.Context) {
	instanceID := c.Param("instanceId")
	var req CheckContactsRequest
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, CheckContactsResponse{
			Success: false,
			Message: fmt.Sprintf("Error en los datos enviados: %v", err),
		})
		return
	}

	// Validar que el instanceID coincida
	if req.InstanceID != instanceID {
		req.InstanceID = instanceID
	}

	// Obtener instancia
	instance, exists := cc.instanceController.instances[req.InstanceID]
	if !exists {
		c.JSON(http.StatusNotFound, CheckContactsResponse{
			Success: false,
			Message: "Instancia no encontrada",
		})
		return
	}

	if !instance.Client.IsConnected() {
		c.JSON(http.StatusBadRequest, CheckContactsResponse{
			Success: false,
			Message: "La instancia no está conectada a WhatsApp",
		})
		return
	}

	// Verificar contactos en WhatsApp
	results, err := instance.Client.IsOnWhatsApp(req.Phones)
	if err != nil {
		cc.logger.Errorf("Error verificando contactos: %v", err)
		c.JSON(http.StatusInternalServerError, CheckContactsResponse{
			Success: false,
			Message: "Error verificando contactos en WhatsApp",
		})
		return
	}

	cc.logger.Infof("Verificación de %d números completada: %d en WhatsApp", len(req.Phones), len(results))

	c.JSON(http.StatusOK, CheckContactsResponse{
		Success: true,
		Message: fmt.Sprintf("Verificación completada: %d de %d números están en WhatsApp", len(results), len(req.Phones)),
		Results: results,
	})
}

// BlockContact - POST /contacts/{instanceId}/block - Bloquear contacto
func (cc *ContactController) BlockContact(c *gin.Context) {
	instanceID := c.Param("instanceId")
	var req BlockContactRequest
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ContactResponse{
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
	instance, exists := cc.instanceController.instances[req.InstanceID]
	if !exists {
		c.JSON(http.StatusNotFound, ContactResponse{
			Success: false,
			Message: "Instancia no encontrada",
		})
		return
	}

	if !instance.Client.IsConnected() {
		c.JSON(http.StatusBadRequest, ContactResponse{
			Success: false,
			Message: "La instancia no está conectada a WhatsApp",
		})
		return
	}

	// Determinar el JID a bloquear
	jid, err := cc.resolveContactJID(instance.Client, req.JID, req.LID, req.Phone)
	if err != nil {
		c.JSON(http.StatusBadRequest, ContactResponse{
			Success: false,
			Message: fmt.Sprintf("Error resolviendo contacto: %v", err),
		})
		return
	}

	// Bloquear contacto
	err = instance.Client.UpdateBlocklist(jid, "add")
	if err != nil {
		cc.logger.Errorf("Error bloqueando contacto %s: %v", jid, err)
		c.JSON(http.StatusInternalServerError, ContactResponse{
			Success: false,
			Message: "Error bloqueando el contacto",
		})
		return
	}

	cc.logger.Infof("Contacto bloqueado: %s", jid)

	c.JSON(http.StatusOK, ContactResponse{
		Success: true,
		Message: "Contacto bloqueado exitosamente",
		Data: &ContactInfo{
			JID:       jid.String(),
			IsBlocked: true,
			UpdatedAt: time.Now(),
		},
	})
}

// UnblockContact - POST /contacts/{instanceId}/unblock - Desbloquear contacto
func (cc *ContactController) UnblockContact(c *gin.Context) {
	instanceID := c.Param("instanceId")
	var req BlockContactRequest
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ContactResponse{
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
	instance, exists := cc.instanceController.instances[req.InstanceID]
	if !exists {
		c.JSON(http.StatusNotFound, ContactResponse{
			Success: false,
			Message: "Instancia no encontrada",
		})
		return
	}

	if !instance.Client.IsConnected() {
		c.JSON(http.StatusBadRequest, ContactResponse{
			Success: false,
			Message: "La instancia no está conectada a WhatsApp",
		})
		return
	}

	// Determinar el JID a desbloquear
	jid, err := cc.resolveContactJID(instance.Client, req.JID, req.LID, req.Phone)
	if err != nil {
		c.JSON(http.StatusBadRequest, ContactResponse{
			Success: false,
			Message: fmt.Sprintf("Error resolviendo contacto: %v", err),
		})
		return
	}

	// Desbloquear contacto
	err = instance.Client.UpdateBlocklist(jid, "remove")
	if err != nil {
		cc.logger.Errorf("Error desbloqueando contacto %s: %v", jid, err)
		c.JSON(http.StatusInternalServerError, ContactResponse{
			Success: false,
			Message: "Error desbloqueando el contacto",
		})
		return
	}

	cc.logger.Infof("Contacto desbloqueado: %s", jid)

	c.JSON(http.StatusOK, ContactResponse{
		Success: true,
		Message: "Contacto desbloqueado exitosamente",
		Data: &ContactInfo{
			JID:       jid.String(),
			IsBlocked: false,
			UpdatedAt: time.Now(),
		},
	})
}

// GetLIDFromJID - GET /contacts/{instanceId}/lid/get - Convertir JID/Phone a LID
func (cc *ContactController) GetLIDFromJID(c *gin.Context) {
	instanceID := c.Param("instanceId")
	identifier := c.Query("identifier") // JID, número o lo que sea
	
	if identifier == "" {
		c.JSON(http.StatusBadRequest, ConvertJIDLIDResponse{
			Success: false,
			Message: "El parámetro 'identifier' es requerido",
		})
		return
	}

	// Obtener instancia
	instance, exists := cc.instanceController.instances[instanceID]
	if !exists {
		c.JSON(http.StatusNotFound, ConvertJIDLIDResponse{
			Success: false,
			Message: "Instancia no encontrada",
		})
		return
	}

	// Parsear y limpiar identificador
	jid, err := cc.parseAndCleanIdentifier(identifier)
	if err != nil {
		c.JSON(http.StatusBadRequest, ConvertJIDLIDResponse{
			Success: false,
			Message: fmt.Sprintf("Identificador inválido: %v", err),
		})
		return
	}

	// Solo convertir si es un JID de usuario (no LID)
	if jid.Server != types.DefaultUserServer {
		c.JSON(http.StatusBadRequest, ConvertJIDLIDResponse{
			Success: false,
			Message: "Solo se pueden convertir JIDs de usuario a LID",
		})
		return
	}

	// Obtener LID desde el store
	lidJID, err := instance.Client.Store.GetLIDMapping(context.Background(), jid, true)
	if err != nil {
		cc.logger.Errorf("Error obteniendo LID para %s: %v", jid, err)
		c.JSON(http.StatusInternalServerError, ConvertJIDLIDResponse{
			Success: false,
			Message: "Error convirtiendo a LID",
		})
		return
	}

	var result ConvertJIDLIDResponse
	result.Success = true
	result.OriginalJID = jid.String()
	result.Phone = jid.User
	
	if lidJID.Server == types.HiddenUserServer && lidJID.User != "" {
		result.ConvertedLID = lidJID.String()
		result.Message = "Conversión JID → LID exitosa"
	} else {
		result.Message = "No se encontró LID para este JID (puede ser un contacto sin LID asignado)"
	}

	cc.logger.Infof("Conversión JID→LID: %s → %s", jid, result.ConvertedLID)

	c.JSON(http.StatusOK, result)
}

// GetJIDFromLID - GET /contacts/{instanceId}/lid/from-lid - Convertir LID a JID/Phone
func (cc *ContactController) GetJIDFromLID(c *gin.Context) {
	instanceID := c.Param("instanceId")
	lidStr := c.Query("lid")
	
	if lidStr == "" {
		c.JSON(http.StatusBadRequest, ConvertJIDLIDResponse{
			Success: false,
			Message: "El parámetro 'lid' es requerido",
		})
		return
	}

	// Obtener instancia
	instance, exists := cc.instanceController.instances[instanceID]
	if !exists {
		c.JSON(http.StatusNotFound, ConvertJIDLIDResponse{
			Success: false,
			Message: "Instancia no encontrada",
		})
		return
	}

	// Parsear LID
	lidJID, err := types.ParseJID(lidStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ConvertJIDLIDResponse{
			Success: false,
			Message: fmt.Sprintf("LID inválido: %v", err),
		})
		return
	}

	// Validar que es un LID
	if lidJID.Server != types.HiddenUserServer {
		c.JSON(http.StatusBadRequest, ConvertJIDLIDResponse{
			Success: false,
			Message: "El identificador proporcionado no es un LID válido",
		})
		return
	}

	// Obtener JID desde el store
	jid, err := instance.Client.Store.GetLIDMapping(context.Background(), lidJID, false)
	if err != nil {
		cc.logger.Errorf("Error obteniendo JID para LID %s: %v", lidJID, err)
		c.JSON(http.StatusInternalServerError, ConvertJIDLIDResponse{
			Success: false,
			Message: "Error convirtiendo LID a JID",
		})
		return
	}

	var result ConvertJIDLIDResponse
	result.Success = true
	result.OriginalLID = lidJID.String()
	
	if jid.Server == types.DefaultUserServer && jid.User != "" {
		result.ConvertedJID = jid.String()
		result.Phone = jid.User
		result.Message = "Conversión LID → JID exitosa"
	} else {
		result.Message = "No se encontró JID para este LID"
	}

	cc.logger.Infof("Conversión LID→JID: %s → %s", lidJID, result.ConvertedJID)

	c.JSON(http.StatusOK, result)
}

// getAllContactsFromStore - Obtener todos los contactos del store
func (cc *ContactController) getAllContactsFromStore(client *whatsmeow.Client) []*ContactInfo {
	var contacts []*ContactInfo
	
	// Esta función necesitaría acceso directo al store de contactos
	// Por ahora retornamos una lista vacía y sugiero implementar
	// a través de la funcionalidad GetUserInfo de WhatsApp
	
	cc.logger.Infof("Obteniendo contactos desde store (funcionalidad por implementar)")
	
	return contacts
}

// searchContactsByQuery - Buscar contactos por consulta
func (cc *ContactController) searchContactsByQuery(client *whatsmeow.Client, query, searchType string) []*ContactInfo {
	var contacts []*ContactInfo
	
	// Implementar búsqueda basada en el tipo
	switch searchType {
	case "phone":
		// Buscar por número de teléfono
		contacts = cc.searchByPhone(client, query)
	case "name":
		// Buscar por nombre (requiere acceso al store de contactos)
		contacts = cc.searchByName(client, query)
	case "jid":
		// Buscar por JID específico
		contacts = cc.searchByJID(client, query)
	case "lid":
		// Buscar por LID específico
		contacts = cc.searchByLID(client, query)
	case "all":
		// Buscar en todos los campos
		contacts = cc.searchByAll(client, query)
	}
	
	return contacts
}

// searchByPhone - Buscar por número de teléfono
func (cc *ContactController) searchByPhone(client *whatsmeow.Client, phone string) []*ContactInfo {
	// Limpiar número de teléfono
	cleanPhone := strings.ReplaceAll(phone, "+", "")
	cleanPhone = strings.ReplaceAll(cleanPhone, " ", "")
	cleanPhone = strings.ReplaceAll(cleanPhone, "-", "")
	
	// Crear JID desde el número
	jid := types.NewJID(cleanPhone, types.DefaultUserServer)
	
	// Obtener información del contacto
	contactInfo, err := cc.getDetailedContactInfo(client, jid)
	if err != nil {
		cc.logger.Warnf("Error obteniendo info de contacto %s: %v", jid, err)
		return []*ContactInfo{}
	}
	
	if contactInfo != nil {
		return []*ContactInfo{contactInfo}
	}
	
	return []*ContactInfo{}
}

// searchByName - Buscar por nombre (implementación básica)
func (cc *ContactController) searchByName(client *whatsmeow.Client, name string) []*ContactInfo {
	// Esta función requiere acceso al store de contactos local
	// Por ahora retorna lista vacía
	cc.logger.Infof("Búsqueda por nombre '%s' - funcionalidad por implementar completamente", name)
	return []*ContactInfo{}
}

// searchByJID - Buscar por JID específico
func (cc *ContactController) searchByJID(client *whatsmeow.Client, jidStr string) []*ContactInfo {
	jid, err := types.ParseJID(jidStr)
	if err != nil {
		cc.logger.Warnf("JID inválido para búsqueda: %s", jidStr)
		return []*ContactInfo{}
	}
	
	contactInfo, err := cc.getDetailedContactInfo(client, jid)
	if err != nil {
		cc.logger.Warnf("Error obteniendo info de contacto %s: %v", jid, err)
		return []*ContactInfo{}
	}
	
	if contactInfo != nil {
		return []*ContactInfo{contactInfo}
	}
	
	return []*ContactInfo{}
}

// searchByLID - Buscar por LID específico
func (cc *ContactController) searchByLID(client *whatsmeow.Client, lidStr string) []*ContactInfo {
	lidJID, err := types.ParseJID(lidStr)
	if err != nil || lidJID.Server != types.HiddenUserServer {
		cc.logger.Warnf("LID inválido para búsqueda: %s", lidStr)
		return []*ContactInfo{}
	}
	
	// Convertir LID a JID
	jid, err := client.Store.GetLIDMapping(context.Background(), lidJID, false)
	if err != nil {
		cc.logger.Warnf("Error convirtiendo LID a JID: %v", err)
		return []*ContactInfo{}
	}
	
	contactInfo, err := cc.getDetailedContactInfo(client, jid)
	if err != nil {
		cc.logger.Warnf("Error obteniendo info de contacto desde LID %s: %v", lidJID, err)
		return []*ContactInfo{}
	}
	
	if contactInfo != nil {
		return []*ContactInfo{contactInfo}
	}
	
	return []*ContactInfo{}
}

// searchByAll - Buscar en todos los campos
func (cc *ContactController) searchByAll(client *whatsmeow.Client, query string) []*ContactInfo {
	var allContacts []*ContactInfo
	
	// Intentar búsqueda por teléfono
	phoneResults := cc.searchByPhone(client, query)
	allContacts = append(allContacts, phoneResults...)
	
	// Intentar búsqueda por JID
	jidResults := cc.searchByJID(client, query)
	allContacts = append(allContacts, jidResults...)
	
	// Intentar búsqueda por LID
	lidResults := cc.searchByLID(client, query)
	allContacts = append(allContacts, lidResults...)
	
	// Remover duplicados
	return cc.removeDuplicateContacts(allContacts)
}

// getDetailedContactInfo - Obtener información detallada de un contacto
func (cc *ContactController) getDetailedContactInfo(client *whatsmeow.Client, jid types.JID) (*ContactInfo, error) {
	// Obtener información básica del usuario
	userInfoMap, err := client.GetUserInfo([]types.JID{jid})
	if err != nil {
		return nil, fmt.Errorf("error obteniendo info de usuario: %v", err)
	}
	
	userInfo, exists := userInfoMap[jid]
	if !exists {
		return nil, fmt.Errorf("usuario no encontrado")
	}
	
	// Crear estructura de contacto
	contact := &ContactInfo{
		JID:          jid.String(),
		Phone:        jid.User,
		Status:       userInfo.Status,
		PushName:     userInfo.PushName,
		IsOnWhatsApp: true,
		UpdatedAt:    time.Now(),
	}
	
	// Obtener LID si existe
	lidJID, err := client.Store.GetLIDMapping(context.Background(), jid, true)
	if err == nil && lidJID.Server == types.HiddenUserServer {
		contact.LID = lidJID.String()
	}
	
	// Obtener información del avatar
	if userInfo.PictureID != "" {
		contact.Avatar = &AvatarInfo{
			ID:   userInfo.PictureID,
			Type: "individual",
		}
	}
	
	// Procesar nombre verificado si existe
	if userInfo.VerifiedName != nil {
		contact.VerifiedName = &VerifiedNameInfo{
			Details: &VerifiedNameDetails{
				VerifiedName: userInfo.VerifiedName.Details.GetVerifiedName(),
				Issuer:       userInfo.VerifiedName.Details.GetIssuer(),
				Serial:       userInfo.VerifiedName.Details.GetSerial(),
				IssueTime:    userInfo.VerifiedName.Details.GetIssueTime(),
			},
		}
		
		// Procesar nombres localizados
		for _, localizedName := range userInfo.VerifiedName.Details.GetLocalizedNames() {
			contact.VerifiedName.Details.LocalizedNames = append(
				contact.VerifiedName.Details.LocalizedNames,
				LocalizedName{
					Locale: localizedName.GetLocale(),
					Name:   localizedName.GetName(),
				},
			)
		}
	}
	
	// Obtener lista de dispositivos
	devices, err := client.GetUserDevicesContext(context.Background(), []types.JID{jid})
	if err == nil {
		for _, device := range devices {
			contact.Devices = append(contact.Devices, device.String())
		}
	}
	
	return contact, nil
}

// resolveContactJID - Resolver JID de contacto desde diferentes identificadores
func (cc *ContactController) resolveContactJID(client *whatsmeow.Client, jidStr, lidStr, phone string) (types.JID, error) {
	// Prioridad: JID > LID > Phone
	if jidStr != "" {
		return types.ParseJID(jidStr)
	}
	
	if lidStr != "" {
		lidJID, err := types.ParseJID(lidStr)
		if err != nil {
			return types.JID{}, fmt.Errorf("LID inválido: %v", err)
		}
		
		// Convertir LID a JID
		jid, err := client.Store.GetLIDMapping(context.Background(), lidJID, false)
		if err != nil {
			return types.JID{}, fmt.Errorf("error convirtiendo LID a JID: %v", err)
		}
		
		return jid, nil
	}
	
	if phone != "" {
		// Limpiar y crear JID desde número
		cleanPhone := strings.ReplaceAll(phone, "+", "")
		cleanPhone = strings.ReplaceAll(cleanPhone, " ", "")
		cleanPhone = strings.ReplaceAll(cleanPhone, "-", "")
		
		return types.NewJID(cleanPhone, types.DefaultUserServer), nil
	}
	
	return types.JID{}, fmt.Errorf("debe proporcionar jid, lid o phone")
}

// parseAndCleanIdentifier - Parsear y limpiar identificador
func (cc *ContactController) parseAndCleanIdentifier(identifier string) (types.JID, error) {
	// Si ya es un JID válido, parsearlo directamente
	if strings.Contains(identifier, "@") {
		return types.ParseJID(identifier)
	}
	
	// Si es solo un número, crear JID
	cleanPhone := strings.ReplaceAll(identifier, "+", "")
	cleanPhone = strings.ReplaceAll(cleanPhone, " ", "")
	cleanPhone = strings.ReplaceAll(cleanPhone, "-", "")
	
	return types.NewJID(cleanPhone, types.DefaultUserServer), nil
}

// removeDuplicateContacts - Remover contactos duplicados
func (cc *ContactController) removeDuplicateContacts(contacts []*ContactInfo) []*ContactInfo {
	seen := make(map[string]bool)
	var unique []*ContactInfo
	
	for _, contact := range contacts {
		if !seen[contact.JID] {
			seen[contact.JID] = true
			unique = append(unique, contact)
		}
	}
	
	return unique
}
