package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDeviceFromTopic(t *testing.T) {
	tests := []struct {
		name     string
		prefix   string
		topic    string
		wantID   string
		wantLeaf string
		wantOK   bool
	}{
		{"device topic", "termostato/", "termostato/a1b2c3/sample", "a1b2c3", "sample", true},
		{"prefix without slash", "termostato", "termostato/a1b2c3/logs", "a1b2c3", "logs", true},
		{"name as id", "termostato/", "termostato/banco-test/info", "banco-test", "info", true},
		{"legacy topic", "termostato/", "termostato/sample", "", "", false},
		{"empty id", "termostato/", "termostato//sample", "", "", false},
		{"extra level", "termostato/", "termostato/a/b/sample", "", "", false},
		{"coredump subtree", "termostato/", "termostato/a1b2c3/coredump/0", "", "", false},
		{"empty leaf", "termostato/", "termostato/a1b2c3/", "", "", false},
		{"other prefix", "termostato/", "altro/a1b2c3/sample", "", "", false},
		{"reserved legacy id", "termostato/", "termostato/legacy/sample", "", "", false},
		{"id max length", "termostato/", "termostato/" + strings.Repeat("x", 32) + "/sample", strings.Repeat("x", 32), "sample", true},
		{"id too long", "termostato/", "termostato/" + strings.Repeat("x", 33) + "/sample", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, leaf, ok := deviceFromTopic(tt.prefix, tt.topic)
			if id != tt.wantID || leaf != tt.wantLeaf || ok != tt.wantOK {
				t.Errorf("deviceFromTopic(%q, %q) = (%q, %q, %v), want (%q, %q, %v)",
					tt.prefix, tt.topic, id, leaf, ok, tt.wantID, tt.wantLeaf, tt.wantOK)
			}
		})
	}
}

func TestNormalizeTopicPrefix(t *testing.T) {
	for in, want := range map[string]string{
		"termostato":  "termostato/",
		"termostato/": "termostato/",
		"casa/term":   "casa/term/",
		"":            "",
	} {
		if got := normalizeTopicPrefix(in); got != want {
			t.Errorf("normalizeTopicPrefix(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDeviceOfLegacy(t *testing.T) {
	mc := &MQTTClient{topics: &TopicConfig{Prefix: "termostato/", Legacy: true}}

	if dev, ok := mc.deviceOf("termostato/sample"); !ok || dev != legacyDevice {
		t.Errorf("legacy topic: got (%q, %v), want (%q, true)", dev, ok, legacyDevice)
	}
	if dev, ok := mc.deviceOf("termostato/a1b2c3/sample"); !ok || dev != "a1b2c3" {
		t.Errorf("device topic: got (%q, %v), want (\"a1b2c3\", true)", dev, ok)
	}

	mc.topics.Legacy = false
	if _, ok := mc.deviceOf("termostato/sample"); ok {
		t.Error("legacy topic accepted with legacyTopics disabled")
	}
}

func TestParseStatus(t *testing.T) {
	tests := []struct {
		payload    string
		wantOnline bool
		wantOK     bool
	}{
		{"online", true, true},
		{"offline", false, true},
		{"ONLINE", false, false},
		{"1", false, false},
		{"", false, false},
	}

	for _, tt := range tests {
		online, ok := parseStatus([]byte(tt.payload))
		if online != tt.wantOnline || ok != tt.wantOK {
			t.Errorf("parseStatus(%q) = (%v, %v), want (%v, %v)",
				tt.payload, online, ok, tt.wantOnline, tt.wantOK)
		}
	}
}

func TestDeviceInfoMissingFields(t *testing.T) {
	var info DeviceInfoSt
	if err := json.Unmarshal([]byte(`{"id":"test01","hostname":"banco"}`), &info); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if info.Hostname == nil || *info.Hostname != "banco" {
		t.Errorf("hostname = %v, want \"banco\"", info.Hostname)
	}
	if info.IP != nil || info.FwVersion != nil || info.Partition != nil {
		t.Errorf("missing fields must stay nil (NULL): %+v", info)
	}
}
