package main

import (
	"os"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v2"
)

const CONFIGFILENAME = "config.yaml"

// LoadConfig carica e valida la configurazione dal file YAML
func LoadConfig() (*Config, error) {
	var config Config
	valid := true

	body, err := os.ReadFile(CONFIGFILENAME)
	if err != nil {
		log.Fatal().Err(err).Msg("Unable to read config file")
		return nil, err
	}

	err = yaml.Unmarshal(body, &config)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to parse config file")
		return nil, err
	}

	// Validate database config
	if config.Db.Dbname == "" {
		log.Error().Msg("DB name not found")
		valid = false
	}
	if config.Db.Host == "" {
		log.Error().Msg("Host name not found")
		valid = false
	}
	if config.Db.Password == "" {
		log.Error().Msg("Password not found")
		valid = false
	}
	if config.Db.User == "" {
		log.Error().Msg("User not found")
		valid = false
	}
	if config.Db.Port == 0 {
		config.Db.Port = 5432
		log.Info().Int("port", 5432).Msg("Using default DB port")
	}

	// Validate MQTT entry
	if config.Mqtt.Host == "" {
		log.Error().Msg("MQTT host not found")
		valid = false
	}

	if config.Mqtt.Port == 0 {
		config.Mqtt.Port = 1883
		log.Info().Int("port", 1883).Msg("Using default MQTT port")
	}

	// Validate topics
	if config.MeasureTopic == "" {
		log.Error().Msg("MQTT measure topic not found in config")
		valid = false
	}

	if config.OpenThermTopic == "" {
		log.Error().Msg("MQTT OpenTherm topic not found in config")
		valid = false
	}

	if config.LogTopic == "" {
		log.Error().Msg("MQTT log topic not found in config")
		valid = false
	}

	// Check if valid
	if !valid {
		log.Fatal().Msg("Config is not valid")
		return nil, err
	}

	log.Info().Msg("Config loaded successfully")
	return &config, nil
}
