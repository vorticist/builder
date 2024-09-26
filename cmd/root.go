package cmd

import (
	"bufio"
	"fmt"
	"github.com/vorticist/logger"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var installFlag bool

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:   "vbuilder [project-path]",
	Short: "A CLI to validate, build, and create a service for Go projects",
	Args:  cobra.ExactArgs(1),
	Run:   runProjectSetup,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		logger.Errorf("%v", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&installFlag, "install", "i", false, "Copy the .service file to systemd folder and enable it")
}

func runProjectSetup(cmd *cobra.Command, args []string) {
	projectPath := args[0]

	// Step 1: Validate if path contains a Go project and extract module name
	moduleName, err := getModuleName(projectPath)
	if err != nil {
		logger.Errorf("Error: %v", err)
		return
	}

	// Step 2: Build the project
	binaryName := moduleName
	binaryPath, err := buildGoProject(projectPath, binaryName)
	if err != nil {
		logger.Errorf("Failed to build the Go project: %v", err)
		return
	}

	// Step 3: Generate the .service file using the absolute path of the binary
	serviceFilePath := filepath.Join(projectPath, fmt.Sprintf("%s.service", binaryName))
	generateServiceFile(serviceFilePath, binaryPath, binaryName)

	logger.Infof("Service file created at: %v", serviceFilePath)

	// Step 4: Optionally install the service file
	if installFlag {
		copyServiceToSystemd(serviceFilePath, binaryName)
	}
}

func getModuleName(projectPath string) (string, error) {
	goModFile := filepath.Join(projectPath, "go.mod")
	file, err := os.Open(goModFile)
	if err != nil {
		return "", fmt.Errorf("failed to open go.mod file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "module") {
			// Extract module name
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				return filepath.Base(parts[1]), nil
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("failed to read go.mod file: %v", err)
	}

	return "", fmt.Errorf("module name not found in go.mod")
}

func buildGoProject(projectPath, binaryName string) (string, error) {
	// Step 1: Build the binary and output it directly to the root of the project
	binaryPath := filepath.Join(projectPath, binaryName)
	cmd := exec.Command("go", "build", "-o", binaryPath, projectPath)
	cmd.Dir = projectPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}

	// Return the absolute path of the binary
	absBinaryPath, err := filepath.Abs(binaryPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path of binary: %v", err)
	}

	return absBinaryPath, nil
}

func generateServiceFile(servicePath, binaryPath, serviceName string) {
	serviceContent := fmt.Sprintf(`[Unit]
Description=vortex.studio/%s Service
After=network.target

[Service]
ExecStart=%s
Restart=always
User=%s
WorkingDirectory=%s
RestartSec=10

[Install]
WantedBy=multi-user.target
`, serviceName, binaryPath, os.Getenv("USER"), filepath.Dir(binaryPath))

	// Write to .service file
	os.WriteFile(servicePath, []byte(serviceContent), 0644)
}

func copyServiceToSystemd(serviceFilePath, serviceName string) {
	systemdPath := fmt.Sprintf("/etc/systemd/system/%s.service", serviceName)

	// Use sudo to copy the service file
	copyCmd := exec.Command("sudo", "cp", serviceFilePath, systemdPath)
	copyCmd.Stdout = os.Stdout
	copyCmd.Stderr = os.Stderr
	err := copyCmd.Run()
	if err != nil {
		logger.Errorf("Failed to copy service file to systemd: %v", err)
		return
	}
	logger.Infof("Service file copied to: %v", systemdPath)

	// Reload systemd daemon
	reloadCmd := exec.Command("sudo", "systemctl", "daemon-reload")
	reloadCmd.Stdout = os.Stdout
	reloadCmd.Stderr = os.Stderr
	err = reloadCmd.Run()
	if err != nil {
		logger.Errorf("Failed to reload systemd daemon: %v", err)
		return
	}

	// Enable the service
	enableCmd := exec.Command("sudo", "systemctl", "enable", fmt.Sprintf("%s.service", serviceName))
	enableCmd.Stdout = os.Stdout
	enableCmd.Stderr = os.Stderr
	err = enableCmd.Run()
	if err != nil {
		logger.Errorf("Failed to enable service: %v", err)
		return
	}

	// Start the service
	startCmd := exec.Command("sudo", "systemctl", "start", fmt.Sprintf("%s.service", serviceName))
	startCmd.Stdout = os.Stdout
	startCmd.Stderr = os.Stderr
	err = startCmd.Run()
	if err != nil {
		logger.Errorf("Failed to start service: %v", err)
		return
	}

	logger.Info("Service enabled and started successfully.")
}
