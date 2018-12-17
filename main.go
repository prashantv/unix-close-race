package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"runtime"
	"syscall"
	"time"
)

var (
	flagPath        = flag.String("path", "/tmp/test.sock", "path to create the unix listener at")
	flagUseNewFd    = flag.Bool("use-new-fd", false, "whether to create the ln out of the file descriptor")
	flagSetNonblock = flag.Bool("set-nonblock", false, "whether to set O_NONBLOCK on the fd")
)

func main() {
	flag.Parse()
	go watchDog()

	os.RemoveAll(*flagPath)
	unixLn, err := net.ListenUnix("unix", &net.UnixAddr{Net: "unix", Name: *flagPath})
	panicOn(err)

	// Get the file-descriptor, which causes a call to Dup
	// This seems to cause the underlying file descriptor to
	// get into a bad state, where Close is not able to stop an Accept.
	f, err := unixLn.File()
	panicOn(err)

	fd := f.Fd()
	log.Print("Got fd from unix ln", fd)

	if *flagUseNewFd {
		// Create a listener from the duplicated file descriptor
		fileFromFd := os.NewFile(fd, f.Name())
		ln, err := net.FileListener(fileFromFd)
		panicOn(err)

		testCloseAfterAccept(ln)
	} else {
		if *flagSetNonblock {
			err := syscall.SetNonblock(int(fd), true /* non-blocking */)
			panicOn(err)
		}
		testCloseAfterAccept(unixLn)
	}

}

func testCloseAfterAccept(ln net.Listener) {
	acceptDone := make(chan struct{})
	go func() {
		defer close(acceptDone)

		log.Print("about to accept")
		_, err := ln.Accept()
		if err != nil {
			log.Print("accept failed", err)
			return
		}
		log.Print("accept completed")
	}()

	// Close has to be called after the Accept has been triggered.
	log.Print("wait for accept")
	time.Sleep(100 * time.Millisecond)
	log.Print("close")
	ln.Close()

	log.Print("wait for accept to end")
	<-acceptDone
}

func panicOn(err error) {
	if err != nil {
		panic(err)
	}
}

func watchDog() {
	time.Sleep(10 * time.Second)

	buf := make([]byte, 1024*1024)
	n := runtime.Stack(buf, true /* all */)
	fmt.Println("FROZEN\nStack:\n", string(buf[:n]))
	os.Exit(1)
}
