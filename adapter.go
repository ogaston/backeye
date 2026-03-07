package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net"
)

type Adapter struct {
	Connection *net.Conn
	reader     *bufio.Reader
}

func NewAdapter(port int) (*Adapter, error) {
	conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		return nil, err
	}
	return &Adapter{
		Connection: &conn,
		reader:     bufio.NewReader(conn),
	}, nil
}

func (a *Adapter) Close() error {
	return (*a.Connection).Close()
}

func (a *Adapter) Send(message string) error {
	_, err := (*a.Connection).Write([]byte(message))
	return err
}

func (a *Adapter) Receive() (string, error) {
	return a.reader.ReadString('\n')
}

func (a *Adapter) Listen(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			msg, err := a.Receive()
			if err != nil {
				log.Printf("[Adapter] receive error: %v", err)
				return
			}
			fmt.Printf("[TRACKER] %s", msg)
		}
	}
}
