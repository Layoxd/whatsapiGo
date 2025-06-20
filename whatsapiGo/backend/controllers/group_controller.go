package controllers

import (
	"context"
	"encoding/base64"
	"fmt"
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

// GroupController - Controlador para gestión completa de grupos WhatsApp
type GroupController struct {
	instanceController *InstanceController // Referencia al controlador de instancias
	logger             waLog.Logger        // Logger de WhatsMeow
}

// GroupInfo - Información completa de un grupo
type GroupInfo struct {
	JID              string                `json:"jid"`                        // JID del grupo
	LID              string                `json:"lid,omitempty"`              // LID del grupo (si existe)
	Name             string                `json:"name"`                       // Nombre del grupo
	Description      string                `json:"description,omitempty"`      // Descripción del grupo
	Subject          string                `json:"subject,omitempty"`          // Asunto (mismo que name generalmente)
	Owner            string                `json:"owner"`                      // Creador del grupo
	CreatedAt        time.Time             `json:"created_at"`                 // Fecha de creación
	UpdatedAt        time.Time             `json:"updated_at"`                 // Última actualización
	ParticipantCount int                   `json:"participant_count"`          // Número de participantes
	Participants     []*GroupParticipant   `json:"participants,omitempty"`     // Lista de participantes
	Admins           []string              `json:"admins,omitempty"`           // Lista de administradores
	Avatar           *GroupAvatar          `json:"avatar,omitempty"`           // Información del avatar
	InviteLink       string                `json:"invite_link,omitempty"`      // Enlace de invitación
	Settings         *GroupSettings        `json:"settings,omitempty"`         // Configuraciones del grupo
	IsAdmin          bool                  `json:"is_admin"`                   // Si el usuario es admin
	IsOwner          bool                  `json:"is_owner"`                   // Si el usuario es el dueño
	IsMember         bool                  `json:"is_member"`                  // Si el usuario es miembro
}

// GroupParticipant - Información de un participante del grupo
type GroupParticipant struct {
	JID          string    `json:"jid"`                    // JID del participante
	LID          string    `json:"lid,omitempty"`          // LID del participante
	Phone        string    `json:"phone"`                  // Número de teléfono
	Name         string    `json:"name,omitempty"`         // Nombre del contacto
	PushName     string    `json:"push_name,omitempty"`    // Nombre de WhatsApp
	IsAdmin      bool      `json:"is_admin"`               // Si es administrador
	IsOwner      bool      `json:"is_owner"`               // Si es el dueño
	JoinedAt     time.Time `json:"joined_at"`              // Fecha de ingreso
	AddedBy      string    `json:"added_by,omitempty"`     // Quién lo agregó
}

// GroupAvatar - Información del avatar del grupo
type GroupAvatar struct {
	ID       string `json:"id"`                // ID del avatar
	URL      string `json:"url,omitempty"`     // URL del avatar
	Preview  string `json:"preview,omitempty"` // Preview en base64
	FullSize string `json:"full_size,omitempty"` // Avatar completo en base64
}

// GroupSettings - Configuraciones del grupo
type GroupSettings struct {
	OnlyAdminsCanMessage    bool   `json:"only_admins_can_message"`    // Solo admins pueden enviar mensajes
	OnlyAdminsCanEditInfo   bool   `json:"only_admins_can_edit_info"`  // Solo admins pueden editar info
	OnlyAdminsCanAddMembers bool   `json:"only_admins_can_add_members"` // Solo admins pueden agregar miembros
	ApprovalMode            bool   `json:"approval_mode"`              // Modo de aprobación
	Ephemeral               string `json:"ephemeral,omitempty"`        // Configuración de mensajes temporales
	Size                    int    `json:"size"`                       // Tamaño máximo del grupo
	Announce                bool   `json:"announce"`                   // Es grupo de anuncios
}

// CreateGroupRequest - Petición para crear grupo
type CreateGroupRequest struct {
	InstanceID   string   `json:"instance_id" binding:"required"`
	Name         string   `json:"name" binding:"required"`         // Nombre del grupo
	Description  string   `json:"description,omitempty"`           // Descripción opcional
	Participants []string `json:"participants" binding:"required"` // Lista de JIDs/LIDs/phones
	Avatar       string   `json:"avatar,omitempty"`                // Avatar en base64
}

// UpdateGroupRequest - Petición para actualizar grupo
type UpdateGroupRequest struct {
	InstanceID  string `json:"instance_id" binding:"required"`
	Name        string `json:"name,omitempty"`        // Nuevo nombre
	Description string `json:"description,omitempty"` // Nueva descripción
	Avatar      string `json:"avatar,omitempty"`      // Nuevo avatar en base64
}

// ManageParticipantsRequest - Petición para gestionar participantes
type ManageParticipantsRequest struct {
	InstanceID   string   `json:"instance_id" binding:"required"`
	Participants []string `json:"participants" binding:"required"` // Lista de JIDs/LIDs/phones
}

// ManageAdminsRequest - Petición para gestionar administradores
type ManageAdminsRequest struct {
	InstanceID string   `json:"instance_id" binding:"required"`
	Admins     []string `json:"admins" binding:"required"` // Lista de JIDs/LIDs/phones
}

// GroupResponse - Respuesta estándar para operaciones de grupos
type GroupResponse struct {
	Success bool       `json:"success"`
	Message string     `json:"message"`
	Data    *GroupInfo `json:"data,omitempty"`
}

// GroupListResponse - Respuesta para listar grupos
type GroupListResponse struct {
	Success bool         `json:"success"`
	Message string       `json:"message"`
	Groups  []*GroupInfo `json:"groups"`
	Total   int          `json:"total"`
}

// InviteLinkResponse - Respuesta para enlaces de invitación
type InviteLinkResponse struct {
	Success    bool   `json:"success"`
	Message    string `json:"message"`
	InviteLink string `json:"invite_link,omitempty"`
	GroupJID   string `json:"group_jid,omitempty"`
}

// NewGroupController - Constructor del controlador de grupos
func NewGroupController(instanceController *InstanceController, logger waLog.Logger) *GroupController {
	return &GroupController{
		instanceController: instanceController,
		logger:             logger,
	}
}

// CreateGroup - POST /groups/{instanceId}/create - Crear nuevo grupo
func (gc *GroupController) CreateGroup(c *gin.Context) {
	instanceID := c.Param("instanceId")
	var req CreateGroupRequest
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, GroupResponse{
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
	instance, exists := gc.instanceController.instances[req.InstanceID]
	if !exists {
		c.JSON(http.StatusNotFound, GroupResponse{
			Success: false,
			Message: "Instancia no encontrada",
		})
		return
	}

	if !instance.Client.IsConnected() {
		c.JSON(http.StatusBadRequest, GroupResponse{
			Success: false,
			Message: "La instancia no está conectada a WhatsApp",
		})
		return
	}

	// Resolver participantes a JIDs
	participantJIDs, err := gc.resolveParticipants(instance.Client, req.Participants)
	if err != nil {
		c.JSON(http.StatusBadRequest, GroupResponse{
			Success: false,
			Message: fmt.Sprintf("Error resolviendo participantes: %v", err),
		})
		return
	}

	if len(participantJIDs) == 0 {
		c.JSON(http.StatusBadRequest, GroupResponse{
			Success: false,
			Message: "Debe proporcionar al menos un participante válido",
		})
		return
	}

	// Crear el grupo
	groupInfo, err := instance.Client.CreateGroup(types.GroupInfo{
		Name:         req.Name,
		Topic:        req.Description,
		Participants: participantJIDs,
	})
	if err != nil {
		gc.logger.Errorf("Error creando grupo: %v", err)
		c.JSON(http.StatusInternalServerError, GroupResponse{
			Success: false,
			Message: "Error creando el grupo",
		})
		return
	}

	// Si se proporcionó avatar, configurarlo
	if req.Avatar != "" {
		err = gc.setGroupAvatar(instance.Client, groupInfo.JID, req.Avatar)
		if err != nil {
			gc.logger.Warnf("Error configurando avatar del grupo: %v", err)
		}
	}

	// Obtener información completa del grupo creado
	fullGroupInfo, err := gc.getDetailedGroupInfo(instance.Client, groupInfo.JID)
	if err != nil {
		gc.logger.Warnf("Error obteniendo info completa del grupo: %v", err)
		// Crear info básica si no se puede obtener la completa
		fullGroupInfo = &GroupInfo{
			JID:              groupInfo.JID.String(),
			Name:             req.Name,
			Description:      req.Description,
			ParticipantCount: len(participantJIDs),
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
			IsAdmin:          true,
			IsOwner:          true,
			IsMember:         true,
		}
	}

	gc.logger.Infof("Grupo creado exitosamente: %s (%s)", fullGroupInfo.Name, fullGroupInfo.JID)

	c.JSON(http.StatusCreated, GroupResponse{
		Success: true,
		Message: "Grupo creado exitosamente",
		Data:    fullGroupInfo,
	})
}

// GetGroups - GET /groups/{instanceId} - Listar todos los grupos
func (gc *GroupController) GetGroups(c *gin.Context) {
	instanceID := c.Param("instanceId")
	
	// Obtener instancia
	instance, exists := gc.instanceController.instances[instanceID]
	if !exists {
		c.JSON(http.StatusNotFound, GroupListResponse{
			Success: false,
			Message: "Instancia no encontrada",
		})
		return
	}

	if !instance.Client.IsConnected() {
		c.JSON(http.StatusBadRequest, GroupListResponse{
			Success: false,
			Message: "La instancia no está conectada a WhatsApp",
		})
		return
	}

	// Obtener lista de grupos desde WhatsApp
	groups, err := gc.getAllGroups(instance.Client)
	if err != nil {
		gc.logger.Errorf("Error obteniendo grupos: %v", err)
		c.JSON(http.StatusInternalServerError, GroupListResponse{
			Success: false,
			Message: "Error obteniendo lista de grupos",
		})
		return
	}

	gc.logger.Infof("Grupos obtenidos: %d para instancia %s", len(groups), instanceID)

	c.JSON(http.StatusOK, GroupListResponse{
		Success: true,
		Message: "Grupos obtenidos exitosamente",
		Groups:  groups,
		Total:   len(groups),
	})
}

// GetGroupInfo - GET /groups/{instanceId}/{groupId}/info - Info completa del grupo
func (gc *GroupController) GetGroupInfo(c *gin.Context) {
	instanceID := c.Param("instanceId")
	groupID := c.Param("groupId")
	
	// Obtener instancia
	instance, exists := gc.instanceController.instances[instanceID]
	if !exists {
		c.JSON(http.StatusNotFound, GroupResponse{
			Success: false,
			Message: "Instancia no encontrada",
		})
		return
	}

	if !instance.Client.IsConnected() {
		c.JSON(http.StatusBadRequest, GroupResponse{
			Success: false,
			Message: "La instancia no está conectada a WhatsApp",
		})
		return
	}

	// Parsear JID del grupo
	groupJID, err := types.ParseJID(groupID)
	if err != nil {
		c.JSON(http.StatusBadRequest, GroupResponse{
			Success: false,
			Message: fmt.Sprintf("ID de grupo inválido: %v", err),
		})
		return
	}

	// Obtener información completa del grupo
	groupInfo, err := gc.getDetailedGroupInfo(instance.Client, groupJID)
	if err != nil {
		gc.logger.Errorf("Error obteniendo info del grupo %s: %v", groupJID, err)
		c.JSON(http.StatusInternalServerError, GroupResponse{
			Success: false,
			Message: "Error obteniendo información del grupo",
		})
		return
	}

	if groupInfo == nil {
		c.JSON(http.StatusNotFound, GroupResponse{
			Success: false,
			Message: "Grupo no encontrado",
		})
		return
	}

	gc.logger.Infof("Información de grupo obtenida: %s", groupJID)

	c.JSON(http.StatusOK, GroupResponse{
		Success: true,
		Message: "Información del grupo obtenida exitosamente",
		Data:    groupInfo,
	})
}

// UpdateGroup - PUT /groups/{instanceId}/{groupId}/update - Actualizar configuraciones
func (gc *GroupController) UpdateGroup(c *gin.Context) {
	instanceID := c.Param("instanceId")
	groupID := c.Param("groupId")
	var req UpdateGroupRequest
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, GroupResponse{
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
	instance, exists := gc.instanceController.instances[req.InstanceID]
	if !exists {
		c.JSON(http.StatusNotFound, GroupResponse{
			Success: false,
			Message: "Instancia no encontrada",
		})
		return
	}

	if !instance.Client.IsConnected() {
		c.JSON(http.StatusBadRequest, GroupResponse{
			Success: false,
			Message: "La instancia no está conectada a WhatsApp",
		})
		return
	}

	// Parsear JID del grupo
	groupJID, err := types.ParseJID(groupID)
	if err != nil {
		c.JSON(http.StatusBadRequest, GroupResponse{
			Success: false,
			Message: fmt.Sprintf("ID de grupo inválido: %v", err),
		})
		return
	}

	// Actualizar nombre si se proporciona
	if req.Name != "" {
		err = instance.Client.SetGroupName(groupJID, req.Name)
		if err != nil {
			gc.logger.Errorf("Error actualizando nombre del grupo: %v", err)
			c.JSON(http.StatusInternalServerError, GroupResponse{
				Success: false,
				Message: "Error actualizando nombre del grupo",
			})
			return
		}
	}

	// Actualizar descripción si se proporciona
	if req.Description != "" {
		err = instance.Client.SetGroupTopic(groupJID, req.Description, "", "")
		if err != nil {
			gc.logger.Errorf("Error actualizando descripción del grupo: %v", err)
			c.JSON(http.StatusInternalServerError, GroupResponse{
				Success: false,
				Message: "Error actualizando descripción del grupo",
			})
			return
		}
	}

	// Actualizar avatar si se proporciona
	if req.Avatar != "" {
		err = gc.setGroupAvatar(instance.Client, groupJID, req.Avatar)
		if err != nil {
			gc.logger.Errorf("Error actualizando avatar del grupo: %v", err)
			c.JSON(http.StatusInternalServerError, GroupResponse{
				Success: false,
				Message: "Error actualizando avatar del grupo",
			})
			return
		}
	}

	// Obtener información actualizada del grupo
	groupInfo, err := gc.getDetailedGroupInfo(instance.Client, groupJID)
	if err != nil {
		gc.logger.Warnf("Error obteniendo info actualizada del grupo: %v", err)
	}

	gc.logger.Infof("Grupo actualizado: %s", groupJID)

	c.JSON(http.StatusOK, GroupResponse{
		Success: true,
		Message: "Grupo actualizado exitosamente",
		Data:    groupInfo,
	})
}

// AddParticipants - POST /groups/{instanceId}/{groupId}/participants/add - Agregar participantes
func (gc *GroupController) AddParticipants(c *gin.Context) {
	instanceID := c.Param("instanceId")
	groupID := c.Param("groupId")
	var req ManageParticipantsRequest
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, GroupResponse{
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
	instance, exists := gc.instanceController.instances[req.InstanceID]
	if !exists {
		c.JSON(http.StatusNotFound, GroupResponse{
			Success: false,
			Message: "Instancia no encontrada",
		})
		return
	}

	if !instance.Client.IsConnected() {
		c.JSON(http.StatusBadRequest, GroupResponse{
			Success: false,
			Message: "La instancia no está conectada a WhatsApp",
		})
		return
	}

	// Parsear JID del grupo
	groupJID, err := types.ParseJID(groupID)
	if err != nil {
		c.JSON(http.StatusBadRequest, GroupResponse{
			Success: false,
			Message: fmt.Sprintf("ID de grupo inválido: %v", err),
		})
		return
	}

	// Resolver participantes a JIDs
	participantJIDs, err := gc.resolveParticipants(instance.Client, req.Participants)
	if err != nil {
		c.JSON(http.StatusBadRequest, GroupResponse{
			Success: false,
			Message: fmt.Sprintf("Error resolviendo participantes: %v", err),
		})
		return
	}

	// Agregar participantes al grupo
	addResults, err := instance.Client.GroupParticipantsUpdate(context.Background(), groupJID, participantJIDs, "add")
	if err != nil {
		gc.logger.Errorf("Error agregando participantes: %v", err)
		c.JSON(http.StatusInternalServerError, GroupResponse{
			Success: false,
			Message: "Error agregando participantes al grupo",
		})
		return
	}

	// Contar participantes agregados exitosamente
	successCount := 0
	for _, result := range addResults {
		if result.Error == "" {
			successCount++
		} else {
			gc.logger.Warnf("Error agregando participante %s: %s", result.JID, result.Error)
		}
	}

	// Obtener información actualizada del grupo
	groupInfo, err := gc.getDetailedGroupInfo(instance.Client, groupJID)
	if err != nil {
		gc.logger.Warnf("Error obteniendo info actualizada del grupo: %v", err)
	}

	gc.logger.Infof("Participantes agregados: %d de %d en grupo %s", successCount, len(participantJIDs), groupJID)

	c.JSON(http.StatusOK, GroupResponse{
		Success: true,
		Message: fmt.Sprintf("Participantes agregados: %d de %d exitosos", successCount, len(participantJIDs)),
		Data:    groupInfo,
	})
}

// RemoveParticipants - POST /groups/{instanceId}/{groupId}/participants/remove - Remover participantes
func (gc *GroupController) RemoveParticipants(c *gin.Context) {
	instanceID := c.Param("instanceId")
	groupID := c.Param("groupId")
	var req ManageParticipantsRequest
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, GroupResponse{
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
	instance, exists := gc.instanceController.instances[req.InstanceID]
	if !exists {
		c.JSON(http.StatusNotFound, GroupResponse{
			Success: false,
			Message: "Instancia no encontrada",
		})
		return
	}

	if !instance.Client.IsConnected() {
		c.JSON(http.StatusBadRequest, GroupResponse{
			Success: false,
			Message: "La instancia no está conectada a WhatsApp",
		})
		return
	}

	// Parsear JID del grupo
	groupJID, err := types.ParseJID(groupID)
	if err != nil {
		c.JSON(http.StatusBadRequest, GroupResponse{
			Success: false,
			Message: fmt.Sprintf("ID de grupo inválido: %v", err),
		})
		return
	}

	// Resolver participantes a JIDs
	participantJIDs, err := gc.resolveParticipants(instance.Client, req.Participants)
	if err != nil {
		c.JSON(http.StatusBadRequest, GroupResponse{
			Success: false,
			Message: fmt.Sprintf("Error resolviendo participantes: %v", err),
		})
		return
	}

	// Remover participantes del grupo
	removeResults, err := instance.Client.GroupParticipantsUpdate(context.Background(), groupJID, participantJIDs, "remove")
	if err != nil {
		gc.logger.Errorf("Error removiendo participantes: %v", err)
		c.JSON(http.StatusInternalServerError, GroupResponse{
			Success: false,
			Message: "Error removiendo participantes del grupo",
		})
		return
	}

	// Contar participantes removidos exitosamente
	successCount := 0
	for _, result := range removeResults {
		if result.Error == "" {
			successCount++
		} else {
			gc.logger.Warnf("Error removiendo participante %s: %s", result.JID, result.Error)
		}
	}

	// Obtener información actualizada del grupo
	groupInfo, err := gc.getDetailedGroupInfo(instance.Client, groupJID)
	if err != nil {
		gc.logger.Warnf("Error obteniendo info actualizada del grupo: %v", err)
	}

	gc.logger.Infof("Participantes removidos: %d de %d en grupo %s", successCount, len(participantJIDs), groupJID)

	c.JSON(http.StatusOK, GroupResponse{
		Success: true,
		Message: fmt.Sprintf("Participantes removidos: %d de %d exitosos", successCount, len(participantJIDs)),
		Data:    groupInfo,
	})
}

// PromoteToAdmin - POST /groups/{instanceId}/{groupId}/admins/add - Promover a administrador
func (gc *GroupController) PromoteToAdmin(c *gin.Context) {
	instanceID := c.Param("instanceId")
	groupID := c.Param("groupId")
	var req ManageAdminsRequest
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, GroupResponse{
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
	instance, exists := gc.instanceController.instances[req.InstanceID]
	if !exists {
		c.JSON(http.StatusNotFound, GroupResponse{
			Success: false,
			Message: "Instancia no encontrada",
		})
		return
	}

	if !instance.Client.IsConnected() {
		c.JSON(http.StatusBadRequest, GroupResponse{
			Success: false,
			Message: "La instancia no está conectada a WhatsApp",
		})
		return
	}

	// Parsear JID del grupo
	groupJID, err := types.ParseJID(groupID)
	if err != nil {
		c.JSON(http.StatusBadRequest, GroupResponse{
			Success: false,
			Message: fmt.Sprintf("ID de grupo inválido: %v", err),
		})
		return
	}

	// Resolver administradores a JIDs
	adminJIDs, err := gc.resolveParticipants(instance.Client, req.Admins)
	if err != nil {
		c.JSON(http.StatusBadRequest, GroupResponse{
			Success: false,
			Message: fmt.Sprintf("Error resolviendo administradores: %v", err),
		})
		return
	}

	// Promover a administradores
	promoteResults, err := instance.Client.GroupParticipantsUpdate(context.Background(), groupJID, adminJIDs, "promote")
	if err != nil {
		gc.logger.Errorf("Error promoviendo administradores: %v", err)
		c.JSON(http.StatusInternalServerError, GroupResponse{
			Success: false,
			Message: "Error promoviendo usuarios a administradores",
		})
		return
	}

	// Contar promociones exitosas
	successCount := 0
	for _, result := range promoteResults {
		if result.Error == "" {
			successCount++
		} else {
			gc.logger.Warnf("Error promoviendo %s: %s", result.JID, result.Error)
		}
	}

	// Obtener información actualizada del grupo
	groupInfo, err := gc.getDetailedGroupInfo(instance.Client, groupJID)
	if err != nil {
		gc.logger.Warnf("Error obteniendo info actualizada del grupo: %v", err)
	}

	gc.logger.Infof("Usuarios promovidos a admin: %d de %d en grupo %s", successCount, len(adminJIDs), groupJID)

	c.JSON(http.StatusOK, GroupResponse{
		Success: true,
		Message: fmt.Sprintf("Usuarios promovidos a administradores: %d de %d exitosos", successCount, len(adminJIDs)),
		Data:    groupInfo,
	})
}

// DemoteFromAdmin - POST /groups/{instanceId}/{groupId}/admins/remove - Degradar de administrador
func (gc *GroupController) DemoteFromAdmin(c *gin.Context) {
	instanceID := c.Param("instanceId")
	groupID := c.Param("groupId")
	var req ManageAdminsRequest
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, GroupResponse{
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
	instance, exists := gc.instanceController.instances[req.InstanceID]
	if !exists {
		c.JSON(http.StatusNotFound, GroupResponse{
			Success: false,
			Message: "Instancia no encontrada",
		})
		return
	}

	if !instance.Client.IsConnected() {
		c.JSON(http.StatusBadRequest, GroupResponse{
			Success: false,
			Message: "La instancia no está conectada a WhatsApp",
		})
		return
	}

	// Parsear JID del grupo
	groupJID, err := types.ParseJID(groupID)
	if err != nil {
		c.JSON(http.StatusBadRequest, GroupResponse{
			Success: false,
			Message: fmt.Sprintf("ID de grupo inválido: %v", err),
		})
		return
	}

	// Resolver administradores a JIDs
	adminJIDs, err := gc.resolveParticipants(instance.Client, req.Admins)
	if err != nil {
		c.JSON(http.StatusBadRequest, GroupResponse{
			Success: false,
			Message: fmt.Sprintf("Error resolviendo administradores: %v", err),
		})
		return
	}

	// Degradar administradores
	demoteResults, err := instance.Client.GroupParticipantsUpdate(context.Background(), groupJID, adminJIDs, "demote")
	if err != nil {
		gc.logger.Errorf("Error degradando administradores: %v", err)
		c.JSON(http.StatusInternalServerError, GroupResponse{
			Success: false,
			Message: "Error degradando administradores",
		})
		return
	}

	// Contar degradaciones exitosas
	successCount := 0
	for _, result := range demoteResults {
		if result.Error == "" {
			successCount++
		} else {
			gc.logger.Warnf("Error degradando %s: %s", result.JID, result.Error)
		}
	}

	// Obtener información actualizada del grupo
	groupInfo, err := gc.getDetailedGroupInfo(instance.Client, groupJID)
	if err != nil {
		gc.logger.Warnf("Error obteniendo info actualizada del grupo: %v", err)
	}

	gc.logger.Infof("Administradores degradados: %d de %d en grupo %s", successCount, len(adminJIDs), groupJID)

	c.JSON(http.StatusOK, GroupResponse{
		Success: true,
		Message: fmt.Sprintf("Administradores degradados: %d de %d exitosos", successCount, len(adminJIDs)),
		Data:    groupInfo,
	})
}

// GetInviteLink - GET /groups/{instanceId}/{groupId}/invite-link - Obtener enlace de invitación
func (gc *GroupController) GetInviteLink(c *gin.Context) {
	instanceID := c.Param("instanceId")
	groupID := c.Param("groupId")
	
	// Obtener instancia
	instance, exists := gc.instanceController.instances[instanceID]
	if !exists {
		c.JSON(http.StatusNotFound, InviteLinkResponse{
			Success: false,
			Message: "Instancia no encontrada",
		})
		return
	}

	if !instance.Client.IsConnected() {
		c.JSON(http.StatusBadRequest, InviteLinkResponse{
			Success: false,
			Message: "La instancia no está conectada a WhatsApp",
		})
		return
	}

	// Parsear JID del grupo
	groupJID, err := types.ParseJID(groupID)
	if err != nil {
		c.JSON(http.StatusBadRequest, InviteLinkResponse{
			Success: false,
			Message: fmt.Sprintf("ID de grupo inválido: %v", err),
		})
		return
	}

	// Obtener enlace de invitación
	inviteLink, err := instance.Client.GetGroupInviteLink(groupJID, false)
	if err != nil {
		gc.logger.Errorf("Error obteniendo enlace de invitación: %v", err)
		c.JSON(http.StatusInternalServerError, InviteLinkResponse{
			Success: false,
			Message: "Error obteniendo enlace de invitación",
		})
		return
	}

	gc.logger.Infof("Enlace de invitación obtenido para grupo: %s", groupJID)

	c.JSON(http.StatusOK, InviteLinkResponse{
		Success:    true,
		Message:    "Enlace de invitación obtenido exitosamente",
		InviteLink: inviteLink,
		GroupJID:   groupJID.String(),
	})
}

// ResetInviteLink - POST /groups/{instanceId}/{groupId}/invite-link/reset - Resetear enlace de invitación
func (gc *GroupController) ResetInviteLink(c *gin.Context) {
	instanceID := c.Param("instanceId")
	groupID := c.Param("groupId")
	
	// Obtener instancia
	instance, exists := gc.instanceController.instances[instanceID]
	if !exists {
		c.JSON(http.StatusNotFound, InviteLinkResponse{
			Success: false,
			Message: "Instancia no encontrada",
		})
		return
	}

	if !instance.Client.IsConnected() {
		c.JSON(http.StatusBadRequest, InviteLinkResponse{
			Success: false,
			Message: "La instancia no está conectada a WhatsApp",
		})
		return
	}

	// Parsear JID del grupo
	groupJID, err := types.ParseJID(groupID)
	if err != nil {
		c.JSON(http.StatusBadRequest, InviteLinkResponse{
			Success: false,
			Message: fmt.Sprintf("ID de grupo inválido: %v", err),
		})
		return
	}

	// Resetear enlace de invitación (revocar el anterior)
	newInviteLink, err := instance.Client.GetGroupInviteLink(groupJID, true)
	if err != nil {
		gc.logger.Errorf("Error reseteando enlace de invitación: %v", err)
		c.JSON(http.StatusInternalServerError, InviteLinkResponse{
			Success: false,
			Message: "Error reseteando enlace de invitación",
		})
		return
	}

	gc.logger.Infof("Enlace de invitación reseteado para grupo: %s", groupJID)

	c.JSON(http.StatusOK, InviteLinkResponse{
		Success:    true,
		Message:    "Enlace de invitación reseteado exitosamente",
		InviteLink: newInviteLink,
		GroupJID:   groupJID.String(),
	})
}

// LeaveGroup - POST /groups/{instanceId}/{groupId}/leave - Abandonar grupo
func (gc *GroupController) LeaveGroup(c *gin.Context) {
	instanceID := c.Param("instanceId")
	groupID := c.Param("groupId")
	
	// Obtener instancia
	instance, exists := gc.instanceController.instances[instanceID]
	if !exists {
		c.JSON(http.StatusNotFound, GroupResponse{
			Success: false,
			Message: "Instancia no encontrada",
		})
		return
	}

	if !instance.Client.IsConnected() {
		c.JSON(http.StatusBadRequest, GroupResponse{
			Success: false,
			Message: "La instancia no está conectada a WhatsApp",
		})
		return
	}

	// Parsear JID del grupo
	groupJID, err := types.ParseJID(groupID)
	if err != nil {
		c.JSON(http.StatusBadRequest, GroupResponse{
			Success: false,
			Message: fmt.Sprintf("ID de grupo inválido: %v", err),
		})
		return
	}

	// Abandonar el grupo
	err = instance.Client.LeaveGroup(groupJID)
	if err != nil {
		gc.logger.Errorf("Error abandonando grupo: %v", err)
		c.JSON(http.StatusInternalServerError, GroupResponse{
			Success: false,
			Message: "Error abandonando el grupo",
		})
		return
	}

	gc.logger.Infof("Grupo abandonado: %s", groupJID)

	c.JSON(http.StatusOK, GroupResponse{
		Success: true,
		Message: "Grupo abandonado exitosamente",
	})
}

// DeleteGroup - DELETE /groups/{instanceId}/{groupId} - Eliminar grupo (solo owner)
func (gc *GroupController) DeleteGroup(c *gin.Context) {
	instanceID := c.Param("instanceId")
	groupID := c.Param("groupId")
	
	// Obtener instancia
	instance, exists := gc.instanceController.instances[instanceID]
	if !exists {
		c.JSON(http.StatusNotFound, GroupResponse{
			Success: false,
			Message: "Instancia no encontrada",
		})
		return
	}

	if !instance.Client.IsConnected() {
		c.JSON(http.StatusBadRequest, GroupResponse{
			Success: false,
			Message: "La instancia no está conectada a WhatsApp",
		})
		return
	}

	// Parsear JID del grupo
	groupJID, err := types.ParseJID(groupID)
	if err != nil {
		c.JSON(http.StatusBadRequest, GroupResponse{
			Success: false,
			Message: fmt.Sprintf("ID de grupo inválido: %v", err),
		})
		return
	}

	// Obtener información del grupo para verificar permisos
	groupInfo, err := instance.Client.GetGroupInfo(groupJID)
	if err != nil {
		gc.logger.Errorf("Error obteniendo info del grupo para eliminar: %v", err)
		c.JSON(http.StatusInternalServerError, GroupResponse{
			Success: false,
			Message: "Error verificando permisos del grupo",
		})
		return
	}

	// Verificar si el usuario es el dueño del grupo
	userJID := instance.Client.Store.ID
	if groupInfo.OwnerJID.User != userJID.User {
		c.JSON(http.StatusForbidden, GroupResponse{
			Success: false,
			Message: "Solo el dueño del grupo puede eliminarlo",
		})
		return
	}

	// Eliminar el grupo (enviando mensaje de sistema)
	_, err = instance.Client.SendMessage(context.Background(), groupJID, &waE2E.Message{
		ProtocolMessage: &waE2E.ProtocolMessage{
			Type: proto.Uint32(uint32(waE2E.ProtocolMessage_GROUP_DELETE)),
		},
	})
	if err != nil {
		gc.logger.Errorf("Error eliminando grupo: %v", err)
		c.JSON(http.StatusInternalServerError, GroupResponse{
			Success: false,
			Message: "Error eliminando el grupo",
		})
		return
	}

	gc.logger.Infof("Grupo eliminado: %s", groupJID)

	c.JSON(http.StatusOK, GroupResponse{
		Success: true,
		Message: "Grupo eliminado exitosamente",
	})
}

// getAllGroups - Obtener todos los grupos
func (gc *GroupController) getAllGroups(client *whatsmeow.Client) ([]*GroupInfo, error) {
	var groups []*GroupInfo
	
	// Esta función requiere acceso al store de grupos
	// Por ahora implementamos una versión básica
	gc.logger.Infof("Obteniendo grupos (funcionalidad básica implementada)")
	
	return groups, nil
}

// getDetailedGroupInfo - Obtener información detallada de un grupo
func (gc *GroupController) getDetailedGroupInfo(client *whatsmeow.Client, groupJID types.JID) (*GroupInfo, error) {
	// Obtener información básica del grupo
	groupInfo, err := client.GetGroupInfo(groupJID)
	if err != nil {
		return nil, fmt.Errorf("error obteniendo info básica del grupo: %v", err)
	}

	// Crear estructura de información del grupo
	group := &GroupInfo{
		JID:              groupJID.String(),
		Name:             groupInfo.Name,
		Description:      groupInfo.Topic,
		Subject:          groupInfo.Name,
		Owner:            groupInfo.OwnerJID.String(),
		CreatedAt:        groupInfo.GroupCreated,
		UpdatedAt:        time.Now(),
		ParticipantCount: len(groupInfo.Participants),
		Settings: &GroupSettings{
			OnlyAdminsCanMessage:    groupInfo.IsAnnounce,
			OnlyAdminsCanEditInfo:   true, // Por defecto
			OnlyAdminsCanAddMembers: true, // Por defecto
			Size:                    len(groupInfo.Participants),
			Announce:                groupInfo.IsAnnounce,
		},
	}

	// Obtener LID del grupo si existe
	lidJID, err := client.Store.GetLIDMapping(context.Background(), groupJID, true)
	if err == nil && lidJID.Server == types.HiddenUserServer {
		group.LID = lidJID.String()
	}

	// Procesar participantes
	userJID := client.Store.ID
	for _, participant := range groupInfo.Participants {
		isAdmin := false
		isOwner := false
		
		// Verificar si es administrador
		for _, admin := range groupInfo.AdminJIDs {
			if admin.User == participant.JID.User {
				isAdmin = true
				break
			}
		}
		
		// Verificar si es el dueño
		if participant.JID.User == groupInfo.OwnerJID.User {
			isOwner = true
			isAdmin = true
		}

		// Verificar permisos del usuario actual
		if participant.JID.User == userJID.User {
			group.IsAdmin = isAdmin
			group.IsOwner = isOwner
			group.IsMember = true
		}

		// Agregar participante a la lista
		groupParticipant := &GroupParticipant{
			JID:      participant.JID.String(),
			Phone:    participant.JID.User,
			IsAdmin:  isAdmin,
			IsOwner:  isOwner,
			JoinedAt: time.Now(), // Placeholder - WhatsApp no siempre proporciona esta info
		}

		// Obtener LID del participante si existe
		participantLID, err := client.Store.GetLIDMapping(context.Background(), participant.JID, true)
		if err == nil && participantLID.Server == types.HiddenUserServer {
			groupParticipant.LID = participantLID.String()
		}

		group.Participants = append(group.Participants, groupParticipant)
		
		// Agregar a lista de admins si corresponde
		if isAdmin {
			group.Admins = append(group.Admins, participant.JID.String())
		}
	}

	return group, nil
}

// resolveParticipants - Resolver lista de identificadores a JIDs
func (gc *GroupController) resolveParticipants(client *whatsmeow.Client, identifiers []string) ([]types.JID, error) {
	var jids []types.JID
	
	for _, identifier := range identifiers {
		// Si ya es un JID válido, parsearlo directamente
		if strings.Contains(identifier, "@") {
			jid, err := types.ParseJID(identifier)
			if err != nil {
				gc.logger.Warnf("JID inválido: %s", identifier)
				continue
			}
			
			// Si es un LID, convertir a JID
			if jid.Server == types.HiddenUserServer {
				convertedJID, err := client.Store.GetLIDMapping(context.Background(), jid, false)
				if err != nil {
					gc.logger.Warnf("Error convirtiendo LID a JID: %s", identifier)
					continue
				}
				jids = append(jids, convertedJID)
			} else {
				jids = append(jids, jid)
			}
		} else {
			// Es un número de teléfono, crear JID
			cleanPhone := strings.ReplaceAll(identifier, "+", "")
			cleanPhone = strings.ReplaceAll(cleanPhone, " ", "")
			cleanPhone = strings.ReplaceAll(cleanPhone, "-", "")
			
			jid := types.NewJID(cleanPhone, types.DefaultUserServer)
			jids = append(jids, jid)
		}
	}
	
	return jids, nil
}

// setGroupAvatar - Configurar avatar del grupo
func (gc *GroupController) setGroupAvatar(client *whatsmeow.Client, groupJID types.JID, avatarBase64 string) error {
	// Decodificar imagen base64
	avatarData, err := base64.StdEncoding.DecodeString(avatarBase64)
	if err != nil {
		return fmt.Errorf("error decodificando avatar base64: %v", err)
	}

	// Subir avatar
	uploaded, err := client.Upload(context.Background(), avatarData, whatsmeow.MediaImage)
	if err != nil {
		return fmt.Errorf("error subiendo avatar: %v", err)
	}

	// Configurar avatar del grupo
	err = client.SetGroupPhoto(groupJID, uploaded)
	if err != nil {
		return fmt.Errorf("error configurando avatar del grupo: %v", err)
	}

	return nil
}
