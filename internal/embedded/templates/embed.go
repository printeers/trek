package templates

import _ "embed"

//go:embed dbm.tmpl
var DbmTmpl string

//go:embed docker-compose.yaml.tmpl
var DockerComposeYamlTmpl string

//go:embed Dockerfile.tmpl
var DockerfileTmpl string

//go:embed trek.yaml.tmpl
var TrekYamlTmpl string
