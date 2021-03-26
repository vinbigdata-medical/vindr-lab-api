package stats

import (
	"bytes"
	"context"
	"log"

	"vindr-lab-api/utils"

	"github.com/minio/minio-go/v7"
)

type MinIOStorage struct {
	minioClient *minio.Client
	bucketName  string
	location    string
}

func NewMinIOStorage(minioClient *minio.Client, bucketName string) *MinIOStorage {
	return &MinIOStorage{
		minioClient: minioClient,
		bucketName:  bucketName,
	}
}

//StoreFile store
func (storage *MinIOStorage) StoreFile(fileName string, fileData []byte) error {
	ctx := context.Background()
	err := storage.minioClient.MakeBucket(ctx, storage.bucketName, minio.MakeBucketOptions{})
	if err != nil {
		// Check to see if we already own this bucket (which happens if you run this twice)
		exists, errBucketExists := storage.minioClient.BucketExists(ctx, storage.bucketName)
		if errBucketExists == nil && exists {
			log.Printf("We already own %s\n", storage.bucketName)
		} else {
			log.Fatalln(err)
			return err
		}
	} else {
		log.Printf("Successfully created %s\n", storage.bucketName)
	}

	// Upload the zip file
	objectName := fileName
	// filePath := objectName
	contentType := "application/json"

	// Upload the zip file with FPutObject
	info, err := storage.minioClient.PutObject(ctx, storage.bucketName, fileName, bytes.NewReader(fileData), -1, minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		log.Fatalln(err)
		return err
	}

	utils.LogInfo("Successfully uploaded %s of size %d\n", objectName, info.Size)

	return nil
}

func (storage *MinIOStorage) DownloadFile(labelExport LabelExport) (*minio.Object, error) {
	minioClient := storage.minioClient
	file, err := minioClient.GetObject(context.Background(), storage.bucketName, labelExport.Tag, minio.GetObjectOptions{})
	return file, err

}

func (storage *MinIOStorage) MakeBucket() error {
	minioClient := storage.minioClient
	err := minioClient.MakeBucket(context.Background(), storage.bucketName, minio.MakeBucketOptions{})
	return err

}
