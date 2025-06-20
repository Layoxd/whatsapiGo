package routes

import (
    "github.com/gin-gonic/gin"
    "tu-proyecto/controllers"
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

    // Grupo de rutas para instancias
    v1 := router.Group("/api/v1")
    {
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
}// Archivo base: routes.go
