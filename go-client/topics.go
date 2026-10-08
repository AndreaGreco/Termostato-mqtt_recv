package main

import "strings"

// legacyDevice è il device assegnato ai messaggi dei topic senza id
// (firmware precedente al multi-device) e alle righe storiche
const legacyDevice = "legacy"

const maxDeviceIDLen = 32

// normalizeTopicPrefix garantisce lo slash finale del prefisso
func normalizeTopicPrefix(prefix string) string {
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		return prefix + "/"
	}
	return prefix
}

// deviceFromTopic estrae device e foglia da un topic <prefix><id>/<leaf>.
// Rifiuta id vuoti, più lunghi di maxDeviceIDLen, livelli in più e l'id
// riservato legacyDevice.
func deviceFromTopic(prefix, topic string) (id, leaf string, ok bool) {
	rest, found := strings.CutPrefix(topic, normalizeTopicPrefix(prefix))
	if !found {
		return "", "", false
	}

	id, leaf, found = strings.Cut(rest, "/")
	if !found || id == "" || leaf == "" || strings.Contains(leaf, "/") {
		return "", "", false
	}
	if len(id) > maxDeviceIDLen || id == legacyDevice {
		return "", "", false
	}

	return id, leaf, true
}

// parseStatus interpreta il payload di <prefix><id>/status
func parseStatus(payload []byte) (online bool, ok bool) {
	switch string(payload) {
	case "online":
		return true, true
	case "offline":
		return false, true
	}
	return false, false
}
