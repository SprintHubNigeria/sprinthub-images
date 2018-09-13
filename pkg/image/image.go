package image

import (
	"fmt"

	"github.com/pkg/errors"

	"google.golang.org/appengine/blobstore"
	"google.golang.org/appengine/image"

	"golang.org/x/net/context"

	"cloud.google.com/go/storage"
)

var allowedImageTypes = map[string]bool{
	"image/png":  true,
	"image/jpg":  true,
	"image/jpeg": true,
}

// Image holds data about images stored in GCS
type Image struct {
	FileName    string
	OriginalURL string
	ServingURL  string
	Data        []byte
	ContentType string
}

// CreateServingURL returns a serving URL for an image in cloud storage
func (img *Image) CreateServingURL(ctx context.Context, bucketName string) (string, error) {
	blobKey, err := blobstore.BlobKeyForFile(ctx, fmt.Sprintf("/gs/%s/%s", bucketName, img.FileName))
	if err != nil {
		return "", errors.Wrap(err, "Could not create serving URL")
	}
	servingURL, err := image.ServingURL(ctx, blobKey, &image.ServingURLOptions{
		Secure: true,
		Size:   450,
	})
	if err != nil {
		return "", errors.Wrap(err, "Could not create serving URL")
	}
	img.ServingURL = servingURL.String()
	return img.ServingURL, nil
}

// DeleteServingURL makes the serving URL unavailable
func (img *Image) DeleteServingURL(ctx context.Context, bucketName string) error {
	key, err := blobstore.BlobKeyForFile(ctx, fmt.Sprintf("/gs/%s/%s", bucketName, img.FileName))
	if err != nil {
		return err
	}
	return image.DeleteServingURL(ctx, key)
}

// DeleteFromGCS removes the image from cloud storage
func (img *Image) DeleteFromGCS(ctx context.Context, bucketName string) error {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return err
	}
	return client.Bucket(bucketName).Object(img.FileName).Delete(ctx)
}
