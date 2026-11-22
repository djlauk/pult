package main

import (
	"reflect"
	"testing"
)

func TestSplitCommand(t *testing.T) {
	cases := map[string][]string{
		"./jlpt":                        {"./jlpt"},
		"npm run start":                 {"npm", "run", "start"},
		"uv run  scripts.liveview IM25b": {"uv", "run", "scripts.liveview", "IM25b"},
		`"C:/Program Files/x.exe" -a ""`: {"C:/Program Files/x.exe", "-a", ""},
		"":                              nil,
	}
	for in, want := range cases {
		if got := splitCommand(in); !reflect.DeepEqual(got, want) {
			t.Errorf("splitCommand(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestToolAddr(t *testing.T) {
	cases := map[string]string{
		"http://localhost:4747":          "localhost:4747",
		"http://localhost:8765/settings": "localhost:8765",
		"http://example.com":             "example.com:80",
	}
	for in, want := range cases {
		got, err := toolAddr(in)
		if err != nil || got != want {
			t.Errorf("toolAddr(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := toolAddr("obsidian://open"); err == nil {
		t.Error("expected error for url without port")
	}
}
