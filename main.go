package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"

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
}

func main() {
	r := gin.Default()

	r.Use(cors.Default())
	r.MaxMultipartMemory = 8 << 20 // 8 MiB

	r.POST("/upload", func(c *gin.Context) {
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

		c.JSON(200, gin.H{
			"message": "Success",
		})
	})

	r.POST("/chunk-upload", func(c *gin.Context) {
		file, err := c.FormFile("file")
		if err != nil {
			c.String(http.StatusBadRequest, "Get File Error err: %s", err.Error())
			return
		}

		var metadata Metadata
		metadataJSON := c.Request.FormValue("metadata")
		println(metadataJSON)
		err = json.Unmarshal([]byte(metadataJSON), &metadata)
		if err != nil {
			c.String(http.StatusBadRequest, "Invalid Metadata Format: %s", err.Error())
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

			finalPath := filepath.Join("./uploads", fmt.Sprintf("merged_%s", metadata.FileName))
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

			println("Chunk upload complete, merging chunkable files...")
		}

		c.SaveUploadedFile(file, "./uploads/temp/")
		c.JSON(200, gin.H{
			"message": "Success",
		})
	})

	r.Run()
}
