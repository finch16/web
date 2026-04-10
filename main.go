package main

import "flag"
import "os"
import "log"

func main() {
	time := flag.Int("t", 3600, "time")
	address := flag.String("b", "localhost:5000", "address")
	secret := flag.String("s", "", "secret")
	storage := flag.String("d", "./storage.db", "path to database file")
	flag.Parse()

	app, err:= start(*time, *secret, *storage, *address);
	if err != nil {
		log.Printf("Server failed: %v", err)
		os.Exit(1)
    }
    defer app.stop()
}
