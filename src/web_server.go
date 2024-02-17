package main

import (
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
)

type templateData struct {
	Version                string
	BasePath               string
	TileServerURL          string
	WindowTitle            string
	ScaleInitialPercentage uint8
	BucketName             string
	PrefixName             string
	Previews               []EventObject
	PreviewsWithTime       map[string]time.Time
	PreviewFilename        string
	KeyPrefix              string
	FullProductExtension   string
	ImageGroups            []ImageGroup
	ImageTypes             []ImageType
	MaxImagesDisplayCount  int
	RetentionPeriod        float64
	PollingPeriod          float64
}

func executeTemplate(w http.ResponseWriter, tmpl *template.Template, data interface{}) {
	w.WriteHeader(http.StatusOK)

	err := tmpl.Execute(w, data)
	if err != nil {
		printError(fmt.Errorf("failed to execute template: %w", err), false)
		prettier(w, "Failed to execute template:"+err.Error(), nil, http.StatusInternalServerError)
	}
}

func imageHandler(w http.ResponseWriter, r *http.Request) {
	imgName := strings.TrimPrefix(r.URL.Path, "/image/")
	serveFile(w, filepath.Join(config.mainCacheDir, imgName))
}

func imagesListHandler(w http.ResponseWriter, _ *http.Request) {
	prettier(w, "Images list", mainCache.images, http.StatusOK)
}

func infosHandler(w http.ResponseWriter, r *http.Request, minioClient *minio.Client) {
	imgName := strings.TrimPrefix(r.URL.Path, "/infos/")

	var strDate string

	img, found := mainCache.findImageByKey(strings.ReplaceAll(imgName, "@", "/"))
	if found {
		strDate = img.LastModified.In(time.Local).Format("2006-01-02 15:04:05 MST")
	} else {
		strDate = "N/A"
	}

	imgDir := imgName[:strings.LastIndex(imgName, "@")+1]
	imgFormattedName := strings.ReplaceAll(imgDir, "@", string(os.PathSeparator))

	links, found := fullProductLinksCache[strings.TrimSuffix(imgFormattedName, string(os.PathSeparator))]
	if !found {
		links = []string{}
	}

	geonames, found := geonamesCache[imgDir+config.GeonamesFilename]
	if !found {
		geonames = Geonames{}
	}

	localization := img.AssociatedLocalization

	features := img.AssociatedFeatures
	if features == nil {
		features = &Features{}
	}

	thumbnails := fetchThumbnailsFrom(imgDir, img.S3Key, minioClient)

	prettier(w, "Image infos", ImageInfos{
		Date:         strDate,
		Links:        links,
		Geonames:     geonames.format(),
		Localization: localization,
		Features:     *features,
		Thumbnails:   thumbnails,
	}, http.StatusOK)
}

func cacheHandler(w http.ResponseWriter, r *http.Request) {
	wanted := strings.TrimPrefix(r.URL.Path, "/cache/")
	wanted = strings.TrimSuffix(wanted, "/")
	parts := strings.Split(wanted, "/")
	// URL structure: img@Name/filename
	if len(parts) != 2 {
		prettier(w, "Invalid URL", nil, http.StatusBadRequest)

		return
	}

	imgName := parts[0]
	imgNameWithSlashes := strings.ReplaceAll(imgName, "@", "/")
	filename := parts[1]

	_, found := fullProductLinksCache[imgNameWithSlashes]
	if !found {
		prettier(w, "Image not found !", nil, http.StatusNotFound)

		return
	}

	serveFile(w, filepath.Join(config.mainCacheDir, imgName+"@"+filename))
}

func thumbnailsHandler(w http.ResponseWriter, r *http.Request) {
	wanted := strings.TrimPrefix(r.URL.Path, "/thumbnails/")
	if wanted == "" {
		prettier(w, "Invalid URL", nil, http.StatusBadRequest)

		return
	}

	_, found := thumbnailsCache.findImageByKey(strings.ReplaceAll(wanted, "@", "/"))
	if !found {
		prettier(w, "Thumbnail not found !", nil, http.StatusNotFound)

		return
	}

	serveFile(w, filepath.Join(config.thumbnailsCacheDir, wanted))
}

func vendorHandler(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/vendor/"), "/")
	if len(parts) != 2 {
		prettier(w, "Invalid URL", nil, http.StatusBadRequest)
		return
	}

	lib, file := parts[0], parts[1]

	fileContent, err := getVendoredFile(lib, file)
	if err != nil {
		prettier(w, fmt.Sprintf("Failed to get vendored file %s/%s: %v", lib, file, err), nil, http.StatusNotFound)

		return
	}

	contentType := contentTypeFromFileName(file)
	w.Header().Set("Content-Type", contentType)
	_, _ = w.Write(fileContent)
}

func serveFile(w http.ResponseWriter, filePath string) {
	file, err := os.Open(filePath)
	if err != nil {
		prettier(w, "Failed to open file: "+err.Error(), nil, http.StatusInternalServerError)

		return
	}

	defer file.Close()

	contentType, err := getFileContentType(file)
	if err != nil {
		prettier(w, "Failed to detect file content-type: "+err.Error(), nil, http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)

	_, err = io.Copy(w, file)
	if err != nil {
		prettier(w, "Failed to read file: "+err.Error(), nil, http.StatusInternalServerError)
	}
}

func deleteCookies(w http.ResponseWriter, r *http.Request) {
	for _, cookie := range r.Cookies() {
		c := &http.Cookie{
			Name:     cookie.Name,
			Value:    "",
			Path:     "/",
			Expires:  time.Unix(0, 0),
			HttpOnly: true,
		}
		http.SetCookie(w, c)
	}
}
