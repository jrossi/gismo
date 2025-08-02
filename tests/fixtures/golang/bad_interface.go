package testdata

// BadFunction uses interface{} which is forbidden
func BadFunction(data interface{}) interface{} {
	return data
}

// ProcessAny uses any which is also forbidden
func ProcessAny(input any) any {
	return input
}
