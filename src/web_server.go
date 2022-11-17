package main

import (
	"fmt"
	"github.com/minio/minio-go/v7"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type templateData struct {
	Version                string
	BasePath               string
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

/*func startWebServer(port uint16) error {
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/image/", imageHandler)
	http.HandleFunc("/images", imagesListHandler)
	http.HandleFunc("/infos/", infosHandler)
	http.HandleFunc("/cache/", cacheHandler)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent) // for ping
	})

	printInfo("Starting web server on port ", port, " ...")
	return http.ListenAndServe(":"+strconv.FormatUint(uint64(port), 10), nil)
}*/

func executeTemplate(w http.ResponseWriter, tmpl *template.Template, data interface{}) {
	w.WriteHeader(http.StatusOK)
	err := tmpl.Execute(w, data)
	if err != nil {
		printError(fmt.Errorf("failed to execute template: %v", err), false)
		prettier(w, "Failed to execute template:"+err.Error(), nil, http.StatusInternalServerError)
	}
}

/*func indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		http.NotFound(w, r)
		return
	}
	deleteCookies(w, r)
	tmpl, err := getIndexTemplate()
	if err != nil {
		prettier(w, err.Error(), nil, http.StatusInternalServerError)
		return
	}
	executeTemplate(w, tmpl, templateData{
		Version:                version,
		BasePath:               config.BasePath,
		WindowTitle:            config.WindowTitle,
		ScaleInitialPercentage: config.ScaleInitialPercentage,
		BucketName:             config.S3.BucketName,
		PrefixName:             "config.S3.KeyPrefix",
		Previews:               mainCache.toEventObjects(),
		// PreviewsWithTime:       mainCache, TODO: add time to EventObject ?
		PreviewFilename:       config.PreviewFilename,
		FullProductExtension:  config.FullProductExtension,
		KeyPrefix:             "config.S3.KeyPrefix",
		ImageGroups:           config.ImageGroups,
		ImageTypes:            config.imageTypes,
		MaxImagesDisplayCount: config.MaxImagesDisplayCount,
		RetentionPeriod:       config.RetentionPeriod.Seconds(),
		PollingPeriod:         config.PollingPeriod.Seconds(),
	})
}*/

func imageHandler(w http.ResponseWriter, r *http.Request) {
	imgName := strings.TrimPrefix(r.URL.Path, "/image/")
	/*file, err := os.Open(path.Join(config.mainCacheDir, imgName))
	if err != nil {
		if os.IsNotExist(err) {
			prettier(w, "This image does not exist !", nil, http.StatusBadRequest)
		} else {
			prettier(w, "Failed to open image: "+err.Error(), nil, http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "image/jpeg")
	w.WriteHeader(http.StatusOK)
	_, err = io.Copy(w, file)
	if err != nil {
		prettier(w, "Failed to send image: "+err.Error(), nil, http.StatusInternalServerError)
	}*/
	serveFile(w, filepath.Join(config.mainCacheDir, imgName))
}

func imagesListHandler(w http.ResponseWriter, r *http.Request) {
	prettier(w, "Images list", mainCache.images, http.StatusOK)
}

func infosHandler(w http.ResponseWriter, r *http.Request, minioClient *minio.Client) {
	imgName := strings.TrimPrefix(r.URL.Path, "/infos/")
	img, found := mainCache.findImageByKey(strings.ReplaceAll(imgName, "@", "/"))
	var strDate string
	if found {
		strDate = img.LastModified.Format("2006-01-02 15:04:05")
	} else {
		strDate = "N/A"
	}
	// imgDir := strings.TrimSuffix(imgName, config.PreviewFilename)
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
	features := img.AssociatedFeatures
	if features == nil {
		features = &Features{}
	}
	thumbnails := fetchThumbnailsFrom(imgDir, minioClient)
	prettier(w, "Image infos", ImageInfos{
		Date:       strDate,
		Links:      links,
		Geonames:   geonames.format(),
		Features:   *features,
		Thumbnails: thumbnails,
	}, http.StatusOK)
}

func cacheHandler(w http.ResponseWriter, r *http.Request) {
	wanted := strings.TrimPrefix(r.URL.Path, "/cache/")
	wanted = strings.TrimSuffix(wanted, "/")
	parts := strings.Split(wanted, "/")
	// URL structure: imgNameWithSlashes/filename
	if len(parts) != 2 {
		prettier(w, "Invalid URL", nil, http.StatusBadRequest)
		return
	}
	imgName := parts[0]
	imgNameWithSlashes := strings.ReplaceAll(imgName, "@", "/")
	_, found := mainCache.findImageByKey(imgNameWithSlashes + "/" + config.PreviewFilename)
	if !found {
		prettier(w, "Image not found !", nil, http.StatusNotFound)
		return
	}
	filename := parts[1]
	_, found = fullProductLinksCache[imgNameWithSlashes]
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
