package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
)

func storageIsOverfilled() bool {
	var dirSize int64 = 0

	readSize := func(path string, file os.FileInfo, err error) error {
		if !file.IsDir() {
			dirSize += file.Size()
		}

		return nil
	}

	err := filepath.Walk(storagePath, readSize)
	if err != nil {
		log.Fatalf("Failed to walk across %s! reason: %s", storagePath, err)
	}

	sizeMB := int(dirSize / 1024 / 1024)

	more := false
	sizeAmount, _ := strconv.Atoi(maxSize[:len(maxSize)-1])
	sizeFormat := maxSize[len(maxSize)-1:]
	if sizeFormat == "m" || sizeFormat == "M" {
		more = sizeMB > sizeAmount
	} else if sizeFormat == "g" || sizeFormat == "G" {
		more = sizeMB/1024 > sizeAmount
	}

	return more
}

func getMaxAttemptsCount() int {
	attemptsToReconnectInt, err := strconv.Atoi(attemptsToReconnect)
	if err != nil {
		log.Fatalf("Failed to parse attempts count %s! reason: %s", attemptsToReconnect, err)
	}
	return attemptsToReconnectInt
}

func writeResponse(w http.ResponseWriter, status int, reason string) {
	w.WriteHeader(status)
	w.Write([]byte(reason))
}

func fileExists(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}
	return true
}
