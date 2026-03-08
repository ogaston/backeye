package main

import (
	"bufio"
	"context"
	"fmt"
	"net"
)

type Adapter struct {
	Connection *net.Conn
	reader     *bufio.Reader
	collector  *LogsCollector
}

func NewAdapter(port int, collector *LogsCollector) (*Adapter, error) {
	conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		return nil, err
	}
	return &Adapter{
		Connection: &conn,
		reader:     bufio.NewReader(conn),
		collector:  collector,
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

func (a *Adapter) Listen(ctx context.Context, onEvent func(event string)) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			msg, err := a.Receive()
			if err != nil {
				a.collector.AddLog(fmt.Sprintf("[Adapter] receive error: %v", err))
				return
			}
			if onEvent != nil {
				onEvent(msg)
			}
		}
	}
}
