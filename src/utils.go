package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	eventAdd    = "ADD"
	eventUpdate = "UPDATE"
	eventRemove = "REMOVE"
)

type event struct {
	EventType string `json:"event_type"`
	EventObj  string `json:"event_obj"`
	EventDate string `json:"event_date"`
}

func (evt event) Json() []byte {
	data, err := json.Marshal(evt)
	if err != nil {
		printError(err, false)
	}
	return data
}

func (evt event) String() string {
	switch evt.EventType {
	case eventAdd:
		return evt.EventType + ":" + evt.EventObj + "_" + evt.EventDate
	case eventUpdate:
		return evt.EventType + ":" + evt.EventObj + "_" + evt.EventDate
	case eventRemove:
		return evt.EventType + ":" + evt.EventObj
	default:
		fmt.Println("Unknown event type:", evt.EventType)
		return "ERROR"
	}
}

func getJustFileName(filePath string) string {
	// return strings.TrimPrefix(strings.TrimPrefix(filePath, filepath.Dir(filePath)), "/")
	return filepath.Base(filePath)
}

func getImageId(name string, date time.Time) string {
	return name + "_" + date.Format(time.RFC3339)
}

func formatImgName(imgPath string) string {
	/*parent := filepath.Dir(imgPath)
	parent = strings.TrimPrefix(parent, filepath.Dir(parent))
	return parent + "_" + getJustFileName(imgPath)*/
	return strings.ReplaceAll(imgPath, "/", "@")
}

func getImagesNames() []string {
	images := []string{}
	/*timeMap := map[string]time.Time{}
	err := filepath.WalkDir(config.CacheDir, func(imagePath string, file fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if file.IsDir() {
			return nil
		}
		info, err := file.Info()
		if err != nil {
			return err
		}
		fileName := getJustFileName(imagePath)
		timeMap[fileName] = info.ModTime()
		images = append(images, fileName)
		// if strings.HasSuffix(imagePath, ".jpg") {
		// 	images = append(images, getJustFileName(imagePath))
		// }
		return nil
	})
	if err != nil {
		printError(err, false)
	}
	startTime := time.Now()
	sort.Slice(images, func(i, j int) bool {
		return timeMap[images[i]].Before(timeMap[images[j]]) // sort according to modification time
	})
	fmt.Println("Sorted images in", time.Since(startTime), "!")*/

ImagesCacheLoop:
	for imgToDo, dateToDo := range imagesCache {
		for i, imgDone := range images {
			if dateToDo.After(imagesCache[imgDone]) {
				images = append(images[:i], append([]string{imgToDo}, images[i:]...)...) // insert new img at the right position
				continue ImagesCacheLoop
			}
		}
		images = append(images, imgToDo)
	}

	return images
}

func generateImagesCache() map[string]time.Time {
	cache := map[string]time.Time{}
	err := filepath.WalkDir(config.CacheDir, func(imagePath string, file fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if file.IsDir() {
			return nil
		}
		info, err := file.Info()
		if err != nil {
			return err
		}
		if info.ModTime().Add(config.RetentionPeriod).Before(time.Now()) {
			log("Removing obsolete file from cache:", imagePath)
			return os.Remove(imagePath)
		}
		if strings.HasSuffix(imagePath, config.PreviewFilename) {
			// images = append(images, getJustFileName(imagePath))
			cache[formatImgName(strings.TrimPrefix(strings.TrimPrefix(imagePath, config.CacheDir), string(os.PathSeparator)))] = info.ModTime()
		}
		return nil
	})
	if err != nil {
		printError(err, false)
	}
	return cache
}

type ImageInfos struct {
	Date  string   `json:"date"`
	Links []string `json:"links"`
}

func prettier(w http.ResponseWriter, message string, data interface{}, status int) {
	if data == nil {
		data = struct{}{}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	err := json.NewEncoder(w).Encode(struct {
		Message string      `json:"message"`
		Data    interface{} `json:"data"`
	}{
		Message: message,
		Data:    data,
	})
	if err != nil {
		printError(err, false)
	}
}
