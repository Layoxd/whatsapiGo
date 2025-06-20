package main

import (
    "log"
    "os"

    "github.com/Layoxd/whatsapiGo/config"
    "github.com/Layoxd/whatsapiGo/controllers"
    "github.com/Layoxd/whatsapiGo/database"
    "github.com/Layoxd/whatsapiGo/routes"
    "github.com/Layoxd/whatsapiGo/utils"
    
    "go.mau.fi/whatsmeow/store/sqlstore"
    waLog "go.mau.fi/whatsmeow/util/log"
)

func main() {
    // Cargar configuración
    cfg := config.LoadConfig()
    
    // Configurar logger
    logger := utils.SetupLogger()
    waLogger := waLog.Stdout("WhatsApp", "INFO", true)
    
    // Conectar a PostgreSQL
    db, err := database.ConnectPostgreSQL(cfg.DatabaseURL)
    if err != nil {
        log.Fatalf("❌ Error conectando a PostgreSQL: %v", err)
    }
    defer db.Close()

    // Crear container del store para WhatsMeow
    container := sqlstore.NewWithDB(db, "postgres", waLogger)
    err = container.Upgrade()
    if err != nil {
        log.Fatalf("❌ Error actualizando schema del store: %v", err)
    }

    // Crear controladores
    instanceController := controllers.NewInstanceController(container, waLogger)
    messageController := controllers.NewMessageController(instanceController, waLogger)

    // Configurar rutas
    router := routes.SetupRoutes(instanceController, messageController)

    // Obtener puerto del environment o usar 8080 por defecto
    port := os.Getenv("PORT")
    if port == "" {
        port = "8080"
    }

    // Iniciar servidor
    logger.Sugar().Infof("🚀 WhatsApp API Server iniciado en puerto %s", port)
    logger.Sugar().Infof("📱 Endpoints disponibles:")
    logger.Sugar().Infof("   POST /api/v1/messages/text - Enviar texto")
    logger.Sugar().Infof("   POST /api/v1/messages/image - Enviar imagen")
    logger.Sugar().Infof("   POST /api/v1/messages/video - Enviar video")
    logger.Sugar().Infof("   POST /api/v1/messages/audio - Enviar audio")
    logger.Sugar().Infof("   POST /api/v1/messages/document - Enviar documento")
    logger.Sugar().Infof("   POST /api/v1/messages/location - Enviar ubicación")
    logger.Sugar().Infof("   POST /api/v1/messages/contact - Enviar contacto")
    
    log.Fatal(router.Run(":" + port))
}
