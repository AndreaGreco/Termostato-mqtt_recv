package main

// SampleSt rappresenta i dati dei sensori di temperatura
type SampleSt struct {
	AmbTemp          float64 `json:"amb_temp"`
	TempSensA        float64 `json:"temp_sens_a"`
	TempSensB        float64 `json:"temp_sens_b"`
	TempOffsetA      float64 `json:"temp_offset_a"`
	TempOffsetB      float64 `json:"temp_offset_b"`
	TempHot          float64 `json:"temp_sens_hot"`
	DiagA            int     `json:"diag_a"`
	DiagB            int     `json:"diag_b"`
	AmbientSetpoint  float64 `json:"ambient_setpoint"`
	Power            float64 `json:"power_mw"`
	HeapFree         int     `json:"heap_free"`
	HeapLargestBlock int     `json:"heap_largest_block"`
	BootCount        int     `json:"boot_count"`
	Timestamp        float64 `json:"timestamp"`
}

// OtDataSt rappresenta i dati OpenTherm della caldaia
type OtDataSt struct {
	Timestamp            float64 `json:"timestamp"`
	HeatingWaterTempRead float64 `json:"heating_water_temp_read"`
	HeatingSetpointRead  float64 `json:"heating_setpoint_read"`
	ModulationLvl        float64 `json:"modulation_lvl"`
	DhwTemperature       float64 `json:"dhw_temperature"`
	DhwSetpoint          float64 `json:"dhw_setpoint"`
	FlameOn              bool    `json:"flame_on"`
	HeatingOn            bool    `json:"heating_on"`
	DhwOn                bool    `json:"dhw_on"`
	// Sensor & Informational
	ReturnWaterTemp          float64 `json:"return_water_temp"`
	ChWaterPressure          float64 `json:"ch_water_pressure"`
	OutsideTemperature       float64 `json:"outside_temperature"`
	ExhaustTemperature       float64 `json:"exhaust_temperature"`
	HeatExchangerTemperature float64 `json:"heat_exchanger_temperature"`
	// Fault & Diagnostics
	FaultFlags    int `json:"fault_flags"`
	OemFaultCode  int `json:"oem_fault_code"`
	OemDiagnostic int `json:"oem_diagnostic"`
	// Boiler capacity
	MaxBoilerCapacityKw int `json:"max_boiler_capacity_kw"`
	MinModulationLevel  int `json:"min_modulation_level"`
	// Operating hours & starts
	BurnerStarts            int `json:"burner_starts"`
	BurnerOperationHours    int `json:"burner_operation_hours"`
	ChPumpStarts            int `json:"ch_pump_starts"`
	ChPumpOperationHours    int `json:"ch_pump_operation_hours"`
	DhwBurnerStarts         int `json:"dhw_burner_starts"`
	DhwBurnerOperationHours int `json:"dhw_burner_operation_hours"`
}

// EspLogSt rappresenta un messaggio di log dell'ESP32
type EspLogSt struct {
	Timestamp float64 `json:"t"`
	Task      string  `json:"task"`
	Msg       string  `json:"msg"`
}

// DeviceInfoSt rappresenta il payload retained di <prefix><id>/info.
// I campi mancanti restano nil e finiscono NULL nel database.
type DeviceInfoSt struct {
	ID        *string `json:"id"`
	Hostname  *string `json:"hostname"`
	IP        *string `json:"ip"`
	FwVersion *string `json:"fw_version"`
	Partition *string `json:"partition"`
}

// DatabaseConfig contiene la configurazione del database
type DatabaseConfig struct {
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Host     string `yaml:"host"`
	Dbname   string `yaml:"dbname"`
	Port     int    `yaml:"port"`
}

// MqttConfig contiene la configurazione MQTT
type MqttConfig struct {
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
}

// Config contiene la configurazione completa dell'applicazione
type Config struct {
	Db           DatabaseConfig `yaml:"database"`
	Mqtt         MqttConfig     `yaml:"mqtt"`
	TopicPrefix  string         `yaml:"topicPrefix"`
	LegacyTopics *bool          `yaml:"legacyTopics"` // nil = default true
}
