package automation

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// INI is a small case-insensitive INI representation compatible with the
// section/key behavior used by AHK IniRead/IniWrite.
type INI struct {
	mu       sync.RWMutex
	sections map[string]map[string]string
}

const maxINILineBytes = 4 << 20

var iniFileLocks [64]sync.Mutex

func iniFileLock(path string) *sync.Mutex {
	cleaned, err := filepath.Abs(path)
	if err != nil {
		cleaned = filepath.Clean(path)
	}
	cleaned = strings.ToLower(cleaned)
	var hash uint32 = 2166136261
	for i := 0; i < len(cleaned); i++ {
		hash = (hash ^ uint32(cleaned[i])) * 16777619
	}
	return &iniFileLocks[hash%uint32(len(iniFileLocks))]
}

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
	s.Buffer(make([]byte, 64*1024), maxINILineBytes)
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
	i.mu.RLock()
	defer i.mu.RUnlock()
	if s, ok := i.sections[strings.ToLower(section)]; ok {
		if v, ok := s[strings.ToLower(key)]; ok {
			return v
		}
	}
	return def
}
func (i *INI) Set(section, key, value string) {
	if i == nil {
		return
	}
	i.mu.Lock()
	defer i.mu.Unlock()
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
	i.mu.Lock()
	defer i.mu.Unlock()
	if s := i.sections[strings.ToLower(section)]; s != nil {
		delete(s, strings.ToLower(key))
	}
}
func (i *INI) Save(path string) error {
	if i == nil {
		return errors.New("automation: nil INI")
	}
	if path == "" {
		return ErrInvalidArgument
	}
	fileMu := iniFileLock(path)
	fileMu.Lock()
	defer fileMu.Unlock()
	i.mu.RLock()
	defer i.mu.RUnlock()
	return saveINISnapshot(path, i.snapshotLocked())
}

func (i *INI) snapshot() map[string]map[string]string {
	if i == nil {
		return nil
	}
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.snapshotLocked()
}

func (i *INI) snapshotLocked() map[string]map[string]string {
	sections := make(map[string]map[string]string, len(i.sections))
	for section, values := range i.sections {
		copied := make(map[string]string, len(values))
		for key, value := range values {
			copied[key] = value
		}
		sections[section] = copied
	}
	return sections
}

func saveINISnapshot(path string, sections map[string]map[string]string) error {
	if err := validateINISnapshot(sections); err != nil {
		return err
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
	sectionNames := make([]string, 0, len(sections))
	for section := range sections {
		sectionNames = append(sectionNames, section)
	}
	sort.Strings(sectionNames)
	for _, section := range sectionNames {
		values := sections[section]
		if section != "" {
			if _, e = w.WriteString("[" + section + "]\n"); e != nil {
				_ = f.Close()
				return e
			}
		}
		keys := make([]string, 0, len(values))
		for key := range values {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			val := values[key]
			if _, e = w.WriteString(key + "=" + val + "\n"); e != nil {
				_ = f.Close()
				return e
			}
		}
		if _, e = w.WriteString("\n"); e != nil {
			_ = f.Close()
			return e
		}
	}
	if e = w.Flush(); e != nil {
		_ = f.Close()
		return e
	}
	if e = f.Sync(); e != nil {
		_ = f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	return os.Rename(tmp, path)
}

func validateINISnapshot(sections map[string]map[string]string) error {
	for section, values := range sections {
		if strings.ContainsAny(section, "[]\r\n") {
			return fmt.Errorf("automation: invalid INI section %q: %w", section, ErrInvalidArgument)
		}
		for key, value := range values {
			if key == "" || strings.ContainsAny(key, "=\r\n") || strings.ContainsAny(value, "\r\n") {
				return fmt.Errorf("automation: invalid INI entry in section %q: %w", section, ErrInvalidArgument)
			}
		}
	}
	return nil
}

// ReadINI and WriteINI serialize access to the same normalized path within
// this process. Atomic replacement protects readers but cannot coordinate an
// arbitrary external writer.
func ReadINI(path, section, key, def string) (string, error) {
	if path == "" {
		return def, ErrInvalidArgument
	}
	fileMu := iniFileLock(path)
	fileMu.Lock()
	defer fileMu.Unlock()
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
	if path == "" {
		return ErrInvalidArgument
	}
	if err := validateINISnapshot(map[string]map[string]string{section: {key: value}}); err != nil {
		return err
	}
	fileMu := iniFileLock(path)
	fileMu.Lock()
	defer fileMu.Unlock()
	i, e := LoadINI(path)
	if e != nil && !os.IsNotExist(e) {
		return e
	}
	if i == nil {
		i = NewINI()
	}
	i.Set(section, key, value)
	return saveINISnapshot(path, i.snapshot())
}
