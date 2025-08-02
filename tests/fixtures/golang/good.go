package testdata

import "fmt"

// GoodFunction is a well-formatted function
func GoodFunction() {
	message := "Hello, World!"
	fmt.Println(message)
}

// ProcessData handles data processing
func ProcessData(data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("empty data")
	}
	// Process the data
	return nil
}
