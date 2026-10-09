package main

import (
	"encoding/json"
	"fmt"
	"hash/crc32"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	maxCoredumpSize     = 1 << 20        // 1 MB
	maxCoredumpPartials = 16             // (dev,cid) in ricomposizione contemporanei
	coredumpPartialTTL  = 24 * time.Hour // il device ripubblica ogni 12 h
	coredumpAckInterval = 5 * time.Second
)

// coredumpMeta è il payload di <prefix><dev>/coredump/<cid>/meta.
// I campi da ElfSHA256 in giù possono mancare: restano nil e vanno NULL.
type coredumpMeta struct {
	Device             string   `json:"device"`
	ID                 string   `json:"id"`
	Size               int      `json:"size"`
	ChunkSize          int      `json:"chunk_size"`
	Chunks             int      `json:"chunks"`
	CRC32              string   `json:"crc32"`
	ElfSHA256          *string  `json:"elf_sha256"`
	Task               *string  `json:"task"`
	PC                 *string  `json:"pc"`
	Backtrace          []string `json:"backtrace"`
	BacktraceCorrupted *bool    `json:"backtrace_corrupted"`
	PanicReason        *string  `json:"panic_reason"`
}

// validate controlla i limiti del meta
func (m *coredumpMeta) validate() error {
	switch {
	case m.Size <= 0 || m.Size > maxCoredumpSize:
		return fmt.Errorf("size %d out of range (1..%d)", m.Size, maxCoredumpSize)
	case m.ChunkSize <= 0:
		return fmt.Errorf("chunk_size %d must be > 0", m.ChunkSize)
	case m.Chunks <= 0 || m.Chunks > maxCoredumpChunks:
		return fmt.Errorf("chunks %d out of range (1..%d)", m.Chunks, maxCoredumpChunks)
	case m.Chunks != (m.Size+m.ChunkSize-1)/m.ChunkSize:
		return fmt.Errorf("chunks %d inconsistent with size %d / chunk_size %d", m.Chunks, m.Size, m.ChunkSize)
	}
	return nil
}

// coredumpSaver salva un core dump ricomposto e verificato (file + database).
// Deve restituire nil solo se è tutto scritto: dopo parte l'ack.
type coredumpSaver interface {
	SaveCoredump(dev, cid string, meta *coredumpMeta, rawMeta, img []byte) error
}

// coredumpAcker pubblica l'ack retained di un core dump salvato
type coredumpAcker interface {
	AckCoredump(dev, cid string)
}

type coredumpKey struct{ dev, cid string }

type coredumpPartial struct {
	chunks  map[int][]byte
	bytes   int           // somma delle lunghezze dei pezzi
	meta    *coredumpMeta // nil finché non arriva
	rawMeta []byte
	first   time.Time // primo messaggio visto, per la scadenza
}

// coredumpAssembler ricompone i core dump a pezzi. I messaggi retained
// arrivano in ordine qualsiasi e vengono reinviati: è idempotente.
type coredumpAssembler struct {
	mu       sync.Mutex
	saver    coredumpSaver
	acker    coredumpAcker
	now      func() time.Time
	partials map[coredumpKey]*coredumpPartial
	saved    map[coredumpKey]bool      // già in tabella coredumps
	lastAck  map[coredumpKey]time.Time // per non ripetere l'ack a ogni pezzo
}

func newCoredumpAssembler(saver coredumpSaver, acker coredumpAcker) *coredumpAssembler {
	return &coredumpAssembler{
		saver:    saver,
		acker:    acker,
		now:      time.Now,
		partials: make(map[coredumpKey]*coredumpPartial),
		saved:    make(map[coredumpKey]bool),
		lastAck:  make(map[coredumpKey]time.Time),
	}
}

// MarkSaved registra un core dump già presente in tabella (caricati all'avvio)
func (a *coredumpAssembler) MarkSaved(dev, cid string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.saved[coredumpKey{dev, cid}] = true
}

// Handle elabora un messaggio data/<n> o meta già validato dal topic
func (a *coredumpAssembler) Handle(dev, cid, kind string, n int, payload []byte) {
	// Payload vuoto = il device sta ripulendo i retained dopo l'ack
	if len(payload) == 0 {
		log.Debug().
			Str("device", dev).
			Str("cid", cid).
			Str("kind", kind).
			Msg("Ignoring empty coredump payload (retained cleanup)")
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	k := coredumpKey{dev, cid}

	// Già salvato: niente riscrittura, solo l'ack (il device non l'ha visto)
	if a.saved[k] {
		delete(a.partials, k)
		a.ackThrottled(k)
		return
	}

	switch kind {
	case "meta":
		a.handleMeta(k, payload)
	case "data":
		a.handleData(k, n, payload)
	}
}

func (a *coredumpAssembler) handleMeta(k coredumpKey, payload []byte) {
	var meta coredumpMeta
	if err := json.Unmarshal(payload, &meta); err != nil {
		log.Error().
			Err(err).
			Str("device", k.dev).
			Str("cid", k.cid).
			Msg("Failed to unmarshal coredump meta")
		return
	}

	// Fa fede il topic
	if meta.Device != k.dev || meta.ID != k.cid || meta.CRC32 != k.cid {
		log.Warn().
			Str("device", k.dev).
			Str("cid", k.cid).
			Str("meta_device", meta.Device).
			Str("meta_id", meta.ID).
			Str("meta_crc32", meta.CRC32).
			Msg("Coredump meta differs from topic, using topic")
	}

	if err := meta.validate(); err != nil {
		a.discard(k, "invalid coredump meta", err)
		return
	}

	p := a.partial(k)
	for n, chunk := range p.chunks {
		if err := checkChunk(&meta, n, chunk); err != nil {
			a.discard(k, "coredump chunk inconsistent with meta", err)
			return
		}
	}

	p.meta = &meta
	p.rawMeta = append([]byte(nil), payload...)
	a.tryComplete(k, p)
}

func (a *coredumpAssembler) handleData(k coredumpKey, n int, payload []byte) {
	if p := a.partials[k]; p != nil && p.meta != nil {
		if err := checkChunk(p.meta, n, payload); err != nil {
			a.discard(k, "coredump chunk inconsistent with meta", err)
			return
		}
	}

	p := a.partial(k)
	// Un duplicato sovrascrive (stesso contenuto): conta solo la differenza
	newBytes := p.bytes - len(p.chunks[n]) + len(payload)
	if newBytes > maxCoredumpSize {
		a.discard(k, "coredump too large", fmt.Errorf("%d bytes received, limit %d", newBytes, maxCoredumpSize))
		return
	}

	p.chunks[n] = append([]byte(nil), payload...)
	p.bytes = newBytes
	a.tryComplete(k, p)
}

// checkChunk verifica indice e lunghezza di un pezzo rispetto al meta
func checkChunk(meta *coredumpMeta, n int, chunk []byte) error {
	if n >= meta.Chunks {
		return fmt.Errorf("chunk %d >= chunks %d", n, meta.Chunks)
	}
	if len(chunk) > meta.ChunkSize {
		return fmt.Errorf("chunk %d is %d bytes, chunk_size %d", n, len(chunk), meta.ChunkSize)
	}
	return nil
}

// partial restituisce lo stato parziale di k, creandolo se serve
func (a *coredumpAssembler) partial(k coredumpKey) *coredumpPartial {
	if p := a.partials[k]; p != nil {
		return p
	}

	if len(a.partials) >= maxCoredumpPartials {
		a.evictOldest()
	}

	p := &coredumpPartial{
		chunks: make(map[int][]byte),
		first:  a.now(),
	}
	a.partials[k] = p
	return p
}

func (a *coredumpAssembler) evictOldest() {
	var oldest coredumpKey
	var oldestTime time.Time
	for k, p := range a.partials {
		if oldestTime.IsZero() || p.first.Before(oldestTime) {
			oldest, oldestTime = k, p.first
		}
	}

	log.Warn().
		Str("device", oldest.dev).
		Str("cid", oldest.cid).
		Int("limit", maxCoredumpPartials).
		Msg("Too many partial coredumps, discarding the oldest")
	delete(a.partials, oldest)
}

// discard scarta lo stato parziale di k: nessun ack, il device ripubblicherà
func (a *coredumpAssembler) discard(k coredumpKey, msg string, err error) {
	log.Error().
		Err(err).
		Str("device", k.dev).
		Str("cid", k.cid).
		Msg(msg)
	delete(a.partials, k)
}

// tryComplete salva e conferma il core dump se sono arrivati meta e tutti i pezzi
func (a *coredumpAssembler) tryComplete(k coredumpKey, p *coredumpPartial) {
	// I pezzi sono tutti < chunks (checkChunk), quindi basta contarli
	if p.meta == nil || len(p.chunks) < p.meta.Chunks {
		return
	}

	img := make([]byte, 0, p.bytes)
	for n := range p.meta.Chunks {
		img = append(img, p.chunks[n]...)
	}

	crc := fmt.Sprintf("%08x", crc32.ChecksumIEEE(img))
	if len(img) != p.meta.Size || crc != k.cid {
		log.Error().
			Str("device", k.dev).
			Str("cid", k.cid).
			Int("size_expected", p.meta.Size).
			Int("size_got", len(img)).
			Str("crc_expected", k.cid).
			Str("crc_got", crc).
			Msg("Coredump verification failed, discarded (no ack)")
		delete(a.partials, k)
		return
	}

	delete(a.partials, k)
	if err := a.saver.SaveCoredump(k.dev, k.cid, p.meta, p.rawMeta, img); err != nil {
		log.Error().
			Err(err).
			Str("device", k.dev).
			Str("cid", k.cid).
			Msg("Failed to save coredump (no ack)")
		return
	}

	a.saved[k] = true
	a.ack(k)
}

func (a *coredumpAssembler) ack(k coredumpKey) {
	a.lastAck[k] = a.now()
	a.acker.AckCoredump(k.dev, k.cid)
}

// ackThrottled ripubblica l'ack al più una volta ogni coredumpAckInterval:
// un reinvio di 60 pezzi non deve produrre 60 ack
func (a *coredumpAssembler) ackThrottled(k coredumpKey) {
	if last, ok := a.lastAck[k]; ok && a.now().Sub(last) < coredumpAckInterval {
		return
	}
	a.ack(k)
}

// Expire scarta gli stati parziali più vecchi di coredumpPartialTTL
func (a *coredumpAssembler) Expire() {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := a.now()
	for k, p := range a.partials {
		if now.Sub(p.first) < coredumpPartialTTL {
			continue
		}

		ev := log.Warn().
			Str("device", k.dev).
			Str("cid", k.cid).
			Int("chunks_received", len(p.chunks)).
			Bool("meta_received", p.meta != nil)
		if p.meta != nil {
			ev = ev.Int("chunks_missing", p.meta.Chunks-len(p.chunks))
		}
		ev.Msg("Partial coredump expired, discarded")
		delete(a.partials, k)
	}
}
