package main

import (
    "log"
    "os"

    "github.com/Layoxd/whatsapiGo/config"
    "github.com/Layoxd/whatsapiGo/controllers"
    "github.com/Layoxd/whatsapiGo/database"
    "github.com/Layoxd/whatsapiGo/models"     // ← NUEVO IMPORT
    "github.com/Layoxd/whatsapiGo/routes"
    "github.com/Layoxd/whatsapiGo/services"  // ← NUEVO IMPORT
    "github.com/Layoxd/whatsapiGo/utils"
    
    "go.mau.fi/whatsmeow/store/sqlstore"
    waLog "go.mau.fi/whatsmeow/util/log"
    "gorm.io/driver/postgres"  // ← NUEVO IMPORT
    "gorm.io/gorm"             // ← NUEVO IMPORT
)

func main() {
    // Cargar configuración
    cfg := config.LoadConfig()
    
    // Configurar logger
    logger := utils.SetupLogger()
    waLogger := waLog.Stdout("WhatsApp", "INFO", true)
    
    // Conectar a PostgreSQL (SQL tradicional para WhatsMeow)
    sqlDB, err := database.ConnectPostgreSQL(cfg.DatabaseURL)
    if err != nil {
        log.Fatalf("❌ Error conectando a PostgreSQL: %v", err)
    }
    defer sqlDB.Close()

    // ===== NUEVA SECCIÓN: CONEXIÓN GORM PARA WEBHOOKS =====
    gormDB, err := gorm.Open(postgres.New(postgres.Config{
        Conn: sqlDB,
    }), &gorm.Config{})
    if err != nil {
        log.Fatalf("❌ Error inicializando GORM: %v", err)
    }

    // ===== NUEVA SECCIÓN: AUTO-MIGRACIÓN DE TABLAS DE WEBHOOK =====
    err = gormDB.AutoMigrate(
        &models.WebhookConfig{},
        &models.WebhookMetrics{},
        &models.WebhookLog{},
        &models.CallRejectConfig{},
    )
    if err != nil {
        log.Fatalf("❌ Error migrando tablas de webhook: %v", err)
    }
    logger.Sugar().Info("✅ Migración de tablas de webhook completada")

    // Crear container del store para WhatsMeow
    container := sqlstore.NewWithDB(sqlDB, "postgres", waLogger)
    err = container.Upgrade()
    if err != nil {
        log.Fatalf("❌ Error actualizando schema del store: %v", err)
    }

    // ===== NUEVA SECCIÓN: CREAR SERVICIO DE WEBHOOK =====
    webhookService := services.NewWebhookService(gormDB, logger)

    // Crear controladores existentes
    instanceController := controllers.NewInstanceController(container, waLogger)
    messageController := controllers.NewMessageController(instanceController, waLogger)
    contactController := controllers.NewContactController(instanceController, waLogger)
    groupController := controllers.NewGroupController(instanceController, waLogger)
    statusController := controllers.NewStatusController(instanceController, waLogger)

    // ===== NUEVA SECCIÓN: CREAR CONTROLADOR DE WEBHOOK =====
    webhookController := controllers.NewWebhookController(gormDB, logger)

    // ===== CONFIGURAR RUTAS ACTUALIZADAS =====
    router := routes.SetupRoutes(
        instanceController, 
        messageController, 
        contactController, 
        groupController, 
        statusController,
        webhookController,  // ← NUEVO PARÁMETRO
        webhookService,     // ← NUEVO PARÁMETRO
    )

    // Obtener puerto del environment o usar 8080 por defecto
    port := os.Getenv("PORT")
    if port == "" {
        port = "8080"
    }

    // Iniciar servidor con logs actualizados
    logger.Sugar().Infof("🚀 WhatsApp API Server iniciado en puerto %s", port)
    logger.Sugar().Infof("📱 Endpoints de mensajes con doble soporte (Base64 + URL)")
    logger.Sugar().Infof("👥 Endpoints de contactos con soporte LID completo")
    logger.Sugar().Infof("👥 Endpoints de grupos con gestión avanzada")
    logger.Sugar().Infof("📢 Endpoints de estados/stories con multimedia completo")
    logger.Sugar().Infof("🔔 Endpoints de webhooks con sistema empresarial")  // ← NUEVO LOG
    logger.Sugar().Infof("📞 Endpoints de llamadas con auto-rechazo inteligente")  // ← NUEVO LOG
    logger.Sugar().Infof("🔄 Conversión JID ↔ LID habilitada en todo el sistema")
    
    log.Fatal(router.Run(":" + port))
}
