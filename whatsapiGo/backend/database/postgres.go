package database

import (
	"database/sql"
	"fmt"
	
	_ "github.com/lib/pq"           // ← EXISTENTE
	"gorm.io/driver/postgres"      // ← NUEVO IMPORT
	"gorm.io/gorm"                 // ← NUEVO IMPORT
	"gorm.io/gorm/logger"          // ← NUEVO IMPORT
)

// ConnectPostgreSQL - Conectar a PostgreSQL (para WhatsMeow)
// ← FUNCIÓN EXISTENTE - MANTENER IGUAL
func ConnectPostgreSQL(databaseURL string) (*sql.DB, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("error abriendo conexión a PostgreSQL: %w", err)
	}

	// Verificar conexión
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("error conectando a PostgreSQL: %w", err)
	}

	// Configurar pool de conexiones
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	fmt.Println("✅ Conexión a PostgreSQL establecida exitosamente")
	return db, nil
}

// ===== NUEVAS FUNCIONES PARA GORM =====

// ConnectGORM - Conectar a PostgreSQL usando GORM (para webhooks)
func ConnectGORM(databaseURL string) (*gorm.DB, error) {
	// Configurar GORM con logger personalizado
	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	}

	// Abrir conexión GORM
	db, err := gorm.Open(postgres.Open(databaseURL), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("error conectando a PostgreSQL con GORM: %w", err)
	}

	// Obtener la instancia de database/sql subyacente para configurar pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("error obteniendo instancia SQL de GORM: %w", err)
	}

	// Configurar pool de conexiones
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)

	// Verificar conexión
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("error verificando conexión GORM: %w", err)
	}

	fmt.Println("✅ Conexión GORM a PostgreSQL establecida exitosamente")
	return db, nil
}

// ConnectFromSQL - Crear instancia GORM desde conexión SQL existente
func ConnectFromSQL(sqlDB *sql.DB) (*gorm.DB, error) {
	// Crear instancia GORM usando conexión existente
	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn: sqlDB,
	}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	
	if err != nil {
		return nil, fmt.Errorf("error creando instancia GORM desde SQL: %w", err)
	}

	fmt.Println("✅ Instancia GORM creada desde conexión SQL existente")
	return gormDB, nil
}

// TestConnection - Probar conexión a la base de datos
func TestConnection(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("error obteniendo instancia SQL: %w", err)
	}

	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("error en ping a la base de datos: %w", err)
	}

	fmt.Println("✅ Test de conexión exitoso")
	return nil
}

// GetConnectionStats - Obtener estadísticas de conexión
func GetConnectionStats(db *gorm.DB) (map[string]interface{}, error) {
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("error obteniendo instancia SQL: %w", err)
	}

	stats := sqlDB.Stats()
	
	return map[string]interface{}{
		"max_open_connections":     stats.MaxOpenConnections,
		"open_connections":         stats.OpenConnections,
		"in_use":                  stats.InUse,
		"idle":                    stats.Idle,
		"wait_count":              stats.WaitCount,
		"wait_duration":           stats.WaitDuration.String(),
		"max_idle_closed":         stats.MaxIdleClosed,
		"max_idle_time_closed":    stats.MaxIdleTimeClosed,
		"max_lifetime_closed":     stats.MaxLifetimeClosed,
	}, nil
}
