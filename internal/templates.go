package internal

import (
	"bytes"
	"fmt"
	"text/template"
)

func ExecuteConfigTemplate(ts Template, version uint) (*string, error) {
	t, err := template.New(ts.Path).Parse(ts.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	var data bytes.Buffer
	err = t.Execute(&data, map[string]interface{}{"NewVersion": version})
	if err != nil {
		return nil, fmt.Errorf("failed to execute template: %w", err)
	}

	str := data.String()

	return &str, nil
}
