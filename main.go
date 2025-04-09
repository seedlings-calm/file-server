package main

import (
	"file-server/pkg"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	minioClient     *pkg.MinIOClient
	bucketName      = "public" //桶
	folerNameImages = "images" //图片文件夹
	folerNameVideos = "videos" //视频文件夹
)

func main() {
	cli, err := pkg.NewMinIOClient("localhost:9000", "minioadmin", "minioadmin", false)
	if err != nil {
		log.Fatalf("init minio client failed: %v", err)
	}
	minioClient = cli
	r := gin.Default()
	r.POST("/upload", uploadFile)
	r.Run(":8880")
}

func uploadFile(c *gin.Context) {
	// 从请求中获取文件
	_, files, _ := c.Request.FormFile("file")
	if files == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file provided"})
		return
	}
	// 检测文件类型
	ext := strings.ToLower(files.Filename[strings.LastIndex(files.Filename, ".")+1:])
	var contentType string
	var folerName string
	switch ext {
	case "jpg", "jpeg", "png", "gif":
		contentType = "image/" + ext
		folerName = folerNameImages
	case "mp4", "mkv", "avi":
		contentType = "video/" + ext
		folerName = folerNameVideos
	case "mp3", "wav", "flac":
		contentType = "audio/" + ext
		folerName = folerNameVideos
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get file types")})
		return
	}

	// 打开文件
	srcFile, err := files.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to open file: %v", err)})
		return
	}
	defer srcFile.Close()

	// 上传文件到 MinIO
	objectName := fmt.Sprintf("%s_%d.%s", time.Now().Format("20060102_150405"), time.Now().UnixNano(), ext)
	err = minioClient.EnsureBucket(c, bucketName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("ensureBucket to MinIO: %v", err)})
		return
	}
	_, err = minioClient.UploadMultipartFile(c, bucketName, folerName, objectName, srcFile, files.Size, contentType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to upload to MinIO: %v", err)})
		return
	}

	// 返回上传成功的响应
	c.JSON(http.StatusOK, gin.H{
		"message": "File uploaded successfully",
		"url":     fmt.Sprintf("http://localhost:80/%s", bucketName+"/"+folerName+objectName),
	})
}
