package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/centaur-vova/sap-segmentation/internal/config"
	"github.com/centaur-vova/sap-segmentation/internal/importer"
	"github.com/centaur-vova/sap-segmentation/internal/logger"
	"github.com/centaur-vova/sap-segmentation/internal/model"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	log, err := logger.NewLogger(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if err := log.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to close logger: %v\n", err)
		}
	}()

	log.Info("Starting SAP segmentation import")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Info("Received shutdown signal", "signal", sig.String())
		cancel()
	}()

	// Подключение к БД с пулом
	db, err := sqlx.ConnectContext(ctx, "postgres", cfg.GetDBConnectionString())
	if err != nil {
		log.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Error("Failed to close database", "error", err)
		}
	}()

	// Настройка пула соединений
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)

	log.Info("Successfully connected to database")

	// Инициализация модели и импортера
	segmentationModel := model.NewSegmentationModel(db)
	imp := importer.NewImporter(cfg, log.Logger, segmentationModel)

	// Запуск импорта
	if err := imp.Run(ctx); err != nil {
		log.Error("Import failed", "error", err)
		os.Exit(1)
	}

	log.Info("Import completed successfully")
}
