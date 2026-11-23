package helper

import (
	"reflect"
	"testing"
	"time"
)

func TestParseStringOrDefaultAndInt(t *testing.T) {
	config := map[string]interface{}{
		"name": "proxy",
		"port": 8080,
	}

	if got := ParseStringOrDefault(config, "name", "default"); got != "proxy" {
		t.Errorf("ParseStringOrDefault(name) = %q, want proxy", got)
	}
	if got := ParseStringOrDefault(config, "missing", "default"); got != "default" {
		t.Errorf("ParseStringOrDefault(missing) = %q, want default", got)
	}
	if got := ParseStringOrDefault(config, "port", "default"); got != "default" {
		t.Errorf("ParseStringOrDefault(port) = %q, want default", got)
	}
	if got := ParseInt(config, "port", 80); got != 8080 {
		t.Errorf("ParseInt(port) = %d, want 8080", got)
	}
	if got := ParseInt(config, "missing", 80); got != 80 {
		t.Errorf("ParseInt(missing) = %d, want 80", got)
	}
	if got := ParseInt(map[string]interface{}{"port": int64(8080)}, "port", 80); got != 80 {
		t.Errorf("ParseInt(int64) = %d, want default 80", got)
	}
}

func TestParseStringMap(t *testing.T) {
	got, err := ParseStringMap(map[string]interface{}{
		"headers": map[string]interface{}{"X-Proxy": "gorp", "X-Version": "1"},
	}, "headers")
	if err != nil {
		t.Fatalf("ParseStringMap() error = %v", err)
	}
	want := map[string]string{"X-Proxy": "gorp", "X-Version": "1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseStringMap() = %v, want %v", got, want)
	}

	for _, config := range []map[string]interface{}{
		{},
		{"headers": "not a map"},
		{"headers": map[string]interface{}{"X-Proxy": 1}},
	} {
		if got, err := ParseStringMap(config, "headers"); err == nil || got != nil {
			t.Errorf("ParseStringMap(%v) = %v, %v; want nil and error", config, got, err)
		}
	}
}

func TestParseStringSlice(t *testing.T) {
	got, err := ParseStringSlice(map[string]interface{}{
		"methods": []interface{}{"GET", "POST"},
	}, "methods")
	if err != nil {
		t.Fatalf("ParseStringSlice() error = %v", err)
	}
	want := []string{"GET", "POST"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseStringSlice() = %v, want %v", got, want)
	}

	if got, err := ParseStringSlice(map[string]interface{}{"methods": []interface{}{"GET", 2}}, "methods"); err == nil || got != nil {
		t.Fatalf("ParseStringSlice(invalid item) = %v, %v; want nil and error", got, err)
	}
	if got, err := ParseStringSlice(map[string]interface{}{"methods": "GET"}, "methods"); err == nil || got != nil {
		t.Fatalf("ParseStringSlice(invalid field) = %v, %v; want nil and error", got, err)
	}
}

func TestParseDuration(t *testing.T) {
	const fallback = 5 * time.Second
	got, err := ParseDuration(map[string]interface{}{"timeout": "250ms"}, "timeout", fallback)
	if err != nil || got != 250*time.Millisecond {
		t.Fatalf("ParseDuration(valid) = %v, %v; want 250ms, nil", got, err)
	}
	got, err = ParseDuration(map[string]interface{}{}, "timeout", fallback)
	if err != nil || got != fallback {
		t.Fatalf("ParseDuration(missing) = %v, %v; want %v, nil", got, err, fallback)
	}
	if _, err := ParseDuration(map[string]interface{}{"timeout": "soon"}, "timeout", fallback); err == nil {
		t.Fatal("ParseDuration(invalid) returned no error")
	}
}

func TestParseBool(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		fallback bool
		want     bool
	}{
		{name: "bool true", value: true, want: true},
		{name: "bool false", value: false, fallback: true},
		{name: "yes", value: "YeS", want: true},
		{name: "on", value: "ON", want: true},
		{name: "false", value: "false"},
		{name: "off", value: "off", fallback: true},
		{name: "unknown string", value: "sometimes", fallback: true, want: true},
		{name: "unsupported type", value: 1, fallback: true, want: true},
		{name: "missing", fallback: true, want: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := ParseBool(map[string]interface{}{"enabled": test.value}, "enabled", test.fallback)
			if got != test.want {
				t.Fatalf("ParseBool() = %t, want %t", got, test.want)
			}
		})
	}
}
