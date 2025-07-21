package main

import (
	"fmt"
	"os"

	"github.com/jrossi/gismo"
)

func main() {
	fmt.Println("🧪 Testing manifest validation...")

	// Create validator
	validator, err := gismo.NewManifestValidator()
	if err != nil {
		fmt.Printf("❌ Failed to create validator: %v\n", err)
		os.Exit(1)
	}

	// Read manifest file
	manifestPath := "../sample-registry/manifest.json"
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

	fmt.Println("✅ Manifest is valid!")
	fmt.Printf("📦 Package: %s v%s\n", manifest.Name, manifest.Version)
	fmt.Printf("📝 Description: %s\n", manifest.Description)

	// Count components
	totalComponents := 0
	for groupName, components := range manifest.Components {
		count := len(components)
		totalComponents += count
		fmt.Printf("   %s: %d component(s)\n", groupName, count)
	}
	fmt.Printf("📊 Total components: %d\n", totalComponents)
}
