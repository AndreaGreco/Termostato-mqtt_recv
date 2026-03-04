-- Add ThermoPro sensor data table

CREATE TABLE "thermo_sensor" (
	time TIMESTAMPTZ NOT NULL,
	temperature DOUBLE PRECISION NOT NULL,
	humidity DOUBLE PRECISION NOT NULL,
	sensor_id VARCHAR(50) DEFAULT 'TP357S'
);

SELECT create_hypertable('thermo_sensor', 'time');

-- Create index for faster queries
CREATE INDEX idx_thermo_sensor_time ON thermo_sensor (time DESC);
CREATE INDEX idx_thermo_sensor_sensor_id ON thermo_sensor (sensor_id, time DESC);
