package main

import (
	"fmt"
	"github.com/jrossi/gismo"
)

func main() {
	// Test the new namespace extraction function
	testURLs := []string{
		"https://github.com/gismo-team/claude-dev-tools",
		"git@github.com:security-org/audit-toolkit.git",
		"https://github.com/mycompany/internal-tools.git",
		"github.com/user/repo",
	}

	fmt.Println("Testing ExtractNamespacePath function:")
	for _, url := range testURLs {
		namespace := gismo.ExtractNamespacePath(url)
		fmt.Printf("Input:  %s\n", url)
		fmt.Printf("Output: %s\n", namespace)
		fmt.Println()
	}
}