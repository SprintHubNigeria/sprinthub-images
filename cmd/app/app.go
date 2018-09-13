package main

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/gobuffalo/envy"
	"github.com/pkg/errors"

	"github.com/SprintHubNigeria/sprinthub-images/pkg/image"

	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
)

const (
	imageLocation    = "imageLocation"
	imageName        = "imageName"
	callbackURL      = "callbackURL"
	gcsStorageBucket = "GCS_STORAGE_BUCKET"
	imagesDir        = "IMAGES_DIR"
)

var (
	bucketName      = ""
	imagesDirectory = ""
	once            = sync.Once{}
)

func main() {
	http.HandleFunc("/_ah/warmup", warmUp)
	http.HandleFunc("/servingUrl", func(w http.ResponseWriter, r *http.Request) {
		once.Do(func() {
			ensureEnvVars(map[*string]string{&bucketName: gcsStorageBucket, &imagesDirectory: imagesDir})
		})
		if r.Method == http.MethodDelete {
			deleteServingURL(w, r)
		} else if r.Method == http.MethodGet {
			makeServingURL(w, r)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
		return
	})
	appengine.Main()
}

func warmUp(w http.ResponseWriter, r *http.Request) {
	ensureEnvVars(map[*string]string{&bucketName: gcsStorageBucket, &imagesDirectory: imagesDir})
	w.WriteHeader(http.StatusOK)
	return
}

func makeServingURL(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	gcsFileName := query.Get(imageLocation)
	ctx := appengine.NewContext(r)

	if gcsFileName == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(""))
		return
	}
	servingURL, err := makeServingURLFromGCS(ctx, gcsFileName)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(err.Error()))
		return
	}
	w.Write([]byte(servingURL))
	return
}

func makeServingURLFromGCS(ctx context.Context, gcsFileName string) (string, error) {
	img := &image.Image{FileName: gcsFileName}
	URL, err := img.CreateServingURL(ctx, bucketName)
	if err != nil {
		log.Criticalf(ctx, "%+v\n", err)
		return "", errors.WithMessage(err, "Could not get serving URL")
	}
	return URL, nil
}

func deleteServingURL(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	fileName := query.Get(imageLocation)
	if fileName == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("No image location in request query\n"))
		return
	}
	ctx := appengine.NewContext(r)
	img := &image.Image{FileName: fileName}
	if err := img.DeleteServingURL(ctx, bucketName); err != nil {
		log.Criticalf(ctx, "Deleting serving URL failed with error: %+v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("Could not delete serving URL for image %s, please retry\n", fileName)))
		return
	}
	if err := img.DeleteFromGCS(ctx, bucketName); err != nil {
		log.Criticalf(ctx, "Deleting image failed with error: %+v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("Could not delete image %s, please retry\n", fileName)))
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Image and serving URL deleted\n"))
}

func ensureEnvVars(envVars map[*string]string) {
	errTemplate := "Missing environment variable %s"
	for envVar, value := range envVars {
		env, err := envy.MustGet(value)
		if err != nil {
			panic(fmt.Sprintf(errTemplate, envVar))
		}
		*envVar = env
	}
}
