package main

import (
	"flag"
	"log"
	"opsadmin/internal/app"
	"os"
)

func main() {
	defaultAddr := "127.0.0.1:8000"
	if port := os.Getenv("PORT"); port != "" {
		defaultAddr = "0.0.0.0:" + port
	}
	addr := flag.String("addr", defaultAddr, "listen address")
	db := flag.String("db", os.Getenv("DATABASE_URL"), "PostgreSQL DATABASE_URL")
	demo := flag.Bool("demo", os.Getenv("OPS_DEMO") != "false", "seed synthetic demo data when database is empty")
	flag.Parse()
	if err := app.Run(*addr, *db, *demo); err != nil {
		log.Fatal(err)
	}
}
