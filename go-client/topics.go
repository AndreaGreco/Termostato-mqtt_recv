package main

import (
	"strconv"
	"strings"
)

// legacyDevice è il device assegnato ai messaggi dei topic senza id
// (firmware precedente al multi-device) e alle righe storiche
const legacyDevice = "legacy"

const maxDeviceIDLen = 32

// maxCoredumpChunks è il limite sull'indice dei pezzi di un core dump
const maxCoredumpChunks = 1024

// normalizeTopicPrefix garantisce lo slash finale del prefisso
func normalizeTopicPrefix(prefix string) string {
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		return prefix + "/"
	}
	return prefix
}

// validDeviceID applica le regole comuni sugli id dei device: non vuoto, non
// più lungo di maxDeviceIDLen, diverso da legacyDevice e da "." / ".." (l'id
// è usato anche come nome di directory per i core dump)
func validDeviceID(id string) bool {
	return id != "" && len(id) <= maxDeviceIDLen &&
		id != legacyDevice && id != "." && id != ".."
}

// deviceFromTopic estrae device e foglia da un topic <prefix><id>/<leaf>.
// Rifiuta id non validi (vedi validDeviceID) e livelli in più.
func deviceFromTopic(prefix, topic string) (id, leaf string, ok bool) {
	rest, found := strings.CutPrefix(topic, normalizeTopicPrefix(prefix))
	if !found {
		return "", "", false
	}

	id, leaf, found = strings.Cut(rest, "/")
	if !found || !validDeviceID(id) || leaf == "" || strings.Contains(leaf, "/") {
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

// coredumpFromTopic interpreta <prefix><dev>/coredump/<cid>/data/<n> e
// <prefix><dev>/coredump/<cid>/meta (per meta n è 0). <cid> deve essere di
// 8 cifre esadecimali minuscole, <n> decimale senza zeri iniziali e minore di
// maxCoredumpChunks. Qualsiasi altra foglia (anche ack) o livello in più è
// rifiutato.
func coredumpFromTopic(prefix, topic string) (dev, cid, kind string, n int, ok bool) {
	rest, found := strings.CutPrefix(topic, normalizeTopicPrefix(prefix))
	if !found {
		return "", "", "", 0, false
	}

	parts := strings.Split(rest, "/")
	if len(parts) < 4 || parts[1] != "coredump" || !validDeviceID(parts[0]) || !validCoredumpID(parts[2]) {
		return "", "", "", 0, false
	}

	switch {
	case len(parts) == 4 && parts[3] == "meta":
		return parts[0], parts[2], "meta", 0, true
	case len(parts) == 5 && parts[3] == "data":
		n, ok := parseChunkIndex(parts[4])
		if !ok {
			return "", "", "", 0, false
		}
		return parts[0], parts[2], "data", n, true
	}
	return "", "", "", 0, false
}

// validCoredumpID verifica che cid sia un CRC-32 di 8 cifre hex minuscole
func validCoredumpID(cid string) bool {
	if len(cid) != 8 {
		return false
	}
	for _, c := range cid {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// parseChunkIndex accetta solo decimali canonici (niente segno né zeri
// iniziali) minori di maxCoredumpChunks
func parseChunkIndex(s string) (int, bool) {
	if s == "" || len(s) > 4 || (len(s) > 1 && s[0] == '0') {
		return 0, false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, false
		}
	}
	n, err := strconv.Atoi(s)
	if err != nil || n >= maxCoredumpChunks {
		return 0, false
	}
	return n, true
}
