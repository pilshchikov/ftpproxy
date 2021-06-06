package main

import (
	"encoding/base64"
	"fmt"
	"github.com/jlaffaye/ftp"
	"io/ioutil"
	"log"
	"os"
	"time"
)

var downloadFileCh = make(chan string)

func downloader() {
	for {
		toDownload, chOk := <-downloadFileCh
		if !chOk {
			time.Sleep(1 * time.Second)
			continue
		}

		_, ok := cache.Load(toDownload)
		if !ok {
			_, err := download(toDownload, 1)
			if err != nil {
				cache.Store(toDownload, DownloadState{
					localFilePath: "",
					error:         err,
				})
			}
		}
	}
}

func download(path string, attempt int) (string, error) {
	localFileName := base64.StdEncoding.EncodeToString([]byte(path))
	localFilePath := fmt.Sprintf("%s/%s", storagePath, localFileName)

	if fileExists(localFilePath) {
		return localFileName, nil
	}

	log.Println(fmt.Sprintf("<<- %s", path))

	c, err := ftp.Dial(address, ftp.DialWithTimeout(5*time.Second))
	if err != nil {
		if attempt == getMaxAttemptsCount() {
			return "", err
		} else {
			return download(path, attempt+1)
		}
	}
	defer c.Quit()

	err = c.Login(login, password)
	if err != nil {
		log.Fatal("Wrong credentials!")
	}

	r, err := c.Retr(path)
	if err != nil {
		if attempt == getMaxAttemptsCount() {
			return "", err
		} else {
			return download(path, attempt+1)
		}
	}
	defer r.Close()

	buf, err := ioutil.ReadAll(r)

	if fileExists(localFilePath) {
		return localFileName, nil
	}

	err = ioutil.WriteFile(localFilePath, buf, 0644)
	if err != nil {
		return "", err
	}
	cache.Store(path, localFileName)
	return localFileName, nil
}

func monitor() {
	for {
		if storageIsOverfilled() {
			files, err := ioutil.ReadDir(storagePath)
			if err != nil {
				log.Printf("Failed to read storage! reason: %s", err)
				time.Sleep(10 * time.Second)
				continue
			}
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
				err = os.Remove(fmt.Sprintf("%s/%s", storagePath, fileToDelete))
				if err != nil {
					log.Printf("Failed to delete file! reason: %s", err)
					os.Exit(1)
				}
			}
		} else {
			time.Sleep(10 * time.Second)
		}
	}
}
