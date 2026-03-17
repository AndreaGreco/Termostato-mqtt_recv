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
func (d *Database) InsertMeasure(ctx context.Context, sample SampleSt) error {
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
		Power:            sample.Power,
		HeapFree:         int32(sample.HeapFree),
		HeapLargestBlock: int32(sample.HeapLargestBlock),
		BootCount:        int32(sample.BootCount),
		Time:             pgTimestamp,
	}

	err := d.queries.InsertMeasure(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to insert measure: %w", err)
	}

	log.Info().
		Float64("temp", sample.AmbTemp).
		Float64("setpoint", sample.AmbientSetpoint).
		Int("heap_free", sample.HeapFree).
		Int("heap_largest_block", sample.HeapLargestBlock).
		Int("boot_count", sample.BootCount).
		Msg("Inserted measure data")
	return nil
}

// InsertOpenTherm inserisce dati OpenTherm nel database
func (d *Database) InsertOpenTherm(ctx context.Context, otSample OtDataSt) error {
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
		Time:                     pgTimestamp,
	}

	err := d.queries.InsertOpenTherm(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to insert opentherm data: %w", err)
	}

	log.Info().
		Float64("heating", otSample.HeatingWaterTempRead).
		Bool("flame", otSample.FlameOn).
		Float64("modulation", otSample.ModulationLvl).
		Msg("Inserted OpenTherm data")
	return nil
}
