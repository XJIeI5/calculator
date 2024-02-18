package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/XJIeI5/calculator/internal/storage"
)

func main() {
	hostPtr := flag.String("host", "http://localhost", "host of server")
	portPtr := flag.Int("port", 8080, "port of server")
	flag.Parse()

	go func() {
		fmt.Printf("run storage server at %s:%d\n", *hostPtr, *portPtr)
		s := storage.GetServer(*hostPtr, *portPtr)
		s.ListenAndServe()
	}()

	var stopChan = make(chan os.Signal, 2)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	<-stopChan // wait for SIGINT
	fmt.Println("stop storage server")
}
