package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/Layoxd/whatsapiGo/controllers"
)

// SetupRoutes - Configurar todas las rutas de la API
func SetupRoutes(instanceController *controllers.InstanceController) *gin.Engine {
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
	}

	return router
}
