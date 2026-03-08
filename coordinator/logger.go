package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

type LogsCollector struct {
	mu      sync.Mutex
	logs    []string
	writer  *bufio.Writer
	logFile *os.File
}

func NewLogsCollector() *LogsCollector {

	// create a log file
	logFile, err := os.Create("coordinator.log")
	if err != nil {
		log.Fatal(err)
	}

	writer := bufio.NewWriter(logFile)

	return &LogsCollector{
		mu:      sync.Mutex{},
		logs:    make([]string, 0),
		writer:  writer,
		logFile: logFile,
	}
}

func (l *LogsCollector) AddLog(log string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logs = append(l.logs, log)
	l.writer.WriteString(time.Now().Format("2006-01-02 15:04:05") + " " + log + "\n")
	l.writer.Flush()
}

func (l *LogsCollector) GetLogs() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.logs
}

func (l *LogsCollector) Close() error {
	return l.logFile.Close()
}

// Logger struct
type Logger struct {
	lastMsgTime        time.Time
	lastMessage        string
	lastPublishingTime time.Time
	mu                 sync.Mutex
	collector          *LogsCollector
}

func NewLogger(collector *LogsCollector) *Logger {
	// clear the screen
	fmt.Println("\033[H\033[J")

	return &Logger{
		lastMsgTime:        time.Now(),
		lastMessage:        "",
		lastPublishingTime: time.Time{},
		mu:                 sync.Mutex{},
		collector:          collector,
	}
}

func (d *Logger) Update(message string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.lastMessage = message
	d.lastMsgTime = time.Now()
}

func (d *Logger) MarkPublished(message string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.lastPublishingTime = time.Now()
	d.lastMessage = message
	d.lastMsgTime = time.Now()
}

func (d *Logger) Render() {
	fmt.Println("\033[H\033[J")
	d.mu.Lock()
	defer d.mu.Unlock()

	status := "IDLE"
	if !d.lastMsgTime.IsZero() && time.Since(d.lastMsgTime) < 1*time.Second {
		status = "ACTIVE"
	}

	activity := "WAITING"
	if !d.lastPublishingTime.IsZero() && time.Since(d.lastPublishingTime) < 1*time.Second {
		activity = "PUBLISHING"
	}

	fmt.Println("   COORDINATOR DASHBOARD    ")
	fmt.Println("===============================")
	fmt.Printf(" Last Message:  %s\n", d.lastMsgTime.Format("15:04:05"))
	fmt.Printf(" Tracking:  [%-10s]\n", status)
	fmt.Printf(" Activity:  [%-10s]\n", activity)
	fmt.Printf(" Last Data: %-20s\n", d.lastMessage)
	fmt.Printf(" Updated:   %s\n", time.Now().Format("15:04:05"))
	fmt.Printf(" Logs:      %d\n", len(d.collector.GetLogs()))
	fmt.Println("-------------------------------")
	fmt.Println(" Press Ctrl+C to exit          ")
}
