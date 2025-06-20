package routes

import (
    "github.com/gin-gonic/gin"
    "github.com/Layoxd/whatsapiGo/controllers"
    "github.com/Layoxd/whatsapiGo/services"  // ← NUEVO IMPORT
)

// SetupRoutes - Configurar todas las rutas de la API
// ← PARÁMETROS ACTUALIZADOS
func SetupRoutes(
    instanceController *controllers.InstanceController,
    messageController *controllers.MessageController,
    contactController *controllers.ContactController,
    groupController *controllers.GroupController,
    statusController *controllers.StatusController,
    webhookController *controllers.WebhookController,  // ← NUEVO PARÁMETRO
    webhookService *services.WebhookService,           // ← NUEVO PARÁMETRO
) *gin.Engine {
    router := gin.Default()
    
    // Middleware CORS
    router.Use(func(c *gin.Context) {
        c.Header("Access-Control-Allow-Origin", "*")
        c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
        c.Header("Access-Control-Allow-Headers", "Content-Type,Authorization,X-API-Key")
        
        if c.Request.Method == "OPTIONS" {
            c.AbortWithStatus(204)
            return
        }
        c.Next()
    })

    // Health check actualizado
    router.GET("/health", func(c *gin.Context) {
        c.JSON(200, gin.H{
            "status":  "ok", 
            "service": "WhatsApp API Go",
            "version": "1.0.0",                           // ← NUEVO
            "features": []string{                         // ← NUEVO
                "webhooks",
                "call_management", 
                "multimedia_status",
                "lid_support",
            },
        })
    })

    // Grupo de rutas API v1
    v1 := router.Group("/api/v1")
    {
        // Rutas de instancias (existentes)
        instances := v1.Group("/instances")
        {
            instances.POST("", instanceController.CreateInstance)
            instances.GET("", instanceController.GetInstances)
            instances.GET("/:id", instanceController.GetInstance)
            instances.DELETE("/:id", instanceController.DeleteInstance)
            instances.GET("/:id/qr", instanceController.GetQRCode)
            instances.POST("/:id/logout", instanceController.LogoutInstance)
        }

        // Rutas de mensajes (existentes)
        messages := v1.Group("/messages")
        {
            messages.POST("/text", messageController.SendTextMessage)
            messages.POST("/image", messageController.SendImageMessage)
            messages.POST("/video", messageController.SendVideoMessage)
            messages.POST("/audio", messageController.SendAudioMessage)
            messages.POST("/document", messageController.SendDocumentMessage)
            messages.POST("/location", messageController.SendLocationMessage)
            messages.POST("/contact", messageController.SendContactMessage)
        }

        // Rutas de contactos (existentes)
        contacts := v1.Group("/contacts")
        {
            contacts.GET("/:instanceId", contactController.GetContacts)
            contacts.GET("/:instanceId/search", contactController.SearchContacts)
            contacts.GET("/:instanceId/info/:jid", contactController.GetContactInfo)
            contacts.POST("/:instanceId/check", contactController.CheckContacts)
            contacts.POST("/:instanceId/block", contactController.BlockContact)
            contacts.POST("/:instanceId/unblock", contactController.UnblockContact)
            contacts.GET("/:instanceId/lid/get", contactController.GetLIDFromJID)
            contacts.GET("/:instanceId/lid/from-lid", contactController.GetJIDFromLID)
        }

        // Rutas de grupos (existentes)
        groups := v1.Group("/groups")
        {
            groups.POST("/:instanceId/create", groupController.CreateGroup)
            groups.DELETE("/:instanceId/:groupId", groupController.DeleteGroup)
            groups.GET("/:instanceId", groupController.GetGroups)
            groups.GET("/:instanceId/:groupId/info", groupController.GetGroupInfo)
            groups.PUT("/:instanceId/:groupId/update", groupController.UpdateGroup)
            groups.POST("/:instanceId/:groupId/participants/add", groupController.AddParticipants)
            groups.POST("/:instanceId/:groupId/participants/remove", groupController.RemoveParticipants)
            groups.POST("/:instanceId/:groupId/admins/add", groupController.PromoteToAdmin)
            groups.POST("/:instanceId/:groupId/admins/remove", groupController.DemoteFromAdmin)
            groups.GET("/:instanceId/:groupId/invite-link", groupController.GetInviteLink)
            groups.POST("/:instanceId/:groupId/invite-link/reset", groupController.ResetInviteLink)
            groups.POST("/:instanceId/:groupId/leave", groupController.LeaveGroup)
        }

        // Rutas de estados/stories (existentes)
        status := v1.Group("/status")
        {
            status.POST("/:instanceId/publish", statusController.PublishStatus)
            status.GET("/:instanceId", statusController.GetOwnStatuses)
            status.GET("/:instanceId/contacts", statusController.GetContactStatuses)
            status.GET("/:instanceId/contact/:jid", statusController.GetContactStatus)
            status.DELETE("/:instanceId/:statusId", statusController.DeleteStatus)
            status.GET("/:instanceId/:statusId/viewers", statusController.GetStatusViewers)
            
            // Configuraciones de privacidad
            status.POST("/:instanceId/privacy", statusController.UpdateStatusPrivacy)
            status.GET("/:instanceId/privacy", statusController.GetStatusPrivacy)
        }

        // ===== NUEVAS RUTAS DE WEBHOOKS =====
        webhooks := v1.Group("/webhooks")
        {
            // Gestión de webhooks
            webhooks.POST("/:instanceId/configure", webhookController.ConfigureWebhook)
            webhooks.POST("/:instanceId/add", webhookController.AddWebhook)
            webhooks.GET("/:instanceId", webhookController.ListWebhooks)
            webhooks.PUT("/:instanceId/:webhookId", webhookController.UpdateWebhook)
            webhooks.DELETE("/:instanceId/:webhookId", webhookController.DeleteWebhook)
            
            // Testing y métricas
            webhooks.POST("/:instanceId/test", webhookController.TestWebhook)
            webhooks.GET("/:instanceId/metrics", webhookController.GetMetrics)
            
            // Logs y reintentos
            webhooks.POST("/:instanceId/retry/:eventId", webhookController.RetryEvent)
            webhooks.GET("/:instanceId/logs", webhookController.GetLogs)
            
            // Filtros
            webhooks.POST("/:instanceId/filters", webhookController.ConfigureFilters)
        }

        // ===== NUEVAS RUTAS DE LLAMADAS =====
        calls := v1.Group("/calls")
        {
            calls.POST("/:instanceId/reject", webhookController.RejectCall)
            calls.GET("/:instanceId/settings", webhookController.GetCallSettings)
            calls.PUT("/:instanceId/settings", webhookController.UpdateCallSettings)
        }
    }

    return router
}
