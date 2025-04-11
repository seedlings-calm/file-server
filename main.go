package main

import (
	"file-server/pkg"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	MinioClientUser = []map[string]string{
		{
			"accessKey": "minioadmin",
			"secretKey": "minioadmin",
		},
		{
			"accessKey": "apiUser",
			"secretKey": "apiUser123456",
		},

		{
			"accessKey": "sysUser",
			"secretKey": "sysUser123456",
		},
	}
	minioClient          *pkg.MinIOClient
	publicBucketName     = "public" //桶
	userSourceBucketName = "usersource"
	folerNameImages      = "images" //图片文件夹
	folerNameVideos      = "videos" //视频文件夹
)

func main() {
	//如果是访问集群， 只需要连接任意一个即可
	cli, err := pkg.NewMinIOClient("localhost:9000", "minioadmin", "minioadmin", false)
	if err != nil {
		log.Fatalf("init minio client failed: %v", err)
	}
	minioClient = cli
	r := gin.Default()
	r.GET("policy", valitePolicy)
	r.POST("/upload", uploadFile)
	r.GET("/download", downloadFile)
	r.GET("/list", listFile)
	r.GET("/migrate", migrateFile)
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
	err = minioClient.EnsureBucket(c, publicBucketName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("ensureBucket to MinIO: %v", err)})
		return
	}
	_, err = minioClient.UploadMultipartFile(c, publicBucketName, folerName, objectName, srcFile, files.Size, contentType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to upload to MinIO: %v", err)})
		return
	}

	// 返回上传成功的响应
	c.JSON(http.StatusOK, gin.H{
		"message": "File uploaded successfully",
		"baseUrl": "http://localhost/files/", //nginx代理地址
		"fileUrl": folerName + "/" + objectName,
	})
}

// fileUrl 值： images/20231001_123456789.jpg
func downloadFile(c *gin.Context) {
	fileUrl := c.Query("fileUrl")
	if fileUrl == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No fileUrl provided"})
		return
	}
	//拆分url为目录和文件名
	arrFile := strings.Split(fileUrl, "/")
	if len(arrFile) != 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Error fileUrl param"})
		return
	}
	object, err := minioClient.DownloadFile(c, publicBucketName, arrFile[0], arrFile[1])
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("DownloadFile err to : %v", err)})
		return
	}
	defer object.Close()

	// 创建本地文件
	localFile, err := os.Create("./download/" + arrFile[1])
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to create local file: %v", err)})
		return
	}
	defer localFile.Close()

	// 将从 MinIO 下载的数据写入到本地文件
	_, err = io.Copy(localFile, object)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to write to local file: %v", err)})
		return
	}
	// 返回下载成功的响应
	c.JSON(http.StatusOK, gin.H{
		"message": "File download successfully",
		"fileUrl": "./download/" + arrFile[1],
	})
}

// list?bucket=public&folder=images
// bucket 必填, folder 选填
func listFile(c *gin.Context) {
	bucketName := c.Query("bucket")
	if bucketName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No bucket provided"})
		return
	}
	folerName := c.Query("folder")
	// 获取文件列表
	files, err := minioClient.ListFiles(c, bucketName, folerName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to list files: %v", err)})
		return
	}

	// 返回文件列表
	fileUrls := make([]interface{}, len(files))
	for i, file := range files {
		fileUrls[i] = file
	}
	c.JSON(http.StatusOK, gin.H{
		"message":  "File list successfully",
		"fileUrls": fileUrls,
	})
}

func migrateFile(c *gin.Context) {
	srcBucket := "public"
	dstBucket := "publicv1"
	prefixes := []string{""}
	overwrite := false
	removeSource := false

	// 可选的重命名函数，例如将所有对象的前缀 "images/" 替换为 "new-images/"
	renameFunc := func(key string) string {
		if strings.HasPrefix(key, "images/") {
			return "new-images/" + strings.TrimPrefix(key, "images/")
		}
		return key
	}
	// 迁移文件
	result, err := minioClient.MigrateFiles(c, srcBucket, dstBucket, prefixes, overwrite, removeSource, renameFunc)

	c.JSON(http.StatusOK, gin.H{
		"message": err,
		"result":  result,
	})
}

func valitePolicy(c *gin.Context) {
	var apiUser = MinioClientUser[1]
	apiUserCli, err := pkg.NewMinIOClient("localhost:9000", apiUser["accessKey"], apiUser["secretKey"], false)
	if err != nil {
		log.Fatalf("init minio client failed: %v", err)
	}
	err = apiUserCli.DeleteFile(c, publicBucketName, "images/", "1.png")
	if err != nil {
		log.Printf("apiUser delete:%s", err.Error())
	}
	list, err := apiUserCli.ListFiles(c, publicBucketName, "images/")
	log.Printf("public listFile: %v,%s", list, err)
	list1, err := apiUserCli.ListFiles(c, userSourceBucketName, "")
	log.Printf("usersource listFile: %v,%s", list1, err)

	var sysUser = MinioClientUser[2]
	sysUserCli, err := pkg.NewMinIOClient("localhost:9000", sysUser["accessKey"], sysUser["secretKey"], false)
	if err != nil {
		log.Fatalf("init minio client failed: %v", err)
	}
	err = sysUserCli.DeleteFile(c, userSourceBucketName, "user-1", "1.png")
	if err != nil {
		log.Printf("sysUser delete:%s", err.Error())
	}

	list2, err := sysUserCli.ListFiles(c, publicBucketName, "images/")
	log.Printf("public listFile: %v,%s", list2, err)
	list3, err := sysUserCli.ListFiles(c, userSourceBucketName, "")
	log.Printf("usersource listFile: %v,%s", list3, err)
}
