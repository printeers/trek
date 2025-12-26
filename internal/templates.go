package internal

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/printeers/trek/internal/configuration"
)

func ExecuteConfigTemplate(ts configuration.Template, version uint) (*string, error) {
	t, err := template.New(ts.Path).Parse(ts.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	var data bytes.Buffer
	err = t.Execute(&data, map[string]any{"NewVersion": version})
	if err != nil {
		return nil, fmt.Errorf("failed to execute template: %w", err)
	}

	str := data.String()

	return &str, nil
}
