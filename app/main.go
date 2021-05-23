package main

import (
	"encoding/base64"
	"fmt"
	"github.com/jlaffaye/ftp"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

var storedFiles = sync.Map{}
var downloadFile = make(chan string)

var address = os.Getenv("address")
var login = os.Getenv("login")
var password = os.Getenv("password")
var storagePath = os.Getenv("storage")
var maxSize = os.Getenv("maxSize")

func processDownload(path string) string {
	downloadFile <- path
	for {
		localFile, ok := storedFiles.Load(path)
		if ok {
			return localFile.(string)
		}
		time.Sleep(1 * time.Second)
	}
}

func get(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		path = r.Header.Get("path")
	}

	log.Println(fmt.Sprintf("?-> %s", path))
	localFile, ok := storedFiles.Load(path)
	if !ok || !fileExists(fmt.Sprintf("%s/%s", storagePath, localFile)) {
		if ok {
			storedFiles.Delete(localFile)
		}
		localFile = processDownload(path)
	}
	rf, err := ioutil.ReadFile(fmt.Sprintf("%s/%s", storagePath, localFile))
	handleErr(err, fmt.Sprintf("read %s", localFile))

	parts := strings.Split(path, "/")

	w.Header().Add("Content-Disposition", fmt.Sprintf("attachment; filename=%s", parts[len(parts)-1]))

	_, err = w.Write(rf)
	handleErr(err, "write resp")
}

func handleErr(err error, message string) bool {
	if err != nil {
		log.Fatal(fmt.Sprintf("ERROR: %s %s", message, err))
		return false
	}
	return true
}

func fileExists(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}
	return true
}

func download(path string) string {
	localFileName := base64.StdEncoding.EncodeToString([]byte(path))
	localFilePath := fmt.Sprintf("%s/%s", storagePath, localFileName)

	if fileExists(localFilePath) {
		return localFileName
	}

	log.Println(fmt.Sprintf("-> %s", path))
	c, err := ftp.Dial(fmt.Sprintf("%s:21", address), ftp.DialWithTimeout(5*time.Second))
	handleErr(err, "connect")

	err = c.Login(login, password)
	handleErr(err, "login")

	r, err := c.Retr(path)
	handleErr(err, fmt.Sprintf("download %s", path))
	defer r.Close()

	buf, err := ioutil.ReadAll(r)

	if fileExists(localFilePath) {
		return localFileName
	}

	err = ioutil.WriteFile(localFilePath, buf, 0644)
	storedFiles.Store(path, localFileName)

	handleErr(err, fmt.Sprintf("save file %s", localFileName))
	return localFileName
}

func scan() {
	files, err := ioutil.ReadDir(storagePath)
	handleErr(err, "scan dir")
	for _, file := range files {
		realName, err := base64.StdEncoding.DecodeString(file.Name())
		handleErr(err, "decode")
		storedFiles.Store(string(realName), file.Name())
		log.Println(fmt.Sprintf("O-O %s %s", string(realName), file.Name()))
	}
}

func downloader() {
	for {
		toDownload, chOk := <-downloadFile
		if !chOk {
			time.Sleep(1 * time.Second)
			continue
		}

		_, ok := storedFiles.Load(toDownload)
		if !ok {
			download(toDownload)
		}
	}
}

func storageIsHuge() bool {
	var dirSize int64 = 0

	readSize := func(path string, file os.FileInfo, err error) error {
		if !file.IsDir() {
			dirSize += file.Size()
		}

		return nil
	}

	err := filepath.Walk(storagePath, readSize)
	handleErr(err, "walk dir")

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

func monitor() {
	for {

		for {
			if storageIsHuge() {
				files, err := ioutil.ReadDir(storagePath)
				handleErr(err, "scan dir monitor")

				lowestModTime := time.Now()
				var fileToDelete string
				for _, file := range files {
					if lowestModTime.After(file.ModTime()) {
						fileToDelete = file.Name()
						lowestModTime = file.ModTime()
					}
				}

				if fileToDelete != "" {
					log.Println(fmt.Sprintf("remove %s", fileToDelete))
					err := os.Remove(fmt.Sprintf("%s/%s", storagePath, fileToDelete))
					handleErr(err, fmt.Sprintf("delete %s", fileToDelete))
				}
			} else {
				time.Sleep(10 * time.Second)
			}
		}

	}
}

func main() {
	scan()
	go downloader()
	go monitor()

	http.HandleFunc("/get", get)
	http.ListenAndServe(":9000", nil)
}
