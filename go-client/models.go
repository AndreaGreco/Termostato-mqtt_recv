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
	Db             DatabaseConfig `yaml:"database"`
	Mqtt           MqttConfig     `yaml:"mqtt"`
	MeasureTopic   string         `yaml:"measureTopic"`
	OpenThermTopic string         `yaml:"OpenThermTopic"`
}
