package handlers

import (
	"testing"

	"faka/server/internal/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCountFreeFiles(t *testing.T) {
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.AutoMigrate(&models.Space{}, &models.ManagedFile{}, &models.Card{}); err != nil {
		t.Fatal(err)
	}

	space := models.Space{Name: "primary", CardPrefix: "MAIN"}
	otherSpace := models.Space{Name: "other", CardPrefix: "OTHER"}
	if err := database.Create(&space).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&otherSpace).Error; err != nil {
		t.Fatal(err)
	}

	files := []models.ManagedFile{
		{OriginalName: "a", StoredPath: "a", SpaceID: space.ID, Status: "available"},
		{OriginalName: "b", StoredPath: "b", SpaceID: space.ID, Status: "available"},
		{OriginalName: "c", StoredPath: "c", SpaceID: space.ID, Status: "available"},
		{OriginalName: "sold", StoredPath: "sold", SpaceID: space.ID, Status: "sold"},
		{OriginalName: "other", StoredPath: "other", SpaceID: otherSpace.ID, Status: "available"},
	}
	if err := database.Create(&files).Error; err != nil {
		t.Fatal(err)
	}

	cards := []models.Card{
		{Code: "PENDING", SpaceID: space.ID, FileCount: 5, Status: "pending"},
		{Code: "AVAILABLE", SpaceID: space.ID, FileCount: 10, Status: "available"},
		{Code: "OTHER", SpaceID: otherSpace.ID, FileCount: 20, Status: "pending"},
	}
	if err := database.Create(&cards).Error; err != nil {
		t.Fatal(err)
	}

	freeFiles, err := countFreeFiles(database, space.ID)
	if err != nil {
		t.Fatal(err)
	}
	if freeFiles != -2 {
		t.Fatalf("expected -2 free files, got %d", freeFiles)
	}
}
