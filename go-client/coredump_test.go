package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"math/rand"
	"testing"
	"time"
)

type fakeSaver struct {
	saves []savedDump
	err   error
}

type savedDump struct {
	dev, cid string
	img      []byte
}

func (f *fakeSaver) SaveCoredump(dev, cid string, meta *coredumpMeta, rawMeta, img []byte) error {
	if f.err != nil {
		return f.err
	}
	f.saves = append(f.saves, savedDump{dev, cid, img})
	return nil
}

type fakeAcker struct{ acks []string }

func (f *fakeAcker) AckCoredump(dev, cid string) { f.acks = append(f.acks, dev+"/"+cid) }

type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time          { return c.t }
func (c *fakeClock) advance(d time.Duration) { c.t = c.t.Add(d) }

func newTestAssembler() (*coredumpAssembler, *fakeSaver, *fakeAcker, *fakeClock) {
	saver, acker := &fakeSaver{}, &fakeAcker{}
	clock := &fakeClock{t: time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)}
	a := newCoredumpAssembler(saver, acker)
	a.now = clock.now
	return a, saver, acker, clock
}

// testDump è un core dump finto spezzato come fa il firmware
type testDump struct {
	dev, cid string
	img      []byte
	chunks   [][]byte
	meta     []byte
}

func newTestDump(size, chunkSize int) testDump {
	img := make([]byte, size)
	rand.New(rand.NewSource(int64(size))).Read(img[:size-4])
	// Come in flash: in coda il CRC-32 del resto, little endian
	body := crc32.ChecksumIEEE(img[:size-4])
	binary.LittleEndian.PutUint32(img[size-4:], body)
	cid := fmt.Sprintf("%08x", body)

	d := testDump{dev: "test01", cid: cid, img: img}
	for off := 0; off < size; off += chunkSize {
		d.chunks = append(d.chunks, img[off:min(off+chunkSize, size)])
	}
	d.meta = []byte(fmt.Sprintf(
		`{"device":"test01","id":%q,"size":%d,"chunk_size":%d,"chunks":%d,"crc32":%q}`,
		cid, size, chunkSize, len(d.chunks), cid))
	return d
}

func (d testDump) sendChunk(a *coredumpAssembler, n int) {
	a.Handle(d.dev, d.cid, "data", n, d.chunks[n])
}

func (d testDump) sendMeta(a *coredumpAssembler) {
	a.Handle(d.dev, d.cid, "meta", 0, d.meta)
}

func assertSavedOnce(t *testing.T, saver *fakeSaver, acker *fakeAcker, d testDump) {
	t.Helper()
	if len(saver.saves) != 1 {
		t.Fatalf("saves = %d, want 1", len(saver.saves))
	}
	if string(saver.saves[0].img) != string(d.img) {
		t.Error("saved image differs from original")
	}
	if len(acker.acks) != 1 || acker.acks[0] != d.dev+"/"+d.cid {
		t.Errorf("acks = %v, want [%s/%s]", acker.acks, d.dev, d.cid)
	}
}

func TestCoredumpOrderings(t *testing.T) {
	d := newTestDump(5000, 1024) // 5 pezzi, l'ultimo corto

	orders := map[string]func(a *coredumpAssembler){
		"in order, meta last": func(a *coredumpAssembler) {
			for n := range d.chunks {
				d.sendChunk(a, n)
			}
			d.sendMeta(a)
		},
		"meta first, reversed": func(a *coredumpAssembler) {
			d.sendMeta(a)
			for n := len(d.chunks) - 1; n >= 0; n-- {
				d.sendChunk(a, n)
			}
		},
		"meta in the middle, shuffled": func(a *coredumpAssembler) {
			d.sendChunk(a, 3)
			d.sendChunk(a, 0)
			d.sendMeta(a)
			d.sendChunk(a, 4)
			d.sendChunk(a, 1)
			d.sendChunk(a, 2)
		},
	}

	for name, send := range orders {
		t.Run(name, func(t *testing.T) {
			a, saver, acker, _ := newTestAssembler()
			send(a)
			assertSavedOnce(t, saver, acker, d)
			if len(a.partials) != 0 {
				t.Errorf("partials = %d, want 0", len(a.partials))
			}
		})
	}
}

func TestCoredumpDuplicateChunksBeforeCompletion(t *testing.T) {
	a, saver, acker, _ := newTestAssembler()
	d := newTestDump(3000, 1024)

	d.sendChunk(a, 0)
	d.sendChunk(a, 0)
	d.sendChunk(a, 1)
	d.sendChunk(a, 1)
	if len(saver.saves) != 0 || len(acker.acks) != 0 {
		t.Fatal("saved or acked before completion")
	}
	if got := a.partials[coredumpKey{d.dev, d.cid}].bytes; got != 2048 {
		t.Errorf("partial bytes = %d, want 2048 (duplicates counted once)", got)
	}

	d.sendChunk(a, 2)
	d.sendMeta(a)
	assertSavedOnce(t, saver, acker, d)
}

func TestCoredumpMissingChunkExpires(t *testing.T) {
	a, saver, acker, clock := newTestAssembler()
	d := newTestDump(5000, 1024)

	d.sendMeta(a)
	for n := range d.chunks {
		if n != 2 {
			d.sendChunk(a, n)
		}
	}
	if len(saver.saves) != 0 || len(acker.acks) != 0 {
		t.Fatal("saved or acked with a missing chunk")
	}

	clock.advance(23 * time.Hour)
	a.Expire()
	if len(a.partials) != 1 {
		t.Fatal("partial expired before 24 h")
	}

	clock.advance(time.Hour)
	a.Expire()
	if len(a.partials) != 0 {
		t.Fatal("partial not expired after 24 h")
	}

	// Dopo la scadenza un reinvio completo ricomincia da zero e va a buon fine
	d.sendMeta(a)
	for n := range d.chunks {
		d.sendChunk(a, n)
	}
	assertSavedOnce(t, saver, acker, d)
}

func TestCoredumpID(t *testing.T) {
	a, b := newTestDump(3000, 1024), newTestDump(5000, 1024)

	// Il CRC dell'immagine intera è la costante del CRC-32: non va usato
	for _, d := range []testDump{a, b} {
		if got := fmt.Sprintf("%08x", crc32.ChecksumIEEE(d.img)); got != "2144df1c" {
			t.Errorf("full image crc = %s, want residue 2144df1c", got)
		}
	}
	if coredumpID(a.img) != a.cid || coredumpID(b.img) != b.cid {
		t.Error("coredumpID differs from the trailing checksum")
	}
	if a.cid == b.cid {
		t.Errorf("two different dumps share id %s", a.cid)
	}
	if coredumpID([]byte{1, 2, 3, 4}) != "" {
		t.Error("image too short must have no id")
	}
}

func TestCoredumpVerificationFailures(t *testing.T) {
	t.Run("wrong crc", func(t *testing.T) {
		a, saver, acker, _ := newTestAssembler()
		d := newTestDump(3000, 1024)

		bad := append([]byte(nil), d.chunks[1]...)
		bad[10] ^= 0xff
		d.sendMeta(a)
		d.sendChunk(a, 0)
		a.Handle(d.dev, d.cid, "data", 1, bad)
		d.sendChunk(a, 2)

		if len(saver.saves) != 0 || len(acker.acks) != 0 {
			t.Error("corrupted dump saved or acked")
		}
		if len(a.partials) != 0 {
			t.Error("partial state not cleared")
		}
	})

	t.Run("wrong size", func(t *testing.T) {
		a, saver, acker, _ := newTestAssembler()
		d := newTestDump(3000, 1024)

		// size 2900 è coerente con 3 pezzi, ma l'immagine ricomposta è 3000
		a.Handle(d.dev, d.cid, "meta", 0, []byte(fmt.Sprintf(
			`{"device":"test01","id":%q,"size":2900,"chunk_size":1024,"chunks":3,"crc32":%q}`, d.cid, d.cid)))
		for n := range d.chunks {
			d.sendChunk(a, n)
		}

		if len(saver.saves) != 0 || len(acker.acks) != 0 {
			t.Error("dump with wrong size saved or acked")
		}
		if len(a.partials) != 0 {
			t.Error("partial state not cleared")
		}
	})

	t.Run("save error", func(t *testing.T) {
		a, saver, acker, _ := newTestAssembler()
		saver.err = errors.New("insert failed")
		d := newTestDump(3000, 1024)

		d.sendMeta(a)
		for n := range d.chunks {
			d.sendChunk(a, n)
		}
		if len(acker.acks) != 0 {
			t.Error("acked although save failed")
		}
		if a.saved[coredumpKey{d.dev, d.cid}] {
			t.Error("marked saved although save failed")
		}
	})
}

func TestCoredumpRedeliveryAfterSave(t *testing.T) {
	a, saver, acker, clock := newTestAssembler()
	d := newTestDump(5000, 1024)

	d.sendMeta(a)
	for n := range d.chunks {
		d.sendChunk(a, n)
	}
	assertSavedOnce(t, saver, acker, d)

	// Reinvio completo subito dopo: niente riscrittura, ack non ripetuto per ogni pezzo
	clock.advance(time.Second)
	for n := range d.chunks {
		d.sendChunk(a, n)
	}
	d.sendMeta(a)
	if len(saver.saves) != 1 {
		t.Errorf("saves = %d after redelivery, want 1", len(saver.saves))
	}
	if len(acker.acks) != 1 {
		t.Errorf("acks = %d within ack interval, want 1", len(acker.acks))
	}

	// Reinvio successivo (es. dopo 12 h): l'ack torna, una volta sola
	clock.advance(12 * time.Hour)
	for n := range d.chunks {
		d.sendChunk(a, n)
	}
	if len(acker.acks) != 2 {
		t.Errorf("acks = %d after later redelivery, want 2", len(acker.acks))
	}
	if len(a.partials) != 0 {
		t.Error("partial state created for an already saved dump")
	}
}

func TestCoredumpAlreadySavedAtStartup(t *testing.T) {
	a, saver, acker, _ := newTestAssembler()
	d := newTestDump(3000, 1024)

	a.MarkSaved(d.dev, d.cid)
	d.sendChunk(a, 0)
	d.sendChunk(a, 1)
	if len(saver.saves) != 0 {
		t.Error("already saved dump rewritten")
	}
	if len(acker.acks) != 1 {
		t.Errorf("acks = %d, want 1", len(acker.acks))
	}
}

func TestCoredumpEmptyPayloadIgnored(t *testing.T) {
	a, saver, acker, _ := newTestAssembler()
	d := newTestDump(3000, 1024)

	a.Handle(d.dev, d.cid, "data", 0, nil)
	a.Handle(d.dev, d.cid, "meta", 0, []byte{})
	if len(a.partials) != 0 {
		t.Error("empty payload created partial state")
	}

	// Anche su un dump già salvato: la ripulitura del device non produce ack
	a.MarkSaved(d.dev, d.cid)
	a.Handle(d.dev, d.cid, "data", 0, nil)
	if len(saver.saves) != 0 || len(acker.acks) != 0 {
		t.Error("empty payload saved or acked")
	}
}

func TestCoredumpLimits(t *testing.T) {
	cases := map[string]string{
		"size over 1 MB":        `{"size":1048577,"chunk_size":2048,"chunks":513}`,
		"chunks over 1024":      `{"size":1025,"chunk_size":1,"chunks":1025}`,
		"chunks inconsistent":   `{"size":5000,"chunk_size":1024,"chunks":4}`,
		"chunk_size zero":       `{"size":5000,"chunk_size":0,"chunks":5}`,
		"size zero":             `{"size":0,"chunk_size":1024,"chunks":0}`,
		"chunk over chunk_size": `{"size":2000,"chunk_size":1000,"chunks":2}`,
	}

	for name, fields := range cases {
		t.Run(name, func(t *testing.T) {
			a, saver, acker, _ := newTestAssembler()
			d := newTestDump(1500, 1024) // pezzo 0 da 1024 byte

			d.sendChunk(a, 0)
			meta := fmt.Sprintf(`{"device":"test01","id":%q,"crc32":%q,%s`, d.cid, d.cid, fields[1:])
			a.Handle(d.dev, d.cid, "meta", 0, []byte(meta))

			if len(a.partials) != 0 {
				t.Error("partial state kept after invalid meta")
			}
			if len(saver.saves) != 0 || len(acker.acks) != 0 {
				t.Error("saved or acked with invalid meta")
			}
		})
	}

	t.Run("chunk index beyond meta", func(t *testing.T) {
		a, _, _, _ := newTestAssembler()
		d := newTestDump(3000, 1024)

		d.sendMeta(a)
		a.Handle(d.dev, d.cid, "data", 3, []byte{1})
		if len(a.partials) != 0 {
			t.Error("partial state kept after chunk beyond chunks")
		}
	})
}

func TestCoredumpMaxPartials(t *testing.T) {
	a, _, _, clock := newTestAssembler()

	for i := range maxCoredumpPartials + 1 {
		a.Handle("test01", fmt.Sprintf("%08x", i), "data", 0, []byte{1})
		clock.advance(time.Minute)
	}

	if len(a.partials) != maxCoredumpPartials {
		t.Errorf("partials = %d, want %d", len(a.partials), maxCoredumpPartials)
	}
	if _, ok := a.partials[coredumpKey{"test01", "00000000"}]; ok {
		t.Error("oldest partial not evicted")
	}
}
