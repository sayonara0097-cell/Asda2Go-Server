package main

import (
	"flag"
	"log"

	"asda2/shared/db"
	"asda2/shared/relay"
	"asda2/shared/types"
)

func main() {
	bind := flag.String("bind", types.EnvString("ASDA2_RELAY_ADDR", "127.0.0.1:5200"), "relay server TCP listen address")
	httpBind := flag.String("http", types.EnvString("ASDA2_RELAY_HTTP_ADDR", "127.0.0.1:7000"), "relay status HTTP listen address")
	flag.Parse()
	if flag.NArg() > 0 {
		*bind = flag.Arg(0)
	}

	if err := db.Init(db.DefaultDB); err != nil {
		log.Fatalf("[DB] %v", err)
	}
	if err := relay.InitBridgeDB(); err != nil {
		log.Fatalf("[RelayDB] %v", err)
	}

	if *httpBind != "" {
		go func() {
			if err := serveStatusHTTP(*httpBind); err != nil {
				log.Fatalf("[RelayHTTP] %v", err)
			}
		}()
	}
	log.Printf("[Relay] Starting on %s", *bind)
	if err := serveRelay(*bind); err != nil {
		log.Fatal(err)
	}
}
