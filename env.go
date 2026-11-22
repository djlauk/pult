package main

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// envSet is an ordered environment where later Set calls win. On Windows,
// keys are case-insensitive (Path == PATH), matching the OS.
type envSet struct {
	keys []string          // original spelling, insertion order
	vals map[string]string // normalized key -> value
	orig map[string]string // normalized key -> original spelling
}

func newEnvSet() *envSet {
	return &envSet{vals: map[string]string{}, orig: map[string]string{}}
}

func normKey(k string) string {
	if runtime.GOOS == "windows" {
		return strings.ToUpper(k)
	}
	return k
}

func (e *envSet) Set(k, v string) {
	n := normKey(k)
	if _, ok := e.vals[n]; !ok {
		e.keys = append(e.keys, n)
		e.orig[n] = k
	}
	e.vals[n] = v
}

func (e *envSet) List() []string {
	out := make([]string, 0, len(e.keys))
	for _, n := range e.keys {
		out = append(out, e.orig[n]+"="+e.vals[n])
	}
	return out
}

// buildEnv layers the environment for a tool, later wins:
// system env, .env in the tool dir, PORT from url, env from tools.json.
func buildEnv(t Tool, dir string) ([]string, error) {
	e := newEnvSet()
	for _, kv := range os.Environ() {
		// Windows has hidden entries like "=C:=C:\x"; keep them verbatim.
		i := strings.Index(kv[1:], "=") + 1
		if i <= 0 {
			continue
		}
		e.Set(kv[:i], kv[i+1:])
	}

	dotenv, err := readDotEnv(filepath.Join(dir, ".env"))
	if err != nil {
		return nil, err
	}
	for _, kv := range dotenv {
		e.Set(kv[0], kv[1])
	}

	port, err := toolPort(t.URL)
	if err != nil {
		return nil, err
	}
	e.Set("PORT", port)

	for k, v := range t.Env {
		e.Set(k, v)
	}
	return e.List(), nil
}

// readDotEnv parses a .env file. A missing file yields no entries.
// Supports comments, blank lines, "export KEY=VALUE" and quoted values.
func readDotEnv(path string) ([][2]string, error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var out [][2]string
	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if n == 1 {
			line = strings.TrimPrefix(line, "\ufeff")
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		k, v, ok := strings.Cut(line, "=")
		k = strings.TrimSpace(k)
		if !ok || k == "" {
			return nil, fmt.Errorf("%s:%d: expected KEY=VALUE", path, n)
		}
		out = append(out, [2]string{k, dotEnvValue(strings.TrimSpace(v))})
	}
	return out, sc.Err()
}

func dotEnvValue(v string) string {
	if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') {
		if end := strings.IndexByte(v[1:], v[0]); end >= 0 {
			return v[1 : end+1]
		}
	}
	if i := strings.Index(v, " #"); i >= 0 {
		v = strings.TrimSpace(v[:i])
	}
	return v
}
