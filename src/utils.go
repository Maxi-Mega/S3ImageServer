package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
)

// formatFileName replaces all the '/' by a '@'.
func formatFileName(imgPath string) string {
	return strings.ReplaceAll(imgPath, "/", "@")
}

func generateImagesCache(pathOnDisk string) ImageCache {
	cache := ImageCache{pathOnDisk: pathOnDisk}
	err := filepath.WalkDir(pathOnDisk, func(imagePath string, file fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if file.IsDir() {
			return nil
		}

		info, err := file.Info()
		if err != nil {
			return err //nolint:wrapcheck
		}

		if info.ModTime().Add(config.RetentionPeriod).Before(time.Now()) {
			printDebug("Removing obsolete file from cache: ", imagePath)

			return os.Remove(imagePath) //nolint:wrapcheck
		}

		if strings.HasSuffix(imagePath, config.PreviewFilename) {
			cache.images = append(cache.images, newS3ImageFromCache(strings.TrimPrefix(imagePath, pathOnDisk), info))
		}

		return nil
	})

	if err != nil {
		printError(fmt.Errorf("failed to generate images cache: %w", err), false)
	}

	return cache
}

func getMainCacheFileLink(img, file string) string {
	return config.BasePath + "/cache/" + img + "/" + file
}

func getThumbnailsCacheFileLink(img string) string {
	return config.BasePath + "/thumbnails/" + img
}

func getFullProductImageLink(minioClient *minio.Client, objKey string) string {
	if config.FullProductSignedURL {
		signedURL, err := minioClient.PresignedGetObject(context.Background(), config.S3.BucketName, objKey, 7*24*time.Hour, url.Values{})
		if err != nil {
			printError(fmt.Errorf("failed to get a presigned object url: %w", err), false)

			return ""
		}

		newSignedURL := strings.TrimPrefix(signedURL.String(), signedURL.Scheme+"://"+signedURL.Host)

		return config.FullProductProtocol + url.QueryEscape(config.FullProductRootURL+newSignedURL)
	}

	return config.FullProductProtocol + config.S3.BucketName + "/" + objKey
}

type ImageInfos struct {
	Date         string        `json:"date"`
	Links        []string      `json:"links"`
	Geonames     string        `json:"geonames"`
	Localization *Localization `json:"localization"`
	Features     Features      `json:"features"`
	Thumbnails   []string      `json:"thumbnails"`
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
		printError(fmt.Errorf("failed to marshal http response to json: %w", err), false)
	}
}

func joinStructs(structs interface{}, sep string, displayFieldsName bool) (joined string) {
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
			if displayFieldsName {
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

func getFileContentType(file *os.File) (string, error) {
	if strings.HasSuffix(file.Name(), ".json") {
		return "application/json", nil
	}

	buffer := make([]byte, 512)

	_, err := file.Read(buffer)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	// Reseting the offset that has been shifted by the Read method
	_, _ = file.Seek(0, 0)

	// Use the net/http package's handy DectectContentType function. Always returns a valid
	// content-type by returning "application/octet-stream" if no others seemed to match.
	return http.DetectContentType(buffer), nil
}

func contentTypeFromFileName(filename string) string {
	f := strings.ToLower(filename)

	switch {
	case strings.HasSuffix(f, ".js"):
		return "application/javascript"
	case strings.HasSuffix(f, ".css"):
		return "text/css"
	default:
		return ""
	}
}

func clearDir(dir string) error {
	files, err := filepath.Glob(filepath.Join(dir, "*"))
	if err != nil {
		return err //nolint:wrapcheck
	}

	for _, file := range files {
		err = os.RemoveAll(file)
		if err != nil {
			return err //nolint:wrapcheck
		}
	}

	return nil
}

func createCache(cachePath string) ImageCache {
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		err = os.MkdirAll(cachePath, 0750)
		if err != nil {
			exitWithError(err)
		}

		return ImageCache{pathOnDisk: cachePath}
	}

	return generateImagesCache(cachePath)
}
