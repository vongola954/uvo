package services_test

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"uvo/internal/models"
	"uvo/internal/services"
)

func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:pl_" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Track{}, &models.Playlist{}, &models.PlaylistTrack{}, &models.SocialPost{}, &models.Like{}, &models.Comment{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestPlaylistHidesPrivateTracksForNonOwner(t *testing.T) {
	db := testDB(t)
	ps := services.NewPlaylistService(db)
	_ = db.Create(&models.Track{UserID: "alice", Title: "pub", FilePath: "/a", IsPublic: true}).Error
	_ = db.Create(&models.Track{UserID: "alice", Title: "priv", FilePath: "/b", IsPublic: false}).Error
	pl, err := ps.Create("alice", "mix", "", true)
	if err != nil {
		t.Fatal(err)
	}
	_ = ps.AddTrack("alice", pl.ID, 1)
	_ = ps.AddTrack("alice", pl.ID, 2)

	asOwner, err := ps.GetTracksForUser("alice", pl.ID)
	if err != nil || len(asOwner) != 2 {
		t.Fatalf("owner want 2 got %d err=%v", len(asOwner), err)
	}
	asGuest, err := ps.GetTracksForUser("bob", pl.ID)
	if err != nil || len(asGuest) != 1 || !asGuest[0].IsPublic {
		t.Fatalf("guest want 1 public got %#v err=%v", asGuest, err)
	}
}

func TestPlaylistSetPublicAndAddTrack(t *testing.T) {
	db := testDB(t)
	ps := services.NewPlaylistService(db)
	tr := &models.Track{UserID: "owner-u", Title: "t1", FilePath: "/t1-unique"}
	if err := db.Create(tr).Error; err != nil {
		t.Fatal(err)
	}
	pl, err := ps.Create("owner-u", "hits", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if pl.IsPublic {
		t.Fatal("want private")
	}
	pl, err = ps.SetPublic("owner-u", pl.ID, true)
	if err != nil || !pl.IsPublic {
		t.Fatalf("set public: %#v err=%v", pl, err)
	}
	var row models.Playlist
	_ = db.First(&row, pl.ID)
	if !row.IsPublic {
		t.Fatal("db is_public not persisted")
	}
	if err := ps.AddTrack("owner-u", pl.ID, tr.ID); err != nil {
		t.Fatal(err)
	}
	if err := ps.AddTrack("owner-u", pl.ID, tr.ID); err != nil {
		t.Fatal("duplicate add should be idempotent")
	}
	tracks, err := ps.GetTracks(pl.ID)
	if err != nil || len(tracks) != 1 {
		t.Fatalf("tracks=%d err=%v", len(tracks), err)
	}
}

func TestFeedOnlyPublicTracks(t *testing.T) {
	db := testDB(t)
	ss := services.NewSocialService(db)
	_ = db.Create(&models.Track{UserID: "alice", Title: "p", FilePath: "/p", IsPublic: false}).Error
	post, err := ss.CreatePost("alice", 1, "hi")
	if err != nil {
		t.Fatal(err)
	}
	var tr models.Track
	_ = db.First(&tr, 1)
	if !tr.IsPublic {
		t.Fatal("create post should make track public")
	}
	feed, err := ss.Feed(10)
	if err != nil || len(feed) != 1 || feed[0].ID != post.ID {
		t.Fatalf("feed %#v err=%v", feed, err)
	}
	_ = db.Model(&tr).Update("is_public", false)
	feed2, _ := ss.Feed(10)
	if len(feed2) != 0 {
		t.Fatalf("private track post should be hidden, got %d", len(feed2))
	}
}
