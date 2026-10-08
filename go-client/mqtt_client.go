package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/rs/zerolog/log"
)

// MQTTClient gestisce la connessione MQTT
type MQTTClient struct {
	client mqtt.Client
	config *MqttConfig
	db     *Database
	topics *TopicConfig
}

// TopicConfig contiene i topic MQTT da sottoscrivere
type TopicConfig struct {
	MeasureTopic   string
	OpenThermTopic string
	LogTopic       string
}

// NewMQTTClient crea un nuovo client MQTT
func NewMQTTClient(config *MqttConfig, topics *TopicConfig, db *Database) *MQTTClient {
	return &MQTTClient{
		config: config,
		topics: topics,
		db:     db,
	}
}

// createClientOptions crea le opzioni per il client MQTT
func (mc *MQTTClient) createClientOptions(clientID string, uri *url.URL) *mqtt.ClientOptions {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s", uri.Host))
	opts.SetUsername(uri.User.Username())
	password, _ := uri.User.Password()
	opts.SetPassword(password)
	opts.SetClientID(clientID)
	return opts
}

// Connect connette al broker MQTT e sottoscrive i topic
func (mc *MQTTClient) Connect(ctx context.Context) error {
	var confURI string

	if mc.config.User != "" && mc.config.Password != "" {
		confURI = fmt.Sprintf("mqtt://%s:%s@%s:%d",
			mc.config.User,
			mc.config.Password,
			mc.config.Host,
			mc.config.Port)
	} else {
		confURI = fmt.Sprintf("mqtt://%s:%d",
			mc.config.Host,
			mc.config.Port)
	}

	uri, err := url.Parse(confURI)
	if err != nil {
		return fmt.Errorf("failed to parse MQTT URI: %w", err)
	}

	opts := mc.createClientOptions("termostato-client", uri)
	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(5 * time.Second)

	// Le subscribe vanno rifatte ad ogni (ri)connessione: con clean session
	// il broker le dimentica e l'auto-reconnect non le ripristina.
	opts.SetOnConnectHandler(func(client mqtt.Client) {
		log.Info().
			Str("broker", uri.Host).
			Msg("Connected to MQTT broker")

		if err := mc.subscribeAll(); err != nil {
			log.Error().Err(err).Msg("Failed to subscribe topics")
		}
	})
	opts.SetConnectionLostHandler(func(client mqtt.Client, err error) {
		log.Warn().Err(err).Msg("MQTT connection lost, reconnecting...")
	})

	mc.client = mqtt.NewClient(opts)
	token := mc.client.Connect()

	// Con ConnectRetry il token si completa solo a connessione riuscita:
	// attendi senza timeout finché non arriva un segnale di terminazione
	select {
	case <-token.Done():
	case <-ctx.Done():
		return ctx.Err()
	}

	if err := token.Error(); err != nil {
		return fmt.Errorf("MQTT connection failed: %w", err)
	}

	return nil
}

// subscribeAll sottoscrive tutti i topic configurati
func (mc *MQTTClient) subscribeAll() error {
	// Subscribe to measure topic
	if err := mc.subscribeMeasureTopic(); err != nil {
		return err
	}

	// Subscribe to OpenTherm topic
	if err := mc.subscribeOpenThermTopic(); err != nil {
		return err
	}

	// Subscribe to log topic
	if err := mc.subscribeLogTopic(); err != nil {
		return err
	}

	log.Info().Msg("Successfully subscribed to all topics")
	return nil
}

// subscribeMeasureTopic sottoscrive il topic delle misure
func (mc *MQTTClient) subscribeMeasureTopic() error {
	log.Info().
		Str("topic", mc.topics.MeasureTopic).
		Msg("Subscribing to measure topic")

	token := mc.client.Subscribe(mc.topics.MeasureTopic, 0, func(client mqtt.Client, msg mqtt.Message) {
		var data SampleSt
		if err := json.Unmarshal(msg.Payload(), &data); err != nil {
			log.Error().
				Err(err).
				Str("topic", msg.Topic()).
				Msg("Failed to unmarshal measure data")
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := mc.db.InsertMeasure(ctx, data); err != nil {
			log.Error().
				Err(err).
				Msg("Failed to insert measure")
		}
	})

	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("subscribe to %s timeout", mc.topics.MeasureTopic)
	}

	if err := token.Error(); err != nil {
		return fmt.Errorf("failed to subscribe to %s: %w", mc.topics.MeasureTopic, err)
	}

	return nil
}

// subscribeOpenThermTopic sottoscrive il topic OpenTherm
func (mc *MQTTClient) subscribeOpenThermTopic() error {
	log.Info().
		Str("topic", mc.topics.OpenThermTopic).
		Msg("Subscribing to OpenTherm topic")

	token := mc.client.Subscribe(mc.topics.OpenThermTopic, 0, func(client mqtt.Client, msg mqtt.Message) {
		var data OtDataSt
		if err := json.Unmarshal(msg.Payload(), &data); err != nil {
			log.Error().
				Err(err).
				Str("topic", msg.Topic()).
				Msg("Failed to unmarshal OpenTherm data")
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := mc.db.InsertOpenTherm(ctx, data); err != nil {
			log.Error().
				Err(err).
				Msg("Failed to insert OpenTherm data")
		}
	})

	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("subscribe to %s timeout", mc.topics.OpenThermTopic)
	}

	if err := token.Error(); err != nil {
		return fmt.Errorf("failed to subscribe to %s: %w", mc.topics.OpenThermTopic, err)
	}

	return nil
}

// subscribeLogTopic sottoscrive il topic dei log ESP32
func (mc *MQTTClient) subscribeLogTopic() error {
	log.Info().
		Str("topic", mc.topics.LogTopic).
		Msg("Subscribing to log topic")

	token := mc.client.Subscribe(mc.topics.LogTopic, 0, func(client mqtt.Client, msg mqtt.Message) {
		var data EspLogSt
		if err := json.Unmarshal(msg.Payload(), &data); err != nil {
			log.Error().
				Err(err).
				Str("topic", msg.Topic()).
				Msg("Failed to unmarshal log data")
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := mc.db.InsertLog(ctx, data); err != nil {
			log.Error().
				Err(err).
				Msg("Failed to insert log")
		}
	})

	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("subscribe to %s timeout", mc.topics.LogTopic)
	}

	if err := token.Error(); err != nil {
		return fmt.Errorf("failed to subscribe to %s: %w", mc.topics.LogTopic, err)
	}

	return nil
}

// Disconnect disconnette dal broker MQTT
func (mc *MQTTClient) Disconnect() {
	if mc.client != nil && mc.client.IsConnected() {
		mc.client.Disconnect(250)
		log.Info().Msg("Disconnected from MQTT broker")
	}
}
