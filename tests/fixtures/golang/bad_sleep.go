package testdata

import "time"

// BadSleep uses time.Sleep which is forbidden
func BadSleep() {
	time.Sleep(100 * time.Millisecond)
}

// WaitWithSleep uses sleep in a loop
func WaitWithSleep() {
	for i := 0; i < 10; i++ {
		time.Sleep(time.Second)
	}
}
