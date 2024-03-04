package main

var version = "0.6.0"

func Version() string {
	return version
}

func SetVersion(v string) {
	version = v
}
