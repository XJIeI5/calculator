package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/XJIeI5/calculator/internal/computation"
	pb "github.com/XJIeI5/calculator/proto"
	"google.golang.org/grpc"
)

func main() {
	parallelPtr := flag.Int("pc", 10, "amount of parallel calculations")
	hostPtr := flag.String("host", "http://localhost", "host of server")
	portPtr := flag.Int("port", 5000, "port of server")
	flag.Parse()

	go func() {
		server := computation.GetServer(*hostPtr, *portPtr, *parallelPtr)
		fmt.Printf("run compute server at %s:%d\n", *hostPtr, *portPtr)
		lis, err := net.Listen("tcp", server.Addr())
		if err != nil {
			panic(err)
		}
		grpcServer := grpc.NewServer()
		pb.RegisterStorageServiceServer(grpcServer, server)
		if err := grpcServer.Serve(lis); err != nil {
			panic(err)
		}
		// comp := computation.GetServer(*hostPtr, *portPtr, *parallelPtr)
		// comp.ListenAndServe()
	}()

	var stopChan = make(chan os.Signal, 2)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	<-stopChan // wait for SIGINT
	fmt.Println("stop compute server")
}
