package main

import (
	"reflect"
	"testing"
)

func TestToStringSlice_Valid(t *testing.T) {
	input := []interface{}{"a", "b", "c"}
	got := toStringSlice(input)
	want := []string{"a", "b", "c"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("toStringSlice(%v) = %v, want %v", input, got, want)
	}
}

func TestToStringSlice_Nil(t *testing.T) {
	got := toStringSlice(nil)
	if got != nil {
		t.Errorf("toStringSlice(nil) = %v, want nil", got)
	}
}

func TestToStringSlice_NonSlice(t *testing.T) {
	got := toStringSlice("string")
	if got != nil {
		t.Errorf("toStringSlice(\"string\") = %v, want nil", got)
	}
}

func TestToStringSlice_MixedTypes(t *testing.T) {
	input := []interface{}{"a", 42, "b", true, "c"}
	got := toStringSlice(input)
	want := []string{"a", "b", "c"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("toStringSlice(%v) = %v, want %v (strings only)", input, got, want)
	}
}

func TestToStringSlice_EmptySlice(t *testing.T) {
	input := []interface{}{}
	got := toStringSlice(input)

	if len(got) != 0 {
		t.Errorf("toStringSlice(%v) = %v, want empty slice", input, got)
	}
}

func TestToSet(t *testing.T) {
	input := []string{"a", "b", "c"}
	got := toSet(input)

	for _, s := range input {
		if !got[s] {
			t.Errorf("toSet(%v)[%q] = false, want true", input, s)
		}
	}

	if got["d"] {
		t.Error("toSet should not contain 'd'")
	}
}

func TestStringField(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]interface{}
		key  string
		want string
	}{
		{
			name: "found",
			m:    map[string]interface{}{"key": "value"},
			key:  "key",
			want: "value",
		},
		{
			name: "not found",
			m:    map[string]interface{}{"other": "value"},
			key:  "key",
			want: "",
		},
		{
			name: "nil map",
			m:    nil,
			key:  "key",
			want: "",
		},
		{
			name: "non-string value",
			m:    map[string]interface{}{"key": 42},
			key:  "key",
			want: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := stringField(tc.m, tc.key)
			if got != tc.want {
				t.Errorf("stringField(%v, %q) = %q, want %q", tc.m, tc.key, got, tc.want)
			}
		})
	}
}
