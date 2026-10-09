package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/rs/zerolog/log"
)

// MQTTClient gestisce la connessione MQTT
type MQTTClient struct {
	client    mqtt.Client
	config    *MqttConfig
	db        *Database
	topics    *TopicConfig
	coredumps *coredumpAssembler // nil = flusso coredump disattivato
}

// TopicConfig contiene la configurazione dei topic MQTT
type TopicConfig struct {
	Prefix string // es. "termostato/", sempre con lo slash finale
	Legacy bool   // sottoscrive anche <prefix>sample ecc. (firmware senza device id)
}

// NewMQTTClient crea un nuovo client MQTT. Con cdSaver nil i core dump non
// vengono sottoscritti.
func NewMQTTClient(config *MqttConfig, topics *TopicConfig, db *Database, cdSaver coredumpSaver) *MQTTClient {
	mc := &MQTTClient{
		config: config,
		topics: topics,
		db:     db,
	}
	if cdSaver != nil {
		mc.coredumps = newCoredumpAssembler(cdSaver, mc)
	}
	return mc
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

	if mc.coredumps != nil {
		mc.loadSavedCoredumps(ctx)
		go mc.expireCoredumps(ctx)
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

// subscription associa un filtro MQTT al suo handler
type subscription struct {
	filter  string
	qos     byte
	handler func(topic string, payload []byte)
}

// subscribeAll sottoscrive tutti i topic configurati.
// Niente <prefix># con switch sulla foglia: catturerebbe anche flussi non JSON
// (es. il futuro <prefix><id>/coredump/#).
func (mc *MQTTClient) subscribeAll() error {
	p := mc.topics.Prefix
	subs := []subscription{
		{p + "+/sample", 0, mc.withDevice(mc.handleSample)},
		{p + "+/opentherm", 1, mc.withDevice(mc.handleOpenTherm)},
		{p + "+/logs", 0, mc.withDevice(mc.handleLog)},
		{p + "+/status", 1, mc.withDevice(mc.handleStatus)},
		{p + "+/info", 1, mc.withDevice(mc.handleInfo)},
	}

	// Il firmware vecchio pubblica senza device id: <prefix>+/sample non
	// riceve <prefix>sample (livelli diversi), servono sottoscrizioni a parte
	if mc.topics.Legacy {
		subs = append(subs,
			subscription{p + "sample", 0, mc.withDevice(mc.handleSample)},
			subscription{p + "opentherm", 1, mc.withDevice(mc.handleOpenTherm)},
			subscription{p + "logs", 0, mc.withDevice(mc.handleLog)},
		)
	}

	// Solo data e meta: <prefix>+/coredump/# riceverebbe anche il nostro ack
	if mc.coredumps != nil {
		subs = append(subs,
			subscription{p + "+/coredump/+/data/+", 1, mc.handleCoredump},
			subscription{p + "+/coredump/+/meta", 1, mc.handleCoredump},
		)
	}

	for _, sub := range subs {
		if err := mc.subscribe(sub.filter, sub.qos, sub.handler); err != nil {
			return err
		}
	}

	log.Info().Msg("Successfully subscribed to all topics")
	return nil
}

// subscribe sottoscrive filter e passa all'handler topic e payload
func (mc *MQTTClient) subscribe(filter string, qos byte, handler func(topic string, payload []byte)) error {
	log.Info().
		Str("topic", filter).
		Msg("Subscribing to topic")

	token := mc.client.Subscribe(filter, qos, func(client mqtt.Client, msg mqtt.Message) {
		handler(msg.Topic(), msg.Payload())
	})

	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("subscribe to %s timeout", filter)
	}

	if err := token.Error(); err != nil {
		return fmt.Errorf("failed to subscribe to %s: %w", filter, err)
	}

	return nil
}

// withDevice adatta un handler che vuole solo il device ricavato dal topic
func (mc *MQTTClient) withDevice(handler func(device string, payload []byte)) func(topic string, payload []byte) {
	return func(topic string, payload []byte) {
		device, ok := mc.deviceOf(topic)
		if !ok {
			log.Warn().
				Str("topic", topic).
				Msg("Invalid device id in topic, message dropped")
			return
		}
		handler(device, payload)
	}
}

// deviceOf ricava il device dal topic: i topic legacy (<prefix><leaf>) sono
// del device legacyDevice, gli altri devono essere <prefix><id>/<leaf>
func (mc *MQTTClient) deviceOf(topic string) (string, bool) {
	if mc.topics.Legacy {
		rest, found := strings.CutPrefix(topic, mc.topics.Prefix)
		if found && !strings.Contains(rest, "/") {
			return legacyDevice, true
		}
	}

	id, _, ok := deviceFromTopic(mc.topics.Prefix, topic)
	return id, ok
}

// touchDevice aggiorna last_seen, così un device compare in devices anche
// senza aver mai mandato info
func (mc *MQTTClient) touchDevice(ctx context.Context, device string) {
	if device == legacyDevice {
		return
	}
	if err := mc.db.TouchDevice(ctx, device); err != nil {
		log.Error().
			Err(err).
			Str("device", device).
			Msg("Failed to update device last_seen")
	}
}

func (mc *MQTTClient) handleSample(device string, payload []byte) {
	var data SampleSt
	if err := json.Unmarshal(payload, &data); err != nil {
		log.Error().
			Err(err).
			Str("device", device).
			Msg("Failed to unmarshal measure data")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := mc.db.InsertMeasure(ctx, device, data); err != nil {
		log.Error().
			Err(err).
			Str("device", device).
			Msg("Failed to insert measure")
	}
	mc.touchDevice(ctx, device)
}

func (mc *MQTTClient) handleOpenTherm(device string, payload []byte) {
	var data OtDataSt
	if err := json.Unmarshal(payload, &data); err != nil {
		log.Error().
			Err(err).
			Str("device", device).
			Msg("Failed to unmarshal OpenTherm data")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := mc.db.InsertOpenTherm(ctx, device, data); err != nil {
		log.Error().
			Err(err).
			Str("device", device).
			Msg("Failed to insert OpenTherm data")
	}
	mc.touchDevice(ctx, device)
}

func (mc *MQTTClient) handleLog(device string, payload []byte) {
	var data EspLogSt
	if err := json.Unmarshal(payload, &data); err != nil {
		log.Error().
			Err(err).
			Str("device", device).
			Msg("Failed to unmarshal log data")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := mc.db.InsertLog(ctx, device, data); err != nil {
		log.Error().
			Err(err).
			Str("device", device).
			Msg("Failed to insert log")
	}
	mc.touchDevice(ctx, device)
}

func (mc *MQTTClient) handleStatus(device string, payload []byte) {
	// Payload vuoto = retained cancellato (mosquitto_pub -r -n)
	if len(payload) == 0 {
		return
	}

	online, ok := parseStatus(payload)
	if !ok {
		log.Warn().
			Str("device", device).
			Str("payload", string(payload)).
			Msg("Invalid status payload, message dropped")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := mc.db.UpsertDeviceStatus(ctx, device, online); err != nil {
		log.Error().
			Err(err).
			Str("device", device).
			Msg("Failed to update device status")
	}
}

func (mc *MQTTClient) handleInfo(device string, payload []byte) {
	// Payload vuoto = retained cancellato (mosquitto_pub -r -n)
	if len(payload) == 0 {
		return
	}

	var info DeviceInfoSt
	if err := json.Unmarshal(payload, &info); err != nil {
		log.Error().
			Err(err).
			Str("device", device).
			Msg("Failed to unmarshal device info")
		return
	}

	// Fa fede il topic: l'id nel payload è solo informativo
	if info.ID != nil && *info.ID != device {
		log.Warn().
			Str("device", device).
			Str("payload_id", *info.ID).
			Msg("Device info id differs from topic, using topic")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := mc.db.UpsertDeviceInfo(ctx, device, info); err != nil {
		log.Error().
			Err(err).
			Str("device", device).
			Msg("Failed to update device info")
	}
}

func (mc *MQTTClient) handleCoredump(topic string, payload []byte) {
	dev, cid, kind, n, ok := coredumpFromTopic(mc.topics.Prefix, topic)
	if !ok {
		log.Warn().
			Str("topic", topic).
			Msg("Invalid coredump topic, message dropped")
		return
	}
	mc.coredumps.Handle(dev, cid, kind, n, payload)
}

// AckCoredump pubblica l'ack retained: il device può cancellare la flash.
// Niente Wait nell'handler (con OrderMatters si bloccherebbe): l'esito si
// controlla in una goroutine.
func (mc *MQTTClient) AckCoredump(dev, cid string) {
	topic := mc.topics.Prefix + dev + "/coredump/" + cid + "/ack"
	token := mc.client.Publish(topic, 1, true, "ok")

	go func() {
		if !token.WaitTimeout(10 * time.Second) {
			log.Warn().Str("topic", topic).Msg("Coredump ack publish timeout")
			return
		}
		if err := token.Error(); err != nil {
			log.Error().Err(err).Str("topic", topic).Msg("Failed to publish coredump ack")
			return
		}
		log.Info().Str("topic", topic).Msg("Coredump ack published")
	}()
}

// loadSavedCoredumps carica i core dump già in tabella, così i reinvii
// ricevono solo l'ack senza interrogare il database a ogni pezzo
func (mc *MQTTClient) loadSavedCoredumps(ctx context.Context) {
	keys, err := mc.db.ListCoredumpKeys(ctx)
	if err != nil {
		// Non bloccante: al peggio un reinvio viene riscritto su disco
		log.Error().Err(err).Msg("Failed to load saved coredumps")
		return
	}
	for _, k := range keys {
		mc.coredumps.MarkSaved(k.Device, k.ID)
	}
	log.Info().Int("count", len(keys)).Msg("Loaded saved coredumps")
}

// expireCoredumps ripulisce ogni ora gli stati parziali scaduti
func (mc *MQTTClient) expireCoredumps(ctx context.Context) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			mc.coredumps.Expire()
		}
	}
}

// Disconnect disconnette dal broker MQTT
func (mc *MQTTClient) Disconnect() {
	if mc.client != nil && mc.client.IsConnected() {
		mc.client.Disconnect(250)
		log.Info().Msg("Disconnected from MQTT broker")
	}
}
