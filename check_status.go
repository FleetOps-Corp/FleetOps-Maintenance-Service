//go:build dev
// +build dev

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/fleetops/maintenance/internal/adapter/repository"
)

func main() {
	_ = godotenv.Load()
	pool, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		panic(err)
	}
	repo := repository.NewPostgresMaintenanceRepository(pool)
	id, _ := uuid.Parse("d5bed4cd-3b18-4893-9782-d559289c0f91")
	m, err := repo.GetByID(context.Background(), id)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Printf("Status of %s is: %s\n", id, m.Status)
	}
}
