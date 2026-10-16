package main

import (
	"context"
	"fmt"
	"math"
	"mqtt_client/internal/db"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// Database gestisce la connessione al database PostgreSQL
type Database struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

// NewDatabase inizializza una nuova connessione al database
func NewDatabase(ctx context.Context, config *DatabaseConfig) (*Database, error) {
	connString := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		config.User,
		config.Password,
		config.Host,
		config.Port,
		config.Dbname)

	poolConfig, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("unable to parse connection string: %w", err)
	}

	// Configure pool settings
	poolConfig.MaxConns = 10
	poolConfig.MinConns = 2
	poolConfig.MaxConnLifetime = time.Hour
	poolConfig.MaxConnIdleTime = 30 * time.Minute
	poolConfig.HealthCheckPeriod = time.Minute

	// Retry connection with exponential backoff
	var pool *pgxpool.Pool
	maxRetries := 10
	for i := 0; i < maxRetries; i++ {
		pool, err = pgxpool.NewWithConfig(ctx, poolConfig)
		if err == nil {
			// Test the connection
			if err = pool.Ping(ctx); err == nil {
				log.Info().Msg("Successfully connected to database")
				return &Database{
					pool:    pool,
					queries: db.New(pool),
				}, nil
			}
			pool.Close()
		}

		waitTime := time.Duration(i+1) * 2 * time.Second
		log.Warn().
			Int("attempt", i+1).
			Int("maxRetries", maxRetries).
			Err(err).
			Dur("retryIn", waitTime).
			Msg("Failed to connect to database")

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(waitTime):
		}
	}

	return nil, fmt.Errorf("failed to connect to database after %d attempts: %w", maxRetries, err)
}

// Close chiude la connessione al database
func (db *Database) Close() {
	if db.pool != nil {
		db.pool.Close()
		log.Info().Msg("Database connection closed")
	}
}

// InsertMeasure inserisce un campione di misura nel database
func (d *Database) InsertMeasure(ctx context.Context, device string, sample SampleSt) error {
	sampleTime := time.UnixMilli(int64(sample.Timestamp * 1000))
	diffTime := time.Until(sampleTime).Seconds()
	diff := math.Abs(diffTime)

	if diff > 60 {
		return fmt.Errorf("measure timestamp out of range: diff=%v seconds", diff)
	}

	// Converti time.Time in pgtype.Timestamp
	pgTimestamp := pgtype.Timestamp{
		Time:  sampleTime,
		Valid: true,
	}

	params := db.InsertMeasureParams{
		AmbTemp:          sample.AmbTemp,
		TempSensA:        sample.TempSensA,
		TempSensB:        sample.TempSensB,
		TempHot:          sample.TempHot,
		TempOffsetA:      sample.TempOffsetA,
		TempOffsetB:      sample.TempOffsetB,
		DiagA:            int32(sample.DiagA),
		DiagB:            int32(sample.DiagB),
		AmbientSetpoint:  sample.AmbientSetpoint,
		HeapFree:         int32(sample.HeapFree),
		HeapLargestBlock: int32(sample.HeapLargestBlock),
		BootCount:        int32(sample.BootCount),
		Time:             pgTimestamp,
		Device:           device,
	}

	err := d.queries.InsertMeasure(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to insert measure: %w", err)
	}

	log.Info().
		Str("device", device).
		Float64("temp", sample.AmbTemp).
		Float64("setpoint", sample.AmbientSetpoint).
		Int("heap_free", sample.HeapFree).
		Int("heap_largest_block", sample.HeapLargestBlock).
		Int("boot_count", sample.BootCount).
		Msg("Inserted measure data")
	return nil
}

// InsertOpenTherm inserisce dati OpenTherm nel database
func (d *Database) InsertOpenTherm(ctx context.Context, device string, otSample OtDataSt) error {
	sampleTime := time.UnixMilli(int64(otSample.Timestamp * 1000))
	diffTime := time.Until(sampleTime).Seconds()
	diff := math.Abs(diffTime)

	if diff > 60 {
		return fmt.Errorf("opentherm timestamp out of range: diff=%v seconds", diff)
	}

	// Converti time.Time in pgtype.Timestamp
	pgTimestamp := pgtype.Timestamp{
		Time:  sampleTime,
		Valid: true,
	}

	params := db.InsertOpenThermParams{
		HeatingWaterTempRead:     otSample.HeatingWaterTempRead,
		HeatingSetpointRead:      otSample.HeatingSetpointRead,
		ModulationLvl:            otSample.ModulationLvl,
		DhwTemperature:           otSample.DhwTemperature,
		DhwSetpoint:              otSample.DhwSetpoint,
		FlameOn:                  otSample.FlameOn,
		HeatingOn:                otSample.HeatingOn,
		DhwOn:                    otSample.DhwOn,
		ReturnWaterTemp:          otSample.ReturnWaterTemp,
		ChWaterPressure:          otSample.ChWaterPressure,
		OutsideTemperature:       otSample.OutsideTemperature,
		ExhaustTemperature:       otSample.ExhaustTemperature,
		HeatExchangerTemperature: otSample.HeatExchangerTemperature,
		FaultFlags:               int32(otSample.FaultFlags),
		OemFaultCode:             int32(otSample.OemFaultCode),
		OemDiagnostic:            int32(otSample.OemDiagnostic),
		MaxBoilerCapacityKw:      int32(otSample.MaxBoilerCapacityKw),
		MinModulationLevel:       int32(otSample.MinModulationLevel),
		BurnerStarts:             int32(otSample.BurnerStarts),
		BurnerOperationHours:     int32(otSample.BurnerOperationHours),
		ChPumpStarts:             int32(otSample.ChPumpStarts),
		ChPumpOperationHours:     int32(otSample.ChPumpOperationHours),
		DhwBurnerStarts:          int32(otSample.DhwBurnerStarts),
		DhwBurnerOperationHours:  int32(otSample.DhwBurnerOperationHours),
		ChSetpointMin:            otSample.ChSetpointMin,
		ChSetpointMax:            otSample.ChSetpointMax,
		MaxChSetpoint:            otSample.MaxChSetpoint,
		DhwSetpointMin:           otSample.DhwSetpointMin,
		DhwSetpointMax:           otSample.DhwSetpointMax,
		DhwFlowRate:              otSample.DhwFlowRate,
		Time:                     pgTimestamp,
		Device:                   device,
	}

	err := d.queries.InsertOpenTherm(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to insert opentherm data: %w", err)
	}

	log.Info().
		Str("device", device).
		Float64("heating", otSample.HeatingWaterTempRead).
		Bool("flame", otSample.FlameOn).
		Float64("modulation", otSample.ModulationLvl).
		Msg("Inserted OpenTherm data")
	return nil
}

// InsertLog inserisce un messaggio di log ESP32 nel database
func (d *Database) InsertLog(ctx context.Context, device string, entry EspLogSt) error {
	logTime := time.UnixMilli(int64(entry.Timestamp * 1000))

	// I log emessi prima della sync SNTP hanno un timestamp non valido:
	// invece di scartarli (servono per il debug del boot) usa l'ora di ricezione
	if math.Abs(time.Since(logTime).Seconds()) > 60 {
		log.Debug().
			Float64("t", entry.Timestamp).
			Str("task", entry.Task).
			Msg("Log timestamp out of range, using receive time")
		logTime = time.Now()
	}

	params := db.InsertLogParams{
		Time:   pgtype.Timestamp{Time: logTime, Valid: true},
		Task:   entry.Task,
		Msg:    entry.Msg,
		Device: device,
	}

	if err := d.queries.InsertLog(ctx, params); err != nil {
		return fmt.Errorf("failed to insert log: %w", err)
	}

	log.Debug().
		Str("device", device).
		Str("task", entry.Task).
		Str("msg", entry.Msg).
		Msg("Inserted log entry")
	return nil
}

// UpsertDeviceInfo registra o aggiorna l'identità di un device (non tocca "name")
func (d *Database) UpsertDeviceInfo(ctx context.Context, device string, info DeviceInfoSt) error {
	params := db.UpsertDeviceInfoParams{
		ID:        device,
		Hostname:  info.Hostname,
		Ip:        info.IP,
		FwVersion: info.FwVersion,
		Partition: info.Partition,
	}

	if err := d.queries.UpsertDeviceInfo(ctx, params); err != nil {
		return fmt.Errorf("failed to upsert device info: %w", err)
	}

	log.Info().
		Str("device", device).
		Interface("info", info).
		Msg("Updated device info")
	return nil
}

// UpsertDeviceStatus aggiorna lo stato online/offline di un device
func (d *Database) UpsertDeviceStatus(ctx context.Context, device string, online bool) error {
	params := db.UpsertDeviceStatusParams{
		ID:     device,
		Online: online,
	}

	if err := d.queries.UpsertDeviceStatus(ctx, params); err != nil {
		return fmt.Errorf("failed to upsert device status: %w", err)
	}

	log.Info().
		Str("device", device).
		Bool("online", online).
		Msg("Updated device status")
	return nil
}

// TouchDevice aggiorna last_seen del device, creandolo se non esiste
func (d *Database) TouchDevice(ctx context.Context, device string) error {
	if err := d.queries.TouchDevice(ctx, device); err != nil {
		return fmt.Errorf("failed to touch device: %w", err)
	}
	return nil
}

// InsertCoredump registra un core dump salvato su disco (idempotente)
func (d *Database) InsertCoredump(ctx context.Context, dev, cid, path string, meta *coredumpMeta) error {
	params := db.InsertCoredumpParams{
		Device:      dev,
		ID:          cid,
		Size:        int32(meta.Size),
		ElfSha256:   meta.ElfSHA256,
		Task:        meta.Task,
		Pc:          meta.PC,
		PanicReason: meta.PanicReason,
		Backtrace:   meta.Backtrace,
		Path:        path,
	}

	if err := d.queries.InsertCoredump(ctx, params); err != nil {
		return fmt.Errorf("failed to insert coredump: %w", err)
	}
	return nil
}

// ListCoredumpKeys restituisce le coppie (device, id) dei core dump già salvati
func (d *Database) ListCoredumpKeys(ctx context.Context) ([]db.ListCoredumpKeysRow, error) {
	rows, err := d.queries.ListCoredumpKeys(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list coredumps: %w", err)
	}
	return rows, nil
}
