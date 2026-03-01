package types

import "github.com/gofrs/uuid"

type Anime struct {
	ID              uuid.UUID     `json:"id" validate:"required,uuid"`
	Title           *string       `json:"title" validate:"omitempty"`
	TitleJapanese   *string       `json:"title_japanese" validate:"omitempty"`
	OriginalPageURL *string       `json:"original_page_url" validate:"omitempty"`
	Thumbnail       *string       `json:"thumbnail" validate:"omitempty"`
	Score           *string       `json:"score" validate:"omitempty"`
	Producer        *string       `json:"producer" validate:"omitempty"`
	Type            *string       `json:"type" validate:"omitempty"`
	Status          *string       `json:"status" validate:"omitempty"`
	TotalEpisodes   *string       `json:"total_episodes" validate:"omitempty"`
	Duration        *string       `json:"duration" validate:"omitempty"`
	ReleaseDate     *string       `json:"release_date" validate:"omitempty"`
	ReleasedDay     *string       `json:"released_day" validate:"omitempty"`
	Studio          *string       `json:"studio" validate:"omitempty"`
	Rating          *string       `json:"rating" validate:"omitempty"`
	Genre           *[]AnimeGenre `json:"genre" validate:"omitempty"`
	Episodes        *[]Episode    `json:"episodes" validate:"omitempty"`
}

type AnimeGenre struct {
	Name *string `json:"name" validate:"omitempty"`
	URL  *string `json:"url" validate:"omitempty"`
}

type Episode struct {
	ID                 uuid.UUID        `json:"id" validate:"required,uuid"`
	Title              *string          `json:"title" validate:"omitempty"`
	PlayerURL          *string          `json:"player_url" validate:"omitempty"`
	PageURL            *string          `json:"page_url" validate:"omitempty"`
	PostedBy           *string          `json:"posted_by" validate:"omitempty"`
	PreviousEpisodeURL *string          `json:"previous_episode_url" validate:"omitempty"`
	NextEpisodeURL     *string          `json:"next_episode_url" validate:"omitempty"`
	SeeAllEpisodesURL  *string          `json:"see_all_episodes_url" validate:"omitempty"`
	ReleaseDate        *string          `json:"release_date" validate:"omitempty"`
	ReleaseTime        *string          `json:"release_time" validate:"omitempty"`
	EpisodeNumber      *string          `json:"episode_number" validate:"omitempty"`
	ListEpisode        *[]ListOfEpisode `json:"list_episode" validate:"omitempty"`
	DownloadLinks      *[]DownloadLink  `json:"download_links" validate:"omitempty"`
}

type ListOfEpisode struct {
	Name    *string `json:"name" validate:"omitempty"`
	PageUrl *string `json:"page_url" validate:"omitempty"`
}

type DownloadLink struct {
	Name    *string `json:"name" validate:"omitempty"`
	URL     *string `json:"url" validate:"omitempty"`
	Size    *string `json:"size" validate:"omitempty"`
	Quality *string `json:"quality" validate:"omitempty"`
	Format  *string `json:"format" validate:"omitempty"`
}

type PaginationAnime struct {
	TotalPages      int     `json:"total_pages"`
	CurrentPage     int     `json:"current_page"`
	HasNext         bool    `json:"has_next"`
	HasPrevious     bool    `json:"has_previous"`
	NextPageURL     *string `json:"next_page_url" validate:"omitempty"`
	PreviousPageURL *string `json:"previous_page_url" validate:"omitempty"`
	PageNumbers     []int   `json:"page_numbers"`
	PerPage         int     `json:"per_page"`
}

type ScrapeResult struct {
	Animes     []Anime         `json:"animes"`
	Pagination PaginationAnime `json:"pagination"`
	Query      *string         `json:"query" validate:"omitempty"`
}

type HomeAnimeResponse struct {
	Key        string  `json:"key"`
	Value      []Anime `json:"value"`
	ViewAllUrl *string `json:"view_all_url" validate:"omitempty"`
}
