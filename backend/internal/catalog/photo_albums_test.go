package catalog

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestPhotoAlbumsAndBatchTags(t *testing.T) {
	ctx := context.Background()
	cat, err := Open(t.TempDir() + "/photo_albums_test.db")
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	defer cat.Close()

	now := time.Now()
	// Insert 3 image videos and 1 video
	img1 := &Video{
		ID:          "img-1",
		DriveID:     "drive-1",
		FileID:      "file-1",
		Title:       "Image 1",
		Ext:         "jpg",
		PublishedAt: now.Add(time.Second),
		CreatedAt:   now.Add(time.Second),
		UpdatedAt:   now.Add(time.Second),
	}
	img2 := &Video{
		ID:          "img-2",
		DriveID:     "drive-1",
		FileID:      "file-2",
		Title:       "Image 2",
		Ext:         "png",
		PublishedAt: now.Add(2 * time.Second),
		CreatedAt:   now.Add(2 * time.Second),
		UpdatedAt:   now.Add(2 * time.Second),
	}
	img3 := &Video{
		ID:          "img-3",
		DriveID:     "drive-1",
		FileID:      "file-3",
		Title:       "Image 3",
		Ext:         "webp",
		PublishedAt: now.Add(3 * time.Second),
		CreatedAt:   now.Add(3 * time.Second),
		UpdatedAt:   now.Add(3 * time.Second),
	}
	vid1 := &Video{
		ID:              "vid-1",
		DriveID:         "drive-1",
		FileID:          "file-vid-1",
		Title:           "Video 1",
		Ext:             "mp4",
		DurationSeconds: 60,
		PublishedAt:     now,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	for _, v := range []*Video{img1, img2, img3, vid1} {
		if err := cat.UpsertVideo(ctx, v); err != nil {
			t.Fatalf("upsert video %s: %v", v.ID, err)
		}
	}

	// 1. Batch tag: add "空心柚七" to img1 and img2
	count, err := cat.BatchUpdateVideoTags(ctx, []string{"img-1", "img-2"}, "add", []string{"空心柚七"})
	if err != nil {
		t.Fatalf("batch update tags: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 updated, got %d", count)
	}

	// 2. Also add system tag "图片" to all 3 images
	_, err = cat.BatchUpdateVideoTags(ctx, []string{"img-1", "img-2", "img-3"}, "add", []string{"图片", "TG"})
	if err != nil {
		t.Fatalf("batch add system tags: %v", err)
	}

	// Verify img1 tags
	v1, err := cat.GetVideo(ctx, "img-1")
	if err != nil {
		t.Fatalf("get img-1: %v", err)
	}
	if len(v1.Tags) != 3 {
		t.Fatalf("expected 3 tags on img-1, got %v", v1.Tags)
	}

	// 3. Test ListPhotoAlbums
	albums, err := cat.ListPhotoAlbums(ctx)
	if err != nil {
		t.Fatalf("ListPhotoAlbums: %v", err)
	}

	// Should have:
	// 1 album: "#空心柚七" with count 2
	// 1 album: "未分类图集" with count 1 (img-3 has only system tags)
	if len(albums) != 2 {
		t.Fatalf("expected 2 albums, got %d: %+v", len(albums), albums)
	}

	if albums[0].Tag != "空心柚七" || albums[0].Count != 2 {
		t.Fatalf("expected album 0 to be 空心柚七 (2), got %s (%d)", albums[0].Tag, albums[0].Count)
	}
	if albums[1].Tag != "未分类" || albums[1].Count != 1 {
		t.Fatalf("expected album 1 to be 未分类 (1), got %s (%d)", albums[1].Tag, albums[1].Count)
	}

	// 4. Test BatchUpdateVideoTags remove
	_, err = cat.BatchUpdateVideoTags(ctx, []string{"img-1"}, "remove", []string{"空心柚七"})
	if err != nil {
		t.Fatalf("batch remove tag: %v", err)
	}
	v1After, _ := cat.GetVideo(ctx, "img-1")
	for _, tName := range v1After.Tags {
		if tName == "空心柚七" {
			t.Fatalf("tag 空心柚七 should have been removed from img-1")
		}
	}
}

func TestPhotoAlbumsCacheAndWindowSampling(t *testing.T) {
	ctx := context.Background()
	cat, err := Open(t.TempDir() + "/photo_albums_window_test.db")
	if err != nil {
		t.Fatalf("open catalog: %v", err)
	}
	defer cat.Close()

	now := time.Now()
	// Insert 6 image items for the same tag "风景"
	for i := 1; i <= 6; i++ {
		img := &Video{
			ID:          fmt.Sprintf("img-%d", i),
			DriveID:     "drive-1",
			FileID:      fmt.Sprintf("file-%d", i),
			Title:       fmt.Sprintf("Image %d", i),
			Ext:         "jpg",
			PublishedAt: now.Add(time.Duration(i) * time.Second),
			CreatedAt:   now.Add(time.Duration(i) * time.Second),
			UpdatedAt:   now.Add(time.Duration(i) * time.Second),
			MediaKind:   "image",
		}
		if err := cat.UpsertVideo(ctx, img); err != nil {
			t.Fatalf("upsert img %d: %v", i, err)
		}
	}

	_, err = cat.BatchUpdateVideoTags(ctx, []string{"img-1", "img-2", "img-3", "img-4", "img-5", "img-6"}, "add", []string{"风景"})
	if err != nil {
		t.Fatalf("batch update tags: %v", err)
	}

	// First call to ListPhotoAlbums (computes & caches)
	albums, err := cat.ListPhotoAlbums(ctx)
	if err != nil {
		t.Fatalf("ListPhotoAlbums: %v", err)
	}
	if len(albums) != 1 {
		t.Fatalf("expected 1 album, got %d", len(albums))
	}
	album := albums[0]
	if album.Tag != "风景" || album.Count != 6 {
		t.Fatalf("album tag=%s, count=%d, want 风景/6", album.Tag, album.Count)
	}
	// Window function should limit samples to exactly 4, newest first: img-6, img-5, img-4, img-3
	if len(album.SamplePhotos) != 4 {
		t.Fatalf("expected 4 sample photos, got %d", len(album.SamplePhotos))
	}
	expectedOrder := []string{"img-6", "img-5", "img-4", "img-3"}
	for i, exp := range expectedOrder {
		if album.SamplePhotos[i].ID != exp {
			t.Fatalf("sample photo[%d] = %s, want %s", i, album.SamplePhotos[i].ID, exp)
		}
	}
	if album.CoverPhoto == nil || album.CoverPhoto.ID != "img-6" {
		t.Fatalf("expected cover photo img-6, got %v", album.CoverPhoto)
	}

	// Verify cached read returns exact same results
	cachedAlbums, err := cat.ListPhotoAlbums(ctx)
	if err != nil {
		t.Fatalf("cached ListPhotoAlbums: %v", err)
	}
	if len(cachedAlbums) != 1 || cachedAlbums[0].Count != 6 {
		t.Fatalf("cache read mismatch: %+v", cachedAlbums)
	}

	// Test cache invalidation: add an 7th image
	img7 := &Video{
		ID:          "img-7",
		DriveID:     "drive-1",
		FileID:      "file-7",
		Title:       "Image 7",
		Ext:         "png",
		PublishedAt: now.Add(7 * time.Second),
		CreatedAt:   now.Add(7 * time.Second),
		UpdatedAt:   now.Add(7 * time.Second),
		MediaKind:   "image",
	}
	if err := cat.UpsertVideo(ctx, img7); err != nil {
		t.Fatalf("upsert img 7: %v", err)
	}
	_, err = cat.BatchUpdateVideoTags(ctx, []string{"img-7"}, "add", []string{"风景"})
	if err != nil {
		t.Fatalf("tag img 7: %v", err)
	}

	// ListPhotoAlbums should reflect count 7 and new top 4 (img-7, img-6, img-5, img-4)
	albumsAfter, err := cat.ListPhotoAlbums(ctx)
	if err != nil {
		t.Fatalf("ListPhotoAlbums after update: %v", err)
	}
	if albumsAfter[0].Count != 7 {
		t.Fatalf("expected count 7 after invalidation, got %d", albumsAfter[0].Count)
	}
	if albumsAfter[0].SamplePhotos[0].ID != "img-7" {
		t.Fatalf("expected sample[0] to be img-7, got %s", albumsAfter[0].SamplePhotos[0].ID)
	}
}
