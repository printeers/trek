package internal

import "os"

func GetSSLMode() string {
	if os.Getenv("DISABLE_SSL") == "true" {
		return "disable"
	}

	return "require"
}
