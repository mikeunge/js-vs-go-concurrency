package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/akamensky/argparse"
	"github.com/google/uuid"

	filehelper "github.com/mikeunge/go/pkg/file-helper"
	pathhelper "github.com/mikeunge/go/pkg/path-helper"
)

type SavedMedia struct {
	Media []MediaItem `json:"Saved Media"`
}

type MediaItem struct {
	Date         string `json:"Date"`
	MediaType    string `json:"Media Type"`
	Location     string `json:"Location"`
	DownloadLink string `json:"Download Link"`
}

var OutPath = ""

func worker(channel chan MediaItem, waitGroup *sync.WaitGroup) {
	defer waitGroup.Done()
	for obj := range channel {
		if err := download(&obj); err != nil {
			fmt.Printf("Worker (download) returned an error, %s\n", err)
		}
	}
}

func download(obj *MediaItem) error {
	urlParts := strings.Split(obj.DownloadLink, "?")
	if len(urlParts) < 2 {
		return fmt.Errorf("Could not parse url, make sure it is a correctly formatted snapchat url")
	}

	reqBody := urlParts[1]
	res, err := http.Post(urlParts[0], "application/x-www-form-urlencoded", bytes.NewBuffer([]byte(reqBody)))
	if err != nil || res.StatusCode != 200 {
		return fmt.Errorf("Could not get download URL from snapchat for download link: %s, response: %d", obj.DownloadLink, res.StatusCode)
	}
	defer res.Body.Close()

	buf := new(bytes.Buffer)
	buf.ReadFrom(res.Body)
	url := buf.String()
	res, err = http.Get(url)
	if err != nil || res.StatusCode != 200 {
		return fmt.Errorf("Could not download image from S3, err: %s", err)
	}
	defer res.Body.Close()

	uid, err := uuid.NewRandom()
	if err != nil {
		return err
	}

	var fp string
	if obj.MediaType == "Image" {
		fp = filepath.Join(OutPath, fmt.Sprintf("%s.%s", uid.String(), "png"))
	} else if obj.MediaType == "Video" {
		fp = filepath.Join(OutPath, fmt.Sprintf("%s.%s", uid.String(), "mp4"))
	} else {
		return fmt.Errorf("Not a valid media type, %s", obj.MediaType)
	}

	buffer := new(strings.Builder)
	if _, err := io.Copy(buffer, res.Body); err != nil {
		return err
	}
	filehelper.WriteFile(fp, buffer.String(), 0700)

	return nil
}

func main() {
	parser := argparse.NewParser("Snapchat-Memorie-Downloader", "Download your snapchat memories.")
	argMemorie := parser.String("m", "memories-path", &argparse.Options{Required: true, Help: "Provide the (full) path to the memories_history.json file."})
	argWorker := parser.Int("w", "worker", &argparse.Options{Required: false, Help: "Amount of worker threads to spawn (default: 100)."})

	err := parser.Parse(os.Args)
	if err != nil {
		fmt.Printf("Parsing error\n%+v", parser.Usage(err))
		os.Exit(1)
	}

	if len(*argMemorie) == 0 {
		fmt.Printf("Memories path cannot be empty\n%+v", parser.Usage(err))
		os.Exit(1)
	}

	if !pathhelper.FileExists(*argMemorie) {
		fmt.Println("Memories file does not exist")
		os.Exit(-1)
	}

	workerThreads := 100
	if *argWorker < 1 {
		workerThreads = 100
	} else {
		workerThreads = *argWorker
	}

	data, err := filehelper.ReadFile(*argMemorie)
	if err != nil {
		fmt.Printf("Could not read from json, %s\nError: %s\n", *argMemorie, err)
		os.Exit(-1)
	}
	var media SavedMedia
	json.Unmarshal(data, &media)

	OutPath = filepath.Join("out", fmt.Sprintf("%d", time.Now().Unix()))
	if err := pathhelper.CreatePathIfNotExist(OutPath); err != nil {
		fmt.Printf("Could not create folder %s, %s\n", OutPath, err)
		os.Exit(1)
	}
	channel := make(chan MediaItem)
	wg := new(sync.WaitGroup)

	fmt.Printf("Spawning %d workers...\n", workerThreads)
	for i := uint(0); i < uint(workerThreads); i++ {
		wg.Add(1)
		go worker(channel, wg)
	}
	for i, obj := range media.Media {
		channel <- obj
		fmt.Printf("\rStatus: %d/%d", i, len(media.Media))
	}
	close(channel)
	wg.Wait()
	fmt.Printf("\nDone - check %s\n", OutPath)
}
