package main

import (
	"os"
	"testing"
)

func TestSecureToken(t *testing.T) {
	t.Run("with seed", func(t *testing.T) {
		token := secureToken("test")
		if token == "" {
			t.Error("secureToken returned empty string")
		}
		if len(token) < 32 {
			t.Errorf("secureToken: got length %d, want >= 32", len(token))
		}
	})

	t.Run("without seed", func(t *testing.T) {
		token := secureToken("")
		if token == "" {
			t.Error("secureToken returned empty string")
		}
		if len(token) < 32 {
			t.Errorf("secureToken: got length %d, want >= 32", len(token))
		}
	})
}

func TestFlagStr(t *testing.T) {
	tests := []struct {
		args []string
		name string
		def  string
		want string
	}{
		{[]string{"--foo", "bar"}, "foo", "default", "bar"},
		{[]string{"-foo", "bar"}, "foo", "default", "bar"},
		{[]string{"--other", "value"}, "foo", "default", "default"},
		{[]string{}, "foo", "default", "default"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldArgs := os.Args
			defer func() { os.Args = oldArgs }()
			os.Args = append([]string{"cmd"}, tt.args...)

			got := flagStr(tt.name, tt.def)
			if *got != tt.want {
				t.Errorf("flagStr(%q, %q) = %q, want %q", tt.name, tt.def, *got, tt.want)
			}
		})
	}
}

func TestFlagBool(t *testing.T) {
	tests := []struct {
		args []string
		name string
		def  bool
		want bool
	}{
		{[]string{"--foo"}, "foo", false, true},
		{[]string{"-foo"}, "foo", false, true},
		{[]string{"--other"}, "foo", false, false},
		{[]string{}, "foo", false, false},
		{[]string{"--foo"}, "foo", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldArgs := os.Args
			defer func() { os.Args = oldArgs }()
			os.Args = append([]string{"cmd"}, tt.args...)

			got := flagBool(tt.name, tt.def)
			if *got != tt.want {
				t.Errorf("flagBool(%q, %v) = %v, want %v", tt.name, tt.def, *got, tt.want)
			}
		})
	}
}

func TestFlagInt(t *testing.T) {
	tests := []struct {
		args []string
		name string
		def  int
		want int
	}{
		{[]string{"--timeout", "30"}, "timeout", 5, 30},
		{[]string{"-timeout", "30"}, "timeout", 5, 30},
		{[]string{"--timeout", "invalid"}, "timeout", 5, 5},
		{[]string{"--other", "10"}, "timeout", 5, 5},
		{[]string{}, "timeout", 5, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldArgs := os.Args
			defer func() { os.Args = oldArgs }()
			os.Args = append([]string{"cmd"}, tt.args...)

			got := flagInt(tt.name, tt.def)
			if *got != tt.want {
				t.Errorf("flagInt(%q, %d) = %d, want %d", tt.name, tt.def, *got, tt.want)
			}
		})
	}
}

func TestFlagStr_NoValue(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"cmd", "--flag"}

	got := flagStr("flag", "default")
	if *got != "default" {
		t.Errorf("flagStr with no value = %q, want %q", *got, "default")
	}
}

func TestFlagInt_NoValue(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"cmd", "--count"}

	got := flagInt("count", 10)
	if *got != 10 {
		t.Errorf("flagInt with no value = %d, want 10", *got)
	}
}
