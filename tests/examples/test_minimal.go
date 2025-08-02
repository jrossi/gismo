package main

import (
	"fmt"
	"os"

	"github.com/jrossi/gismo/pkg/engine"
)

func main() {
	fmt.Println("🧪 Testing minimal manifest validation...")

	// Create validator
	validator, err := engine.NewManifestValidator()
	if err != nil {
		fmt.Printf("❌ Failed to create validator: %v\n", err)
		os.Exit(1)
	}

	// Read minimal manifest file
	manifestPath := "minimal_manifest.json"
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		fmt.Printf("❌ Failed to read manifest: %v\n", err)
		os.Exit(1)
	}

	// Validate
	manifest, err := validator.ValidateManifestBytes(data)
	if err != nil {
		fmt.Printf("❌ Validation error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Minimal manifest is valid!")
	fmt.Printf("📦 Package: %s v%s\n", manifest.Name, manifest.Version)
	fmt.Printf("📊 Total component groups: %d\n", len(manifest.Components))
}
