package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

const coredumpDecodeTimeout = 60 * time.Second

// fileCoredumpSaver salva i core dump in <dir>/<dev>/ e li registra nel database
type fileCoredumpSaver struct {
	dir    string // coredumpDir
	elfDir string // opzionale: ELF dell'applicazione per la decodifica
	db     *Database
	now    func() time.Time
}

func newFileCoredumpSaver(dir, elfDir string, db *Database) (*fileCoredumpSaver, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("coredump dir %s: %w", dir, err)
	}
	return &fileCoredumpSaver{dir: dir, elfDir: elfDir, db: db, now: time.Now}, nil
}

// SaveCoredump scrive <data>-<cid>-<elf_sha256>.elf e .json, poi la riga in
// tabella. Solo se tutto riesce restituisce nil (e parte l'ack).
func (s *fileCoredumpSaver) SaveCoredump(dev, cid string, meta *coredumpMeta, rawMeta, img []byte) error {
	devDir := filepath.Join(s.dir, dev)
	if err := os.MkdirAll(devDir, 0o755); err != nil {
		return fmt.Errorf("failed to create %s: %w", devDir, err)
	}

	sha := "unknown"
	if meta.ElfSHA256 != nil && *meta.ElfSHA256 != "" {
		sha = sanitizeFileComponent(*meta.ElfSHA256)
	}
	base := filepath.Join(devDir, fmt.Sprintf("%s-%s-%s",
		s.now().UTC().Format("20060102T150405Z"), cid, sha))
	elfPath := base + ".elf"

	if err := writeFileAtomic(elfPath, img); err != nil {
		return err
	}
	if err := writeFileAtomic(base+".json", rawMeta); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.db.InsertCoredump(ctx, dev, cid, elfPath, meta); err != nil {
		return err
	}
	if err := s.db.TouchDevice(ctx, dev); err != nil {
		log.Error().Err(err).Str("device", dev).Msg("Failed to update device last_seen")
	}

	ev := log.Info().
		Str("device", dev).
		Str("cid", cid).
		Int("size", meta.Size).
		Str("path", elfPath)
	if meta.PanicReason != nil {
		ev = ev.Str("panic_reason", *meta.PanicReason)
	}
	ev.Msg("Coredump saved")

	if s.elfDir != "" && sha != "unknown" {
		go s.decode(elfPath, base+".txt", sha)
	}
	return nil
}

// decode lancia esp-coredump se è disponibile l'ELF dell'applicazione
// corrispondente. Best effort: errori solo a log, mai blocco dell'ack.
func (s *fileCoredumpSaver) decode(corePath, outPath, sha string) {
	matches, _ := filepath.Glob(filepath.Join(s.elfDir, "Termostato-"+sha+"*.elf"))
	if len(matches) == 0 {
		log.Debug().Str("elf_sha256", sha).Msg("No application ELF for coredump decoding")
		return
	}
	tool, err := exec.LookPath("esp-coredump")
	if err != nil {
		log.Warn().Msg("esp-coredump not installed, coredump not decoded")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), coredumpDecodeTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, tool, "info_corefile", "-c", corePath, "-t", "raw", matches[0]).CombinedOutput()
	if werr := writeFileAtomic(outPath, out); werr != nil {
		log.Warn().Err(werr).Msg("Failed to write coredump decode output")
		return
	}
	if err != nil {
		log.Warn().Err(err).Str("output", outPath).Msg("esp-coredump failed")
		return
	}
	log.Info().Str("output", outPath).Msg("Coredump decoded")
}

// sanitizeFileComponent rende un campo del payload sicuro come parte di un
// nome di file (elf_sha256 è una stringa opaca che arriva dal device)
func sanitizeFileComponent(s string) string {
	return strings.Map(func(r rune) rune {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			return r
		}
		return '_'
	}, s)
}

// writeFileAtomic scrive su un file temporaneo nella stessa directory e poi
// lo rinomina: un crash a metà non lascia file troncati
func writeFileAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file for %s: %w", path, err)
	}
	defer os.Remove(tmp.Name()) // no-op dopo il rename

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("failed to sync %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to close %s: %w", path, err)
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return fmt.Errorf("failed to chmod %s: %w", path, err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("failed to rename %s: %w", path, err)
	}
	return nil
}
