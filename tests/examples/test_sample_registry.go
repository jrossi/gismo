package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jrossi/gismo/pkg/engine"
)

func main() {
	// Test manifest parsing
	manifestPath := "../sample-registry/manifest.json"

	fmt.Printf("🧪 Testing sample registry: %s\n", manifestPath)

	// Create manifest parser
	parser, err := engine.NewManifestParser()
	if err != nil {
		fmt.Printf("❌ Failed to create manifest parser: %v\n", err)
		os.Exit(1)
	}

	// Parse the manifest
	manifest, err := parser.ParseManifestFile(manifestPath)
	if err != nil {
		fmt.Printf("❌ Manifest parsing failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Manifest parsed successfully!\n")
	fmt.Printf("📦 Package: %s v%s\n", manifest.Name, manifest.Version)
	fmt.Printf("📝 Description: %s\n", manifest.Description)
	fmt.Printf("👤 Author: %s\n", manifest.Author)

	// Count components
	totalComponents := 0
	for groupName, components := range manifest.Components {
		count := len(components)
		totalComponents += count
		fmt.Printf("   %s: %d component(s)\n", groupName, count)
	}
	fmt.Printf("📊 Total components: %d\n", totalComponents)

	// Test package validation
	fmt.Printf("\n🔍 Testing package validation...\n")
	validator := engine.NewPackageValidator(true)

	ctx := context.Background()
	validationResult, err := validator.ValidatePackage(ctx, "../sample-registry", manifest)
	if err != nil {
		fmt.Printf("❌ Validation failed: %v\n", err)
		os.Exit(1)
	}

	if validationResult.Valid {
		fmt.Printf("✅ Package validation passed!\n")
	} else {
		fmt.Printf("❌ Package validation failed!\n")
		for _, error := range validationResult.Errors {
			fmt.Printf("   Error: %s\n", error)
		}
	}

	if len(validationResult.Warnings) > 0 {
		fmt.Printf("⚠️  Validation warnings:\n")
		for _, warning := range validationResult.Warnings {
			fmt.Printf("   Warning: %s\n", warning)
		}
	}

	// Show checksums
	if len(validationResult.Checksums) > 0 {
		fmt.Printf("\n🔐 File checksums generated: %d files\n", len(validationResult.Checksums))
	}

	fmt.Printf("\n🎉 Sample registry test completed successfully!\n")
}
