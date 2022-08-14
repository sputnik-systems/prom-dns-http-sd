package main

import (
	"log"

	"github.com/sputnik-systems/prom-dns-http-sd/internal/app"
)

func main() {
	log.Fatal(app.Run())
}
