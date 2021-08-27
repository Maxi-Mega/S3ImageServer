package main

import (
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

type templateData struct {
	Version         string
	BucketName      string
	PrefixName      string
	Previews        []string
	PreviewFilename string
	ImageTypes      []string
	RetentionPeriod float64
	PollingPeriod   float64
}

func startWebServer(port uint16) error {
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/image/", imageHandler)
	http.HandleFunc("/images", imagesListHandler)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent) // for ping
	})

	fmt.Println("\nStarting web server on port", port, "...")
	return http.ListenAndServe(":"+strconv.FormatUint(uint64(port), 10), nil)
}

func executeTemplate(w http.ResponseWriter, tmpl *template.Template, data interface{}) {
	w.WriteHeader(http.StatusOK)
	err := tmpl.Execute(w, data)
	if err != nil {
		prettier(w, "Failed to execute template:"+err.Error(), nil, http.StatusInternalServerError)
	}
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	deleteCookies(w, r)
	tmpl, err := getIndexTemplate()
	if err != nil {
		prettier(w, err.Error(), nil, http.StatusInternalServerError)
		return
	}
	executeTemplate(w, tmpl, templateData{
		Version:         version,
		BucketName:      config.S3.BucketName,
		PrefixName:      config.S3.KeyPrefix,
		Previews:        getImagesNames(),
		PreviewFilename: config.PreviewFilename,
		ImageTypes:      config.ImageTypes,
		RetentionPeriod: config.RetentionPeriod.Seconds(),
		PollingPeriod:   config.PollingPeriod.Seconds(),
	})
}

func imageHandler(w http.ResponseWriter, r *http.Request) {
	imgName := strings.TrimPrefix(r.URL.Path, "/image/")
	file, err := os.Open(path.Join(config.CacheDir, imgName))
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
	}
}

func imagesListHandler(w http.ResponseWriter, r *http.Request) {
	prettier(w, "Images list", getImagesNames(), http.StatusOK)
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
