package routes

import (
    "github.com/gin-gonic/gin"
    "github.com/Layoxd/whatsapiGo/controllers"
)

// SetupRoutes - Configurar todas las rutas de la API
func SetupRoutes(
    instanceController *controllers.InstanceController,
    messageController *controllers.MessageController,
    contactController *controllers.ContactController,
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

    // Health check
    router.GET("/health", func(c *gin.Context) {
        c.JSON(200, gin.H{"status": "ok", "service": "WhatsApp API Go"})
    })

    // Grupo de rutas API v1
    v1 := router.Group("/api/v1")
    {
        // Rutas de instancias
        instances := v1.Group("/instances")
        {
            instances.POST("", instanceController.CreateInstance)
            instances.GET("", instanceController.GetInstances)
            instances.GET("/:id", instanceController.GetInstance)
            instances.DELETE("/:id", instanceController.DeleteInstance)
            instances.GET("/:id/qr", instanceController.GetQRCode)
            instances.POST("/:id/logout", instanceController.LogoutInstance)
        }

        // Rutas de mensajes
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

        // Rutas de contactos
        contacts := v1.Group("/contacts")
        {
            contacts.GET("/:instanceId", contactController.GetContacts)
            contacts.GET("/:instanceId/search", contactController.SearchContacts)
            contacts.GET("/:instanceId/info/:jid", contactController.GetContactInfo)
            contacts.POST("/:instanceId/check", contactController.CheckContacts)
            contacts.POST("/:instanceId/block", contactController.BlockContact)
            contacts.POST("/:instanceId/unblock", contactController.UnblockContact)
            
            // Rutas para conversión LID ↔ JID
            contacts.GET("/:instanceId/lid/get", contactController.GetLIDFromJID)
            contacts.GET("/:instanceId/lid/from-lid", contactController.GetJIDFromLID)
        }
    }

    return router
}
