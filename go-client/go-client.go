package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	// Configura zerolog per output human-friendly
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	log.Info().Msg("Starting Termostato MQTT client...")

	// Carica la configurazione
	config, err := LoadConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load config")
	}

	// Connetti al database
	log.Info().Msg("Connecting to database...")
	db, err := NewDatabase(ctx, &config.Db)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer db.Close()

	// Configura il client MQTT
	log.Info().Msg("Setting up MQTT client...")
	topics := &TopicConfig{
		MeasureTopic:   config.MeasureTopic,
		OpenThermTopic: config.OpenThermTopic,
		LogTopic:       config.LogTopic,
	}
	mqttClient := NewMQTTClient(&config.Mqtt, topics, db)

	if err := mqttClient.Connect(ctx); err != nil {
		log.Fatal().Err(err).Msg("Failed to setup MQTT client")
	}
	defer mqttClient.Disconnect()

	log.Info().Msg("System ready. Waiting for messages...")

	// Attendi segnale di terminazione
	sig := <-sigs
	log.Info().
		Str("signal", sig.String()).
		Msg("Received signal, shutting down gracefully...")

	cancel()

	// Tempo per la chiusura pulita
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	<-shutdownCtx.Done()

	log.Info().Msg("Shutdown complete")
}
