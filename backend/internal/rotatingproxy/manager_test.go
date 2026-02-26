package rotatingproxy

import (
	"io"
	"net"
	"testing"
	"time"
)

func TestDispatchSocksConnection_LimitsConcurrentWorkers(t *testing.T) {
	ps := &proxyServer{
		socksWorkerSem: make(chan struct{}, 1),
	}

	block := make(chan struct{})
	conn1Client, conn1Server := net.Pipe()
	defer conn1Client.Close()
	defer conn1Server.Close()

	started := make(chan struct{}, 1)
	accepted := ps.dispatchSocksConnection(conn1Server, func(conn net.Conn) {
		defer conn.Close()
		started <- struct{}{}
		<-block
	})
	if !accepted {
		t.Fatal("first connection was unexpectedly rejected")
	}

	select {
	case <-started:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("first worker did not start")
	}

	conn2Client, conn2Server := net.Pipe()
	defer conn2Client.Close()
	accepted = ps.dispatchSocksConnection(conn2Server, func(conn net.Conn) {
		defer conn.Close()
	})
	if accepted {
		t.Fatal("second connection was accepted despite full worker semaphore")
	}

	_ = conn2Client.SetReadDeadline(time.Now().Add(250 * time.Millisecond))
	buf := make([]byte, 1)
	_, err := conn2Client.Read(buf)
	if err == nil {
		t.Fatal("expected second connection to be closed")
	}
	if err != io.EOF {
		t.Fatalf("second connection read error = %v, want EOF", err)
	}

	close(block)

	deadline := time.Now().Add(500 * time.Millisecond)
	for len(ps.socksWorkerSem) != 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if len(ps.socksWorkerSem) != 0 {
		t.Fatal("worker semaphore token was not released after handler exit")
	}
}
