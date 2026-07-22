package services

import (
	"archive/zip"
	"path/filepath"
	"testing"
	"time"

	"faka/server/internal/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Space{}, &models.ManagedFile{}, &models.Card{}, &models.Redemption{}, &models.AuditLog{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestWeightedPickBounded(t *testing.T) {
	files := make([]models.ManagedFile, 0, 100)
	base := time.Now().UTC()
	for i := 0; i < 100; i++ {
		files = append(files, models.ManagedFile{
			ID:         uint(i + 1),
			UploadedAt: base.Add(time.Duration(i) * time.Minute),
		})
	}
	picked := WeightedPick(files, 10)
	if len(picked) != 10 {
		t.Fatalf("expected 10, got %d", len(picked))
	}
	seen := map[uint]struct{}{}
	for _, p := range picked {
		if _, ok := seen[p.ID]; ok {
			t.Fatalf("duplicate id %d", p.ID)
		}
		seen[p.ID] = struct{}{}
	}
	all := WeightedPick(files, 1000)
	if len(all) != 100 {
		t.Fatalf("expected full pool 100, got %d", len(all))
	}
}

func TestCreateCardsAndRedeemCPA(t *testing.T) {
	db := testDB(t)
	space := &models.Space{Name: "default", CardPrefix: "TEST"}
	if err := db.Create(space).Error; err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	// seed 5 available files
	for i := 0; i < 5; i++ {
		name := filepath.Join(dir, "f"+string(rune('a'+i))+".json")
		if err := writeFile(name, []byte(`{"access_token":"a.b.c","email":"u`+string(rune('0'+i))+`@x.com"}`)); err != nil {
			t.Fatal(err)
		}
		item := models.ManagedFile{
			OriginalName: filepath.Base(name),
			StoredPath:   name,
			GeneratedAt:  time.Now().UTC(),
			UploadedAt:   time.Now().UTC(),
			SpaceID:      space.ID,
			Status:       "available",
		}
		if err := db.Create(&item).Error; err != nil {
			t.Fatal(err)
		}
	}
	cards, err := CreateCards(db, space, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 2 {
		t.Fatalf("cards=%d", len(cards))
	}
	// mark pending
	if err := db.Model(&models.Card{}).Where("id IN ?", []uint{cards[0].ID, cards[1].ID}).Update("status", "pending").Error; err != nil {
		t.Fatal(err)
	}
	out, err := RedeemCardsCPA(db, dir, cards[0].Code)
	if err != nil {
		t.Fatal(err)
	}
	if out == "" {
		t.Fatal("empty archive path")
	}
	var sold int64
	db.Model(&models.ManagedFile{}).Where("status = ?", "sold").Count(&sold)
	if sold != 2 {
		t.Fatalf("sold files=%d", sold)
	}
}

func TestWriteZipFromPicksFlattensMultipleCards(t *testing.T) {
	dir := t.TempDir()
	firstPath := filepath.Join(dir, "first.json")
	secondPath := filepath.Join(dir, "second.json")
	if err := writeFile(firstPath, []byte(`{"account":"first"}`)); err != nil {
		t.Fatal(err)
	}
	if err := writeFile(secondPath, []byte(`{"account":"second"}`)); err != nil {
		t.Fatal(err)
	}

	cards := []models.Card{{ID: 1, Code: "TEST-FIRST"}, {ID: 2, Code: "TEST-SECOND"}}
	picks := map[uint][]models.ManagedFile{
		1: {{ID: 1, OriginalName: "account.json", StoredPath: firstPath}},
		2: {{ID: 2, OriginalName: "account.json", StoredPath: secondPath}},
	}
	out := filepath.Join(dir, "accounts.zip")
	if err := writeZipFromPicks(out, cards, picks); err != nil {
		t.Fatal(err)
	}

	zr, err := zip.OpenReader(out)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()

	want := []string{"account.json", "2_account.json"}
	if len(zr.File) != len(want) {
		t.Fatalf("zip entries=%d, want %d", len(zr.File), len(want))
	}
	for i, entry := range zr.File {
		if entry.Name != want[i] {
			t.Fatalf("zip entry %d=%q, want %q", i, entry.Name, want[i])
		}
	}
}

func TestParseCardCodes(t *testing.T) {
	_, err := ParseCardCodes("BAD")
	if err == nil {
		t.Fatal("expected format error")
	}
	codes, err := ParseCardCodes("TEST-ABCDEFGHIJKLMNOPQRSTUVWXYZ012345")
	if err != nil {
		t.Fatal(err)
	}
	if len(codes) != 1 {
		t.Fatalf("len=%d", len(codes))
	}
}
