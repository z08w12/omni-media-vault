package api

import (
	"net/http"

	"github.com/video-site/backend/internal/catalog"
)

type PhotoAlbumDTO struct {
	Tag             string         `json:"tag"`
	Title           string         `json:"title"`
	Count           int            `json:"count"`
	CoverPhoto      *VideoCardDTO  `json:"coverPhoto,omitempty"`
	SamplePhotos    []VideoCardDTO `json:"samplePhotos"`
	LatestTimestamp int64          `json:"latestTimestamp"`
}

func (s *Server) handlePhotoAlbums(w http.ResponseWriter, r *http.Request) {
	albums, err := s.Catalog.ListPhotoAlbums(r.Context())
	if err != nil {
		writeErr(w, r, http.StatusInternalServerError, err)
		return
	}
	out := make([]PhotoAlbumDTO, 0, len(albums))
	for _, album := range albums {
		var coverDTO *VideoCardDTO
		if album.CoverPhoto != nil {
			cards := mapVideoSummaries([]*catalog.VideoSummary{album.CoverPhoto})
			if len(cards) > 0 {
				coverDTO = &cards[0]
			}
		}
		sampleDTOs := mapVideoSummaries(album.SamplePhotos)
		out = append(out, PhotoAlbumDTO{
			Tag:             album.Tag,
			Title:           album.Title,
			Count:           album.Count,
			CoverPhoto:      coverDTO,
			SamplePhotos:    sampleDTOs,
			LatestTimestamp: album.LatestTimestamp,
		})
	}
	w.Header().Set("Cache-Control", "private, max-age=15")
	writeJSON(w, http.StatusOK, out)
}
