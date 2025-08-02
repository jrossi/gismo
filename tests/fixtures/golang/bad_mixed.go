package testdata

import (
	"fmt"
	"time"
)

// BadMixed has multiple issues
func BadMixed(data interface{}) {
	// Using interface{} parameter
	fmt.Println(data)

	// Using time.Sleep
	time.Sleep(1 * time.Second)

	// TODO: Fix this later

	// Using panic
	if data == nil {
		panic("data is nil")
	}
}

// ProcessGeneric uses any
func ProcessGeneric(items []any) {
	for _, item := range items {
		fmt.Println(item)
	}
}
