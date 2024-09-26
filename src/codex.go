package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config structure to hold exclusion rules
type Config struct {
	ExcludeFolders []string `yaml:"ExcludeFolders"`
	ExcludeFiles   []string `yaml:"ExcludeFiles"`
}

// Log message formatting
func logMessage(level, message string) {
	currentTime := time.Now().Format("2006-01-02 15:04:05")
	fmt.Printf("%s [%s] %s\n", currentTime, level, message)
}

// Function to read the YAML configuration
func readConfig(configPath string) (Config, error) {
	var config Config
	fileContent, err := ioutil.ReadFile(configPath)
	if err != nil {
		return config, err
	}
	err = yaml.Unmarshal(fileContent, &config)
	if err != nil {
		return config, err
	}
	return config, nil
}

// Function to write the YAML configuration
func writeConfig(configPath string, config Config) error {
	data, err := yaml.Marshal(&config)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(configPath, data, 0644)
}

// Function to detect project type and generate codex.yml
func detectProjectType(path string) string {
	if _, err := os.Stat(filepath.Join(path, "package.json")); err == nil {
		return "nodejs"
	}
	if _, err := os.Stat(filepath.Join(path, "requirements.txt")); err == nil {
		return "python"
	}
	if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil {
		return "go"
	}
	if _, err := os.Stat(filepath.Join(path, "pom.xml")); err == nil {
		return "java"
	}
	// Add more project type detections as needed
	return "default"
}

// Function to generate a codex.yml based on project type
func generateCodexYml(path string, projectType string) Config {
	var config Config

	switch projectType {
	case "nodejs":
		config = Config{
			ExcludeFolders: []string{"node_modules", "dist", "build"},
			ExcludeFiles:   []string{"package-lock.json", "yarn.lock"},
		}
	case "python":
		config = Config{
			ExcludeFolders: []string{"__pycache__", ".venv"},
			ExcludeFiles:   []string{"requirements.txt", "Pipfile.lock"},
		}
	case "go":
		config = Config{
			ExcludeFolders: []string{"vendor"},
			ExcludeFiles:   []string{"go.sum"},
		}
	case "java":
		config = Config{
			ExcludeFolders: []string{"target", ".gradle"},
			ExcludeFiles:   []string{"*.jar", "*.war"},
		}
	default:
		config = Config{
			ExcludeFolders: []string{".git", "bin", "obj"},
			ExcludeFiles:   []string{"*.log", "*.tmp"},
		}
	}

	return config
}

// Function to check if a file or folder should be excluded
func shouldExclude(path string, excludes []string) bool {
	for _, exclude := range excludes {
		if strings.Contains(path, exclude) {
			return true
		}
	}
	return false
}

func main() {
	// Subcommands
	initCmd := flag.NewFlagSet("init", flag.ExitOnError)
	runCmd := flag.NewFlagSet("run", flag.ExitOnError)

	// Flags for the 'run' command
	outputPath := runCmd.String("output", "code.txt", "Path to the output file")

	if len(os.Args) < 2 {
		fmt.Println("Expected 'init' or 'run' subcommands")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "init":
		initCmd.Parse(os.Args[2:])
		if len(initCmd.Args()) < 1 {
			fmt.Println("Usage: codex init <directory>")
			os.Exit(1)
		}
		dir := initCmd.Arg(0)

		// Detect project type and generate codex.yml
		projectType := detectProjectType(dir)
		config := generateCodexYml(dir, projectType)
		configPath := filepath.Join(dir, "codex.yml")

		err := writeConfig(configPath, config)
		if err != nil {
			logMessage("ERROR", fmt.Sprintf("Failed to write codex.yml: %s", err))
			os.Exit(1)
		}
		logMessage("INFO", fmt.Sprintf("Generated codex.yml for %s project in %s", projectType, configPath))

	case "run":
		runCmd.Parse(os.Args[2:])
		if len(runCmd.Args()) < 1 {
			fmt.Println("Usage: codex run <directory>")
			os.Exit(1)
		}
		dir := runCmd.Arg(0)

		// Determine the configuration file path
		configPath := filepath.Join(dir, "codex.yml")
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			logMessage("INFO", fmt.Sprintf("codex.yml not found in %s, using default settings", dir))
			projectType := "default"
			config := generateCodexYml(dir, projectType)
			configPath = filepath.Join(filepath.Dir(os.Args[0]), "config", "default.yml")
			writeConfig(configPath, config)
		}

		// Load the configuration
		config, err := readConfig(configPath)
		if err != nil {
			logMessage("ERROR", fmt.Sprintf("Failed to read config file: %s", err))
			os.Exit(1)
		}

		// Prepare the output file
		outputFile, err := os.Create(*outputPath)
		if err != nil {
			logMessage("ERROR", fmt.Sprintf("Failed to create output file: %s", err))
			os.Exit(1)
		}
		defer outputFile.Close()

		writer := bufio.NewWriter(outputFile)

		// Traverse the directory structure and process files
		err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				logMessage("ERROR", fmt.Sprintf("Error accessing path %s: %s", path, err))
				return err
			}

			// Check if the current file or folder should be excluded
			if info.IsDir() && shouldExclude(path, config.ExcludeFolders) {
				logMessage("INFO", fmt.Sprintf("Skipping folder: %s", path))
				return filepath.SkipDir
			}
			if !info.IsDir() && shouldExclude(info.Name(), config.ExcludeFiles) {
				logMessage("INFO", fmt.Sprintf("Skipping file: %s", path))
				return nil
			}

			// Process files
			if !info.IsDir() {
				logMessage("INFO", fmt.Sprintf("Processing file: %s", path))
				writer.WriteString(fmt.Sprintf("##### %s #####\n\n", path))

				content, err := ioutil.ReadFile(path)
				if err != nil {
					logMessage("ERROR", fmt.Sprintf("Failed to read file: %s", err))
					return err
				}
				writer.Write(content)
				writer.WriteString("\n\n")
			}

			return nil
		})

		if err != nil {
			logMessage("ERROR", fmt.Sprintf("Error walking the path: %s", err))
			os.Exit(1)
		}

		// Ensure all buffered data is written to the file
		writer.Flush()
		logMessage("INFO", fmt.Sprintf("All code has been extracted to %s", *outputPath))

	default:
		fmt.Println("Expected 'init' or 'run' subcommands")
		os.Exit(1)
	}
}
