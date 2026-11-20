package main

import (
	"errors"
	"testing"
)

func TestSamplerResult(t *testing.T) {
	for _, test := range []struct {
		name   string
		data   string
		status string
		hex    string
	}{
		{"picked", `{"status":"picked","red":0.07450980392156863,"green":0.08627450980392157,"blue":0.10588235294117647,"x":-100,"y":25}`, "picked", "#13161B"},
		{"clamped", `{"status":"picked","red":1.2,"green":-0.1,"blue":0.5,"x":0,"y":0}`, "picked", "#FF0080"},
		{"cancelled", `{"status":"cancelled"}`, "cancelled", ""},
		{"permission", `{"status":"permission"}`, "permission", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			picked, err := decodeSample([]byte(test.data))
			if err != nil || picked.Status != test.status {
				t.Fatalf("decodeSample = %+v, %v; want %s", picked, err, test.status)
			}
			if test.hex != "" && picked.color().Hex() != test.hex {
				t.Fatalf("color = %s; want %s", picked.color().Hex(), test.hex)
			}
			if test.name == "clamped" && !picked.outsideSRGB() {
				t.Fatal("extended sample was not marked outside sRGB")
			}
		})
	}
}

func TestSamplerRejectsIncompleteResult(t *testing.T) {
	for _, data := range []string{
		`{}`, `{"status":"picked"}`, `{"status":"unknown"}`,
		`{"status":"picked","red":null,"green":0,"blue":0,"x":0,"y":0}`,
		`{"status":"picked","red":0,"green":0,"blue":0,"x":0}`,
		`{"status":"picked","red":0,"green":0,"blue":0,"x":1e400,"y":0}`,
		`{"status":"picked","red":"0","green":0,"blue":0,"x":0,"y":0}`,
		`{"status":"cancelled"} {"status":"picked"}`,
	} {
		if _, err := decodeSample([]byte(data)); !errors.Is(err, errSamplerProtocol) {
			t.Errorf("decodeSample(%s) error = %v; want protocol error", data, err)
		}
	}
}

func TestPermissionStatus(t *testing.T) {
	for _, test := range []struct {
		data    string
		granted bool
		valid   bool
	}{
		{`{"status":"ready"}`, true, true},
		{`{"status":"permission"}`, false, true},
		{`{"status":"picked"}`, false, false},
		{`{"status":"unknown"}`, false, false},
		{`{}`, false, false},
		{`not json`, false, false},
	} {
		granted, err := decodePermissionStatus([]byte(test.data))
		if (err == nil) != test.valid || granted != test.granted {
			t.Errorf("decodePermissionStatus(%s) = %v, %v; want %v, valid %v", test.data, granted, err, test.granted, test.valid)
		}
	}
}
