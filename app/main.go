package main

import (
	"fmt"
	"net/http"
	"os"

	"golang.org/x/net/context"
	"google.golang.org/appengine/image"

	"cloud.google.com/go/storage"
	"google.golang.org/appengine/blobstore"

	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
)

const (
	imageLocation = "imageLocation"
)

var (
	bucketName = ""
)

func main() {
	bucketName = os.Getenv("GCS_STORAGE_BUCKET")
	if bucketName == "" {
		panic(fmt.Errorf("Missing environment variable %q", "GCS_STORAGE_BUCKET"))
	}
	http.HandleFunc("/_ah/warmup", warmUp)
	http.HandleFunc("/servingUrl", func(w http.ResponseWriter, r *http.Request) {
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
	servingURL, err := createServingURL(ctx, bucketName, gcsFileName)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(err.Error()))
		log.Criticalf(ctx, "%+v\n", err)
		return
	}
	w.Write([]byte(servingURL))
	return
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
	if err := deleteImageServingURL(ctx, bucketName, fileName); err != nil {
		log.Criticalf(ctx, "Deleting serving URL failed with error: %+v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("Could not delete serving URL for image %s, please retry\n", fileName)))
		return
	}
	if err := deleteFromGCS(ctx, bucketName, fileName); err != nil {
		log.Criticalf(ctx, "Deleting image failed with error: %+v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("Could not delete image %s, please retry\n", fileName)))
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Image and serving URL deleted\n"))
}

// createServingURL returns a serving URL for an image in cloud storage
func createServingURL(ctx context.Context, bucketName, fileName string) (string, error) {
	blobKey, err := blobstore.BlobKeyForFile(ctx, fmt.Sprintf("/gs/%s/%s", bucketName, fileName))
	if err != nil {
		return "", err
	}
	servingURL, err := image.ServingURL(ctx, blobKey, &image.ServingURLOptions{
		Secure: true,
		Size:   450,
	})
	if err != nil {
		return "", err
	}
	return servingURL.String(), nil
}

// deleteImageServingURL makes the serving URL unavailable
func deleteImageServingURL(ctx context.Context, bucketName, fileName string) error {
	key, err := blobstore.BlobKeyForFile(ctx, fmt.Sprintf("/gs/%s/%s", bucketName, fileName))
	if err != nil {
		return err
	}
	return image.DeleteServingURL(ctx, key)
}

// deleteFromGCS removes the image from cloud storage
func deleteFromGCS(ctx context.Context, bucketName, fileName string) error {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return err
	}
	return client.Bucket(bucketName).Object(fileName).Delete(ctx)
}
