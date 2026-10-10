package main

var version = "0.8.0-rc.1"

func Version() string {
	return version
}

func SetVersion(v string) {
	version = v
}
