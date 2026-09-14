package main

import (
	"flag"
	"log"
	"opsadmin/internal/app"
	"os"
)

func main() {
	addrDefault := os.Getenv("ADDR")
	if p := os.Getenv("PORT"); p != "" {
		addrDefault = ":" + p
	}
	if addrDefault == "" {
		addrDefault = "127.0.0.1:8000"
	}
	addr := flag.String("addr", addrDefault, "listen address")
	db := flag.String("db", os.Getenv("DATABASE_URL"), "PostgreSQL connection string (defaults to DATABASE_URL)")
	demo := flag.Bool("demo", true, "seed synthetic demo data when database is empty")
	flag.Parse()
	if err := app.Run(*addr, *db, *demo); err != nil {
		log.Fatal(err)
	}
}
