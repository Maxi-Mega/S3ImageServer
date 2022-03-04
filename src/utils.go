package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/minio/minio-go/v7"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
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
			printDebug("Removing obsolete file from cache: ", imagePath)
			return os.Remove(imagePath)
		}
		if strings.HasSuffix(imagePath, config.PreviewFilename) {
			// images = append(images, getJustFileName(imagePath))
			cache[formatFileName(strings.TrimPrefix(strings.TrimPrefix(imagePath, config.CacheDir), string(os.PathSeparator)))] = info.ModTime()
		}
		return nil
	})
	if err != nil {
		printError(fmt.Errorf("failed to generate images cache: %v", err), false)
	}
	return cache
}

func getCorrespondingImage(objKey string) (image string, found bool) {
	for img := range imagesCache {
		lastSlash := strings.LastIndex(img, "@")
		if lastSlash < 0 {
			continue
		}
		imgDir := img[:lastSlash]
		if strings.HasPrefix(objKey, imgDir) {
			return img, true
		}
	}
	return "", false
}

func getFullProductImageLink(minioClient *minio.Client, objKey string) string {
	if config.FullProductSignedUrl {
		signedUrl, err := minioClient.PresignedGetObject(context.Background(), config.S3.BucketName, objKey, 7*24*time.Hour, url.Values{})
		if err != nil {
			printError(fmt.Errorf("failed to get a presigned object url: %v", err), false)
			return ""
		}
		newSignedUrl := strings.TrimPrefix(signedUrl.String(), signedUrl.Scheme+"://"+signedUrl.Host)
		return config.FullProductProtocol + url.QueryEscape(config.FullProductRootUrl+newSignedUrl)
	}
	return config.FullProductProtocol + config.S3.BucketName + "/" + objKey
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
		printError(fmt.Errorf("failed to marshal http response to json: %v", err), false)
	}
}

func joinStructs(structs interface{}, sep string, displayFieldName bool) (joined string) {
	vStructs := reflect.ValueOf(structs)
	if vStructs.Kind() != reflect.Slice {
		return ""
	}

	for i := 0; i < vStructs.Len(); i++ {
		val := vStructs.Index(i)
		if val.Kind() != reflect.Struct { // not a struct
			continue
		}

		for fi := 0; fi < val.Type().NumField(); fi++ {
			if displayFieldName {
				fieldName := val.Type().Field(fi).Name
				joined += fieldName + ": "
			}
			fieldValue := val.Field(fi).Interface()
			joined += fmt.Sprint(fieldValue)
			if fi < val.Type().NumField()-1 {
				joined += "/"
			}
		}

		if i < vStructs.Len()-1 {
			joined += sep
		}
	}
	return joined
}
