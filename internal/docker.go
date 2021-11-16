package internal

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const PostgresDockerImage = "postgres:13"

func DockerRunPostgresContainer() (string, error) {
	cmdDocker := exec.Command("docker", "run", "--rm", "-d", "-e", "POSTGRES_PASSWORD=postgres", PostgresDockerImage)
	cmdDocker.Stderr = os.Stderr
	containerID, err := cmdDocker.Output()

	return strings.ReplaceAll(string(containerID), "\n", ""), err
}

func DockerKillContainer(containerID string) {
	cmdDocker := exec.Command("docker", "kill", containerID)
	_ = cmdDocker.Run()
}

func DockerGetContainerIP(containerID string) (string, error) {
	cmdDocker := exec.Command(
		"docker",
		"inspect",
		"-f",
		"{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}",
		containerID,
	)
	cmdDocker.Stderr = os.Stderr
	output, err := cmdDocker.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get container IP: %w", err)
	}

	return strings.TrimSuffix(string(output), "\n"), nil
}
