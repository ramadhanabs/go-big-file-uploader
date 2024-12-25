package db

import (
	"database/sql"
	"fmt"
	"log"
)

type Images struct {
	ID        int    `json:"id"`
	UserId    string `json:"user_id"`
	Path      string `json:"path"`
	CreatedAt string `json:"created_at"`
}

func CreateImage(userId string, path string) error {
	query := "INSERT INTO images (user_id, path) VALUES (?, ?)"
	_, err := DB.Exec(query, userId, path)

	if err != nil {
		log.Printf("Failed to create image: %v", err)
	}

	return err
}

func DeleteImage(userId string, id string) error {
	query := "DELETE FROM images WHERE user_id = ? AND id = ?"
	_, err := DB.Exec(query, userId, id)

	if err != nil {
		log.Printf("Failed to delete image: %v", err)
	}

	return err
}

func GetImageById(id string) (Images, error) {
	query := "SELECT id, user_id, path, created_at FROM images WHERE id = ?"
	var image Images

	row := DB.QueryRow(query, id)

	err := row.Scan(&image.ID, &image.UserId, &image.Path, &image.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return Images{}, fmt.Errorf("no image found with ID: %s", id)
		}
		return Images{}, err
	}

	return image, nil
}

func GetImagesByUserId(userId string) ([]Images, error) {
	query := "SELECT * FROM images WHERE user_id = ?"
	rows, err := DB.Query(query, userId)
	if err != nil {
		return nil, err
	}

	var images []Images
	for rows.Next() {
		var image Images
		if err := rows.Scan(&image.ID, &image.UserId, &image.Path, &image.CreatedAt); err != nil {
			return nil, err
		}
		images = append(images, image)
	}

	return images, nil
}
