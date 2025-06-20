package main

import (
    "log"
    "database/sql"
    
    "github.com/gin-gonic/gin"
    "go.mau.fi/whatsmeow/store/sqlstore"
    waLog "go.mau.fi/whatsmeow/util/log"
    _ "github.com/lib/pq"
    
    "tu-proyecto/controllers"
    "tu-proyecto/routes"
)

func main() {
    // Configurar logger
    logger := waLog.Stdout("Main", "INFO", true)
    
    // Conectar a PostgreSQL
    db, err := sql.Open("postgres", "postgres://user:password@localhost/whatsapp_api?sslmode=disable")
    if err != nil {
        log.Fatalf("Error conectando a PostgreSQL: %v", err)
    }
    defer db.Close()

    // Crear container del store para WhatsMeow
    container := sqlstore.NewWithDB(db, "postgres", logger)
    err = container.Upgrade()
    if err != nil {
        log.Fatalf("Error actualizando schema del store: %v", err)
    }

    // Crear controladores
    instanceController := controllers.NewInstanceController(container, logger)

    // Configurar rutas
    router := routes.SetupRoutes(instanceController)

    // Iniciar servidor
    logger.Infof("🚀 Servidor iniciado en puerto 8080")
    log.Fatal(router.Run(":8080"))
}// Archivo base: main.go
