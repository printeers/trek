package internal

import (
	"log"
	"os/exec"
)

func assertToolsAvailable(tools []string) {
	for _, tool := range tools {
		_, err := exec.LookPath(tool)
		if err != nil {
			log.Fatalf("Missing %s\n", tool)
		}
	}
}

func AssertApplyToolsAvailable() {
	assertToolsAvailable([]string{"psql"})
}

func AssertGenerateToolsAvailable() {
	assertToolsAvailable([]string{"psql", "docker"})
}
