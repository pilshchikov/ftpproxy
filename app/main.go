package main

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

var cache = sync.Map{}

var address = os.Getenv("ADDRESS")
var login = os.Getenv("LOGIN")
var password = os.Getenv("PASSWORD")

var storagePath = os.Getenv("STORAGE_PATH")
var maxSize = os.Getenv("MAX_SIZE")
var attemptsToReconnect = os.Getenv("ATTEMPTS_TO_RECONNECT")

func processDownload(path string) DownloadState {
	downloadFileCh <- path
	for {
		localFile, ok := cache.Load(path)
		if ok {
			return localFile.(DownloadState)
		}
		time.Sleep(1 * time.Second)
	}
}

func get(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		path = r.Header.Get("path")
	}

	log.Println(fmt.Sprintf("<- %s", path))
	localFile, ok := cache.Load(path)
	if !ok || !fileExists(fmt.Sprintf("%s/%s", storagePath, localFile)) {
		if ok {
			cache.Delete(localFile)
		}
		downloadState := processDownload(path)
		if downloadState.error != nil {
			writeResponse(w, 500, downloadState.error.Error())
			return
		}
		localFile = downloadState.localFilePath
	}
	rf, err := ioutil.ReadFile(fmt.Sprintf("%s/%s", storagePath, localFile))
	if err != nil {
		writeResponse(w, 500, fmt.Sprintf("Failed to read downloaded file: %s", err))
		return
	}

	parts := strings.Split(path, "/")
	w.Header().Add("Content-Disposition", fmt.Sprintf("attachment; filename=%s", parts[len(parts)-1]))

	_, err = w.Write(rf)
	if err != nil {
		writeResponse(w, 500, fmt.Sprintf("Failed to write downloaded file: %s", err))
		return
	}
}

func scan() {
	log.Printf("Searching for existed files in %s", storagePath)
	files, err := ioutil.ReadDir(storagePath)
	if err != nil {
		log.Fatalf("Failed to read previous files dir %s! reason: %s", storagePath, err)
	}
	for _, file := range files {
		realName, err := base64.StdEncoding.DecodeString(file.Name())
		if err != nil {
			log.Fatalf("Failed to decode file %s! reason: %s", file.Name(), err)
		}
		cache.Store(string(realName), file.Name())
		log.Println(fmt.Sprintf("Found %s -> %s", string(realName), file.Name()))
	}
	log.Println("-----------------------------------")
}

func main() {
	getMaxAttemptsCount()
	scan()
	go downloader()
	go monitor()

	http.HandleFunc("/get", get)
	http.ListenAndServe(":9000", nil)
}
