package main

var version = "0.8.0"

// Version returns the generator version written into generated files.
func Version() string {
	return version
}

// SetVersion sets the generator version written into generated files.
func SetVersion(v string) {
	version = v
}
