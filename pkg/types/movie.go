package types

import "time"

type Movie struct {
	ID              uuid.UUID  `json:"id" validate:"required,uuid"`
	Title           string     `json:"title" validate:"required"`
	OriginalTitle   *string    `json:"original_title" validate:"omitempty"`
	Thumbnail       *string    `json:"thumbnail" validate:"omitempty"`
	Synopsis        *string    `json:"synopsis" validate:"omitempty"`
	Rating          *float64   `json:"rating" validate:"omitempty"`
	Duration        *int64     `json:"duration" validate:"omitempty"`
	Year            *int32     `json:"year" validate:"omitempty"`
	DatePublished   *time.Time `json:"date_published" validate:"omitempty"`
	LabelQuality    *string    `json:"label_quality" validate:"omitempty"`
	Genre           *string    `json:"genre" validate:"omitempty"`
	OriginalPageUrl *string    `json:"original_page_url" validate:"omitempty"`
}

type MovieDetail struct {
	Movie
	Votes         *int64          `json:"votes" validate:"omitempty"`
	ReleaseDate   *time.Time      `json:"release_date" validate:"omitempty"`
	UpdatedAt     *time.Time      `json:"updated_at" validate:"omitempty"`
	PlayerUrl     *[]PlayerUrl    `json:"player_url" validate:"omitempty"`
	TrailerUrl    *string         `json:"trailer_url" validate:"omitempty"`
	Director      *[]MoviePerson  `json:"director" validate:"omitempty"`
	MovieStar     *[]MoviePerson  `json:"movie_star" validate:"omitempty"`
	Countries     *[]CountryMovie `json:"countries" validate:"omitempty"`
	Genres        *[]Genre        `json:"genres" validate:"omitempty"`
	SimilarMovies *[]Movie        `json:"similar_movies" validate:"omitempty"`
	DownloadLink  *string         `json:"download_link" validate:"omitempty"`
	Type          string          `json:"type" validate:"required"` // 'movie' | 'series'
	SourceUrl     *string         `json:"source_url" validate:"omitempty"`
	ResolvedFrom  *string         `json:"resolved_from_url" validate:"omitempty"`
}

type SeriesDetail struct {
	Movie
	SeasonName    *string         `json:"season_name" validate:"omitempty"` // Name of the season (e.g., "Season 1")
	Status        *string         `json:"status" validate:"omitempty"`      // Status of the series (e.g., "Ongoing", "Completed")
	SeasonList    *[]SeasonList   `json:"season_list" validate:"omitempty"` // List of seasons
	Votes         *int64          `json:"votes" validate:"omitempty"`
	ReleaseDate   *time.Time      `json:"release_date" validate:"omitempty"`
	UpdatedAt     *time.Time      `json:"updated_at" validate:"omitempty"`
	Director      *[]MoviePerson  `json:"director" validate:"omitempty"`
	MovieStar     *[]MoviePerson  `json:"movie_star" validate:"omitempty"`
	Countries     *[]CountryMovie `json:"countries" validate:"omitempty"`
	Genres        *[]Genre        `json:"genres" validate:"omitempty"`
	SimilarMovies *[]Movie        `json:"similar_movies" validate:"omitempty"`
	DownloadLink  *string         `json:"download_link" validate:"omitempty"`
	Type          string          `json:"type" validate:"required"` // 'movie' | 'series'
	SourceUrl     *string         `json:"source_url" validate:"omitempty"`
	ResolvedFrom  *string         `json:"resolved_from_url" validate:"omitempty"`
}

type MoviePerson struct {
	Name    *string `json:"name" validate:"omitempty"`
	PageUrl *string `json:"page_url" validate:"omitempty"`
}

type PlayerUrl struct {
	URL  *string `json:"url" validate:"omitempty"`
	Type *string `json:"type" validate:"omitempty"`
}

type CountryMovie struct {
	Name    *string `json:"name" validate:"omitempty"`
	PageUrl *string `json:"page_url" validate:"omitempty"`
}

type Genre struct {
	Name    *string `json:"name" validate:"omitempty"`
	PageUrl *string `json:"page_url" validate:"omitempty"`
}

type SeasonList struct {
	CurrentSeason *int32         `json:"current_season" validate:"omitempty"` // Current season number (e.g., 1)
	TotalSeason   *int32         `json:"total_season" validate:"omitempty"`   // Total number of seasons (e.g., 5)
	EpisodeList   *[]EpisodeList `json:"episode_list" validate:"omitempty"`   // List of episodes in the season
}

type EpisodeList struct {
	EpisodeNumber *int32       `json:"episode_number" validate:"omitempty"` // Episode number (e.g., 1)
	EpisodeUrl    *string      `json:"episode_url" validate:"omitempty"`    // URL to watch the episode
	PlayerUrl     *[]PlayerUrl `json:"player_url" validate:"omitempty"`
	TrailerUrl    *string      `json:"trailer_url" validate:"omitempty"`
}

type SeriesEpisode struct {
	EpisodeUrl    *string      `json:"episode_url" validate:"omitempty"`
	EpisodeNumber *int32       `json:"episode_number" validate:"omitempty"`
	PlayerUrl     *[]PlayerUrl `json:"player_url" validate:"omitempty"`
	TrailerUrl    *string      `json:"trailer_url" validate:"omitempty"`
	DownloadUrl   *string      `json:"download_url" validate:"omitempty"`
}
