package database

import (
	"database/sql"
	"fmt"
	
	_ "github.com/lib/pq"
)

// ConnectPostgreSQL - Conectar a PostgreSQL
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
}// Archivo base: postgres.go
