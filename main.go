package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go-big-file-uploader/db"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

type Metadata struct {
	Order    int    `json:"order"`
	FileId   string `json:"fileId"`
	Offset   int    `json:"offset"`
	Limit    int    `json:"limit"`
	FileSize int    `json:"fileSize"`
	FileName string `json:"fileName"`
	CheckSum string `json:"checkSum"`
}

func main() {
	r := gin.Default()
	r.Static("/uploads", "./uploads")

	r.Use(cors.Default())
	r.MaxMultipartMemory = 8 << 20 // 8 MiB

	db.Init("./my_db.db")
	defer db.DB.Close()

	r.GET("/images", func(c *gin.Context) {
		userId := c.Query("user_id")

		images, err := db.GetImagesByUserId(userId)
		if err != nil {
			c.String(http.StatusInternalServerError, "Failed to fetch images: %s", err.Error())
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"data": images,
		})
	})

	r.POST("/delete-image", func(c *gin.Context) {
		userId := c.Request.FormValue("user_id")
		id := c.Request.FormValue("id")

		image, err := db.GetImageById(id)
		if err != nil {
			c.String(http.StatusNotFound, "Data not found")
			return
		}

		err = os.Remove(image.Path)
		if err != nil {
			c.String(http.StatusInternalServerError, fmt.Sprintf("Error when removing file: %s", err))
			return
		}

		err = db.DeleteImage(userId, id)
		if err != nil {
			c.String(http.StatusInternalServerError, "Error when deleting data")
			return
		}

		c.JSON(200, gin.H{
			"message": "Success delete",
		})
	})

	r.POST("/upload", func(c *gin.Context) {
		userId := c.Request.FormValue("user_id")
		file, err := c.FormFile("file")
		if err != nil {
			c.String(http.StatusBadRequest, "get form err: %s", err.Error())
			return
		}

		filename := filepath.Base(file.Filename)
		if err := c.SaveUploadedFile(file, "./uploads/"+filename); err != nil {
			c.String(http.StatusBadRequest, "Upload File err: %s", err.Error())
			return
		}

		db.CreateImage(userId, "./uploads/"+filename)

		c.JSON(200, gin.H{
			"message": "Success",
		})
	})

	r.POST("/chunk-upload", func(c *gin.Context) {
		userId := c.Request.FormValue("user_id")
		time.Sleep(2 * time.Second) // delay 2s

		file, err := c.FormFile("file")
		if err != nil {
			c.String(http.StatusBadRequest, "Get File Error err: %s", err.Error())
			return
		}

		openedFile, err := file.Open()
		if err != nil {
			c.String(http.StatusInternalServerError, "Error read file content: %s", err.Error())
			return
		}
		defer openedFile.Close()

		var metadata Metadata
		metadataJSON := c.Request.FormValue("metadata")
		err = json.Unmarshal([]byte(metadataJSON), &metadata)
		if err != nil {
			c.String(http.StatusBadRequest, "Invalid Metadata Format: %s", err.Error())
			return
		}

		// Checksum
		expectedChecksum := metadata.CheckSum

		hasher := sha256.New()
		if _, err := io.Copy(hasher, openedFile); err != nil {
			c.String(http.StatusInternalServerError, "Failed to read file content: %s", err.Error())
			return
		}
		computedChecksum := hex.EncodeToString(hasher.Sum(nil))

		if computedChecksum != expectedChecksum {
			c.String(422, "Mismatch checksum, please retry upload")
			return
		}

		if err := c.SaveUploadedFile(file, fmt.Sprintf("./uploads/temp/%v_%v", metadata.Order, metadata.FileId)); err != nil {
			c.String(http.StatusBadRequest, "Upload Chunk file err: %s", err.Error())
			return
		}

		if metadata.FileSize == metadata.Limit {
			chunks, err := filepath.Glob(filepath.Join("./uploads/temp", fmt.Sprintf("*_%s", metadata.FileId)))
			if err != nil {
				c.String(http.StatusBadRequest, "Error finding chunk: %s", err.Error())
				return
			}

			sort.Slice(chunks, func(i, j int) bool {
				orderI, _ := strconv.Atoi(string(filepath.Base(chunks[i])[0]))
				orderJ, _ := strconv.Atoi(string(filepath.Base(chunks[j])[0]))

				return orderI < orderJ
			})

			finalPath := filepath.Join("./uploads", metadata.FileName)
			finalFile, err := os.Create(finalPath)
			if err != nil {
				c.String(http.StatusBadRequest, "Error creating final file: %s", err.Error())
				return
			}
			defer finalFile.Close()

			for _, chunk := range chunks {
				chunkFile, err := os.Open(chunk)
				if err != nil {
					c.String(http.StatusBadRequest, "Error opening chunk file: %s", err.Error())
					return
				}

				_, err = io.Copy(finalFile, chunkFile)
				chunkFile.Close()

				if err != nil {
					c.String(http.StatusBadRequest, "Error merging chunk file: %s", err.Error())
					return
				}
			}

			// Clean up chunks
			for _, chunk := range chunks {
				os.Remove(chunk)
			}

			db.CreateImage(userId, finalPath)

			println("Chunk upload complete, merging chunkable files...")
		}

		c.SaveUploadedFile(file, "./uploads/temp/")
		c.JSON(200, gin.H{
			"message": "Success",
		})
	})

	r.Run()
}
