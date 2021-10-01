package main

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func getJustFileName(filePath string) string {
	// return strings.TrimPrefix(strings.TrimPrefix(filePath, filepath.Dir(filePath)), "/")
	return filepath.Base(filePath)
}

func getImageId(name string, date time.Time) string {
	return name + "_" + date.Format(time.RFC3339)
}

func getImageType(imgName string) string {
	imgName = strings.TrimPrefix(imgName, "PREVIEW@")
	imgName = imgName[:strings.Index(imgName, "@")]
	return imgName
}

func formatFileName(imgPath string) string {
	return strings.ReplaceAll(imgPath, "/", "@")
}

func getImagesList() []EventObject {
	images := []EventObject{}

imagesCacheLoop:
	for imgToDo, dateToDo := range imagesCache {
		imgToDoObj := EventObject{
			ImgType: getImageType(imgToDo),
			ImgKey:  imgToDo,
			ImgName: getGeoname(imgToDo),
		}

		for i, imgDone := range images {
			if dateToDo.After(imagesCache[imgDone.ImgKey]) {
				images = append(images[:i], append([]EventObject{imgToDoObj}, images[i:]...)...) // insert new img at the right position
				continue imagesCacheLoop
			}
		}

		images = append(images, imgToDoObj) // insert new img at the end if it has the oldest date
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
			cache[formatFileName(strings.TrimPrefix(strings.TrimPrefix(imagePath, config.CacheDir), string(os.PathSeparator)))] = info.ModTime()
		}
		return nil
	})
	if err != nil {
		printError(err, false)
	}
	return cache
}

type ImageInfos struct {
	Date     string   `json:"date"`
	Links    []string `json:"links"`
	Geonames string   `json:"geonames"`
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
