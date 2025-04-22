package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

func init() {
	logDir := filepath.Join("/evolvia", "log", "auth_service")
	err := os.MkdirAll(logDir, 0755)
	if err != nil {
		log.Fatalf("Failed to create log directory: %v", err)
	}

	currentTime := time.Now()
	logFileName := fmt.Sprintf("log_%s.log", currentTime.Format("2006-01-02"))
	logFile := filepath.Join(logDir, logFileName)

	file, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	defer file.Close()

	log.SetOutput(file)

	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
}

func main() {
}
