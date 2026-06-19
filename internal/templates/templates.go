package templates

import (
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
)

const (
	// BuiltinPrefix marks a template source as an embedded built-in template.
	BuiltinPrefix = "builtin:"
)

// Templates reads built-in templates from an injected filesystem.
type Templates struct {
	files fs.FS
}

// New creates a built-in template registry backed by files.
func New(files fs.FS) Templates {
	return Templates{files: files}
}

// IsBuiltin reports whether source references an embedded built-in template.
func IsBuiltin(source string) bool {
	return strings.HasPrefix(strings.TrimSpace(source), BuiltinPrefix)
}

// Name trims the builtin prefix and surrounding whitespace from source.
func Name(source string) string {
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(source), BuiltinPrefix))
}

// Read returns an embedded built-in template by name.
func (t Templates) Read(source string) (filename string, body string, err error) {
	name := Name(source)
	if name == "" {
		return "", "", fmt.Errorf("built-in template name must not be empty")
	}
	if strings.Contains(name, "/") || strings.Contains(name, `\`) {
		return "", "", fmt.Errorf("built-in template name %q must not contain path separators", name)
	}
	if t.files == nil {
		return "", "", fmt.Errorf("built-in templates are not configured")
	}

	filename = name + ".tmpl"
	bodyBytes, err := fs.ReadFile(t.files, filename)
	if err != nil {
		return "", "", fmt.Errorf("read built-in template %q: %w", name, err)
	}

	return filename, string(bodyBytes), nil
}

// Exists reports whether the named built-in template can be read.
func (t Templates) Exists(source string) error {
	_, _, err := t.Read(source)
	return err
}

// Names returns all available built-in template names.
func (t Templates) Names() []string {
	if t.files == nil {
		return nil
	}

	entries, err := fs.ReadDir(t.files, ".")
	if err != nil {
		return nil
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || path.Ext(entry.Name()) != ".tmpl" {
			continue
		}

		names = append(names, strings.TrimSuffix(entry.Name(), ".tmpl"))
	}

	sort.Strings(names)
	return names
}
