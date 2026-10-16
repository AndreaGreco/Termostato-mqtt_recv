package main

import (
	"encoding/json"
	"testing"
)

// I limiti e la portata ACS arrivano null finche' la caldaia non risponde:
// devono diventare nil (NULL in tabella), non 0.
func TestOtDataNullableBoilerFields(t *testing.T) {
	var ot OtDataSt
	payload := `{"timestamp":1,"ch_setpoint_min":null,"max_ch_setpoint":45,` +
		`"dhw_setpoint_min":37,"dhw_setpoint_max":60,"dhw_flow_rate":null}`
	if err := json.Unmarshal([]byte(payload), &ot); err != nil {
		t.Fatal(err)
	}
	if ot.ChSetpointMin != nil || ot.ChSetpointMax != nil || ot.DhwFlowRate != nil {
		t.Errorf("null/assenti devono restare nil")
	}
	if ot.MaxChSetpoint == nil || *ot.MaxChSetpoint != 45 {
		t.Errorf("max_ch_setpoint = %v, atteso 45", ot.MaxChSetpoint)
	}
	if ot.DhwSetpointMin == nil || *ot.DhwSetpointMin != 37 ||
		ot.DhwSetpointMax == nil || *ot.DhwSetpointMax != 60 {
		t.Errorf("dhw bounds = %v-%v, attesi 37-60", ot.DhwSetpointMin, ot.DhwSetpointMax)
	}
}

// L'esterna dal meteo e' null senza previsione, e assente dai firmware
// precedenti: in entrambi i casi nil, non 0 gradi.
func TestOtDataWeatherTemp(t *testing.T) {
	cases := []struct {
		payload string
		want    *float64
	}{
		{`{"timestamp":1,"weather_temp":-2.5}`, ptr(-2.5)},
		{`{"timestamp":1,"weather_temp":null}`, nil},
		{`{"timestamp":1}`, nil},
	}
	for _, c := range cases {
		var ot OtDataSt
		if err := json.Unmarshal([]byte(c.payload), &ot); err != nil {
			t.Fatal(err)
		}
		switch {
		case c.want == nil && ot.WeatherTemp != nil:
			t.Errorf("%s: weather_temp = %v, atteso nil", c.payload, *ot.WeatherTemp)
		case c.want != nil && (ot.WeatherTemp == nil || *ot.WeatherTemp != *c.want):
			t.Errorf("%s: weather_temp = %v, atteso %v", c.payload, ot.WeatherTemp, *c.want)
		}
	}
}

func ptr(v float64) *float64 { return &v }
