package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/XJIeI5/calculator/internal/computation"
)

func main() {
	parallelPtr := flag.Int("pc", 10, "amount of parallel calculations")
	hostPtr := flag.String("host", "http://localhost", "host of server")
	portPtr := flag.Int("port", 5000, "port of server")
	flag.Parse()

	go func() {
		fmt.Printf("run compute server at %s:%d\n", *hostPtr, *portPtr)
		comp := computation.GetServer(*hostPtr, *portPtr, *parallelPtr)
		comp.ListenAndServe()
	}()

	var stopChan = make(chan os.Signal, 2)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	<-stopChan // wait for SIGINT
	fmt.Println("stop compute server")
}
