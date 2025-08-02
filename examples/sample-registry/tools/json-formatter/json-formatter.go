package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	json "github.com/goccy/go-json"
)

// JSONFormatter provides JSON formatting and validation utilities
type JSONFormatter struct {
	indent   string
	compact  bool
	validate bool
	sortKeys bool
}

// NewJSONFormatter creates a new JSON formatter with default settings
func NewJSONFormatter() *JSONFormatter {
	return &JSONFormatter{
		indent:   "  ",
		compact:  false,
		validate: true,
		sortKeys: false,
	}
}

// Format formats JSON input with specified options
func (jf *JSONFormatter) Format(input []byte) ([]byte, error) {
	// First validate the JSON
	var data interface{}
	if err := json.Unmarshal(input, &data); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	// Format based on options
	if jf.compact {
		return json.Marshal(data)
	}

	return json.MarshalIndent(data, "", jf.indent)
}

// Validate checks if the input is valid JSON
func (jf *JSONFormatter) Validate(input []byte) error {
	var data interface{}
	return json.Unmarshal(input, &data)
}

// FormatFile formats a JSON file and optionally writes to output file
func (jf *JSONFormatter) FormatFile(inputPath, outputPath string) error {
	// Read input file
	input, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("failed to read input file: %w", err)
	}

	// Format the JSON
	formatted, err := jf.Format(input)
	if err != nil {
		return fmt.Errorf("failed to format JSON: %w", err)
	}

	// Write to output (stdout if no output path specified)
	if outputPath == "" {
		_, err = os.Stdout.Write(formatted)
		fmt.Println() // Add newline
		return err
	}

	return os.WriteFile(outputPath, formatted, 0600)
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `JSON Formatter - Fast JSON formatting and validation tool

Usage: json-formatter [options] [input-file] [output-file]

Options:
  -c, --compact     Compact output (no indentation)
  -v, --validate    Validate JSON only (no formatting output)
  -i, --indent=STR  Set indentation string (default: "  ")
  -s, --sort-keys   Sort object keys alphabetically
  -h, --help        Show this help message

Examples:
  json-formatter data.json                    # Format to stdout
  json-formatter data.json formatted.json    # Format to file
  cat data.json | json-formatter              # Format from stdin
  json-formatter --validate data.json        # Validate only
  json-formatter --compact data.json         # Compact format

`)
}

func main() {
	formatter := NewJSONFormatter()
	var inputFile, outputFile string
	validateOnly := false

	// Simple argument parsing
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-h" || arg == "--help":
			printUsage()
			os.Exit(0)
		case arg == "-c" || arg == "--compact":
			formatter.compact = true
		case arg == "-v" || arg == "--validate":
			validateOnly = true
		case arg == "-s" || arg == "--sort-keys":
			formatter.sortKeys = true
		case strings.HasPrefix(arg, "--indent="):
			formatter.indent = strings.TrimPrefix(arg, "--indent=")
		case strings.HasPrefix(arg, "-i"):
			if len(arg) > 2 {
				formatter.indent = arg[2:]
			} else if i+1 < len(args) {
				formatter.indent = args[i+1]
				i++
			}
		case !strings.HasPrefix(arg, "-"):
			if inputFile == "" {
				inputFile = arg
			} else if outputFile == "" {
				outputFile = arg
			}
		}
	}

	// Handle input source
	var input []byte
	var err error

	if inputFile == "" {
		// Read from stdin
		input, err = io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading from stdin: %v\n", err)
			os.Exit(1)
		}
	} else {
		// Read from file
		input, err = os.ReadFile(inputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading file %s: %v\n", inputFile, err)
			os.Exit(1)
		}
	}

	// Validate mode
	if validateOnly {
		if err := formatter.Validate(input); err != nil {
			fmt.Fprintf(os.Stderr, "Invalid JSON: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✓ Valid JSON")
		return
	}

	// Format mode
	if inputFile != "" {
		err = formatter.FormatFile(inputFile, outputFile)
	} else {
		// Format from stdin
		formatted, formatErr := formatter.Format(input)
		if formatErr != nil {
			err = formatErr
		} else if outputFile == "" {
			_, err = os.Stdout.Write(formatted)
			if err == nil {
				fmt.Println() // Add newline
			}
		} else {
			err = os.WriteFile(outputFile, formatted, 0600)
		}
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
