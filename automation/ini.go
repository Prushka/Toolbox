package automation

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// INI is a small case-insensitive INI representation compatible with the
// section/key behavior used by AHK IniRead/IniWrite.
type INI struct{ sections map[string]map[string]string }

func NewINI() *INI { return &INI{sections: make(map[string]map[string]string)} }
func LoadINI(path string) (*INI, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	ini := NewINI()
	section := ""
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(strings.TrimPrefix(s.Text(), "\ufeff"))
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
			if _, ok := ini.sections[section]; !ok {
				ini.sections[section] = map[string]string{}
			}
			continue
		}
		i := strings.IndexByte(line, '=')
		if i < 1 {
			continue
		}
		if _, ok := ini.sections[section]; !ok {
			ini.sections[section] = map[string]string{}
		}
		ini.sections[section][strings.ToLower(strings.TrimSpace(line[:i]))] = strings.TrimSpace(line[i+1:])
	}
	if e := s.Err(); e != nil {
		return nil, e
	}
	return ini, nil
}
func (i *INI) Get(section, key, def string) string {
	if i == nil {
		return def
	}
	if s, ok := i.sections[strings.ToLower(section)]; ok {
		if v, ok := s[strings.ToLower(key)]; ok {
			return v
		}
	}
	return def
}
func (i *INI) Set(section, key, value string) {
	if i.sections == nil {
		i.sections = map[string]map[string]string{}
	}
	s := strings.ToLower(section)
	if i.sections[s] == nil {
		i.sections[s] = map[string]string{}
	}
	i.sections[s][strings.ToLower(key)] = value
}
func (i *INI) Delete(section, key string) {
	if i == nil {
		return
	}
	if s := i.sections[strings.ToLower(section)]; s != nil {
		delete(s, strings.ToLower(key))
	}
}
func (i *INI) Save(path string) error {
	if i == nil {
		return errors.New("automation: nil INI")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	dir := filepath.Dir(path)
	f, e := os.CreateTemp(dir, "."+filepath.Base(path)+"-*.tmp")
	if e != nil {
		return e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	w := bufio.NewWriter(f)
	sections := make([]string, 0, len(i.sections))
	for section := range i.sections {
		sections = append(sections, section)
	}
	sort.Strings(sections)
	for _, section := range sections {
		values := i.sections[section]
		if section != "" {
			fmt.Fprintf(w, "[%s]\n", section)
		}
		keys := make([]string, 0, len(values))
		for key := range values {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			val := values[key]
			fmt.Fprintf(w, "%s=%s\n", key, val)
		}
		fmt.Fprintln(w)
	}
	if e = w.Flush(); e == nil {
		e = f.Close()
	} else {
		f.Close()
	}
	if e != nil {
		return e
	}
	return os.Rename(tmp, path)
}
func ReadINI(path, section, key, def string) (string, error) {
	i, e := LoadINI(path)
	if e != nil {
		if os.IsNotExist(e) {
			return def, nil
		}
		return "", e
	}
	return i.Get(section, key, def), nil
}
func WriteINI(path, section, key, value string) error {
	i, e := LoadINI(path)
	if e != nil && !os.IsNotExist(e) {
		return e
	}
	if i == nil {
		i = NewINI()
	}
	i.Set(section, key, value)
	return i.Save(path)
}
