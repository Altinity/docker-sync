package main

import (
	"log"
	"net/http"
	_ "net/http/pprof"

	"github.com/Altinity/docker-sync/cmd/dockersync/cmd"
)

func main() {
	go func() {
		log.Print(http.ListenAndServe(":1234", nil))
	}()
	cmd.Execute()
}
