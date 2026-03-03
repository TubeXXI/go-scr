package scraper

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"tubexxi/scraper/pkg/logger"
	"tubexxi/scraper/pkg/types"

	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/chromedp"
	"github.com/gofrs/uuid"
	"go.uber.org/zap"
)

type AnimeClient struct {
	ChromeClient *ChromeClient
	HTTPClient   *http.Client
	BaseURL      string
}

func NewAnimeClient() *AnimeClient {
	return &AnimeClient{
		ChromeClient: NewChromeClient(),
		HTTPClient:   &http.Client{Timeout: 30 * time.Second},
		BaseURL:      AnimeBaseURL,
	}
}

func (ac *AnimeClient) Close() {
	if ac.ChromeClient != nil {
		ac.ChromeClient.Close()
	}
}

// GetLatest fetches the latest anime list
func (ac *AnimeClient) GetLatest(page int) (*types.ScrapeResult, error) {
	var url string
	if page <= 1 {
		url = fmt.Sprintf("%s/?post_type=anime", ac.BaseURL)
	} else {
		url = fmt.Sprintf("%s/page/%d/?post_type=anime", ac.BaseURL, page)
	}

	html, err := ac.FetchHTML(url)
	if err != nil {
		logger.Logger.Error("Error fetching latest anime", zap.Error(err))
		return nil, err
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		logger.Logger.Error("Error parsing latest anime HTML", zap.Error(err))
		return nil, err
	}

	animes := ac.extractArchiveAnimes(doc)
	pagination := ac.parsePaginationArchive(doc, page)
	pagination.PerPage = len(animes)
	if pagination.PerPage == 0 {
		pagination.PerPage = 20
	}

	return &types.ScrapeResult{
		Animes:     animes,
		Pagination: pagination,
	}, nil
}

// GetOngoing fetches ongoing anime list
func (ac *AnimeClient) GetOngoing(page int) (*types.ScrapeResult, error) {
	var url string
	if page <= 1 {
		url = fmt.Sprintf("%s/ongoing-anime/", ac.BaseURL)
	} else {
		url = fmt.Sprintf("%s/ongoing-anime/page/%d/", ac.BaseURL, page)
	}

	html, err := ac.FetchHTML(url)
	if err != nil {
		logger.Logger.Error("Error fetching ongoing anime", zap.Error(err))
		return nil, err
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		logger.Logger.Error("Error parsing ongoing anime HTML", zap.Error(err))
		return nil, err
	}

	animes := ac.extractOngoingAnimes(doc)
	pagination := ac.parsePaginationOngoing(doc, page)
	pagination.PerPage = len(animes)
	if pagination.PerPage == 0 {
		pagination.PerPage = 20
	}

	return &types.ScrapeResult{
		Animes:     animes,
		Pagination: pagination,
	}, nil
}

// GetComplete fetches complete anime list
func (ac *AnimeClient) GetComplete(page int) (*types.ScrapeResult, error) {
	var url string
	if page <= 1 {
		url = fmt.Sprintf("%s/complete-anime/", ac.BaseURL)
	} else {
		url = fmt.Sprintf("%s/complete-anime/page/%d/", ac.BaseURL, page)
	}

	html, err := ac.FetchHTML(url)
	if err != nil {
		logger.Logger.Error("Error fetching complete anime", zap.Error(err))
		return nil, err
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		logger.Logger.Error("Error parsing complete anime HTML", zap.Error(err))
		return nil, err
	}

	animes := ac.extractCompleteAnimes(doc)
	pagination := ac.parsePaginationOngoing(doc, page)
	pagination.PerPage = len(animes)
	if pagination.PerPage == 0 {
		pagination.PerPage = 20
	}

	return &types.ScrapeResult{
		Animes:     animes,
		Pagination: pagination,
	}, nil
}

// Search searches for anime by query
func (ac *AnimeClient) Search(query string, page int) (*types.ScrapeResult, error) {
	var url string
	if page <= 1 {
		url = fmt.Sprintf("%s/?s=%s&post_type=anime", ac.BaseURL, query)
	} else {
		url = fmt.Sprintf("%s/?s=%s&post_type=anime&page=%d", ac.BaseURL, query, page)
	}

	html, err := ac.FetchHTML(url)
	if err != nil {
		logger.Logger.Error("Error searching anime", zap.Error(err))
		return nil, err
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		logger.Logger.Error("Error parsing search results HTML", zap.Error(err))
		return nil, err
	}

	animes := ac.extractArchiveAnimes(doc)
	pagination := ac.parsePaginationArchive(doc, page)
	pagination.PerPage = len(animes)
	if pagination.PerPage == 0 {
		pagination.PerPage = 20
	}

	return &types.ScrapeResult{
		Animes:     animes,
		Pagination: pagination,
		Query:      &query,
	}, nil
}

// GetGenres fetches the list of anime genres
func (ac *AnimeClient) GetGenres() ([]types.AnimeGenre, error) {
	url := fmt.Sprintf("%s/genre-list/", ac.BaseURL)

	html, err := ac.FetchHTML(url)
	if err != nil {
		logger.Logger.Error("Error fetching genres", zap.Error(err))
		return nil, err
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		logger.Logger.Error("Error parsing genres HTML", zap.Error(err))
		return nil, err
	}

	var genres []types.AnimeGenre
	seen := make(map[string]bool)

	doc.Find("a[href]").Each(func(i int, a *goquery.Selection) {
		href, _ := a.Attr("href")
		if href == "" {
			return
		}

		if !strings.Contains(href, "/genres/") {
			return
		}

		name := strings.TrimSpace(a.Text())
		absURL := ac.makeAbsoluteURL(href)

		if absURL == "" || name == "" {
			return
		}

		key := strings.ToLower(name) + "|" + absURL
		if seen[key] {
			return
		}
		seen[key] = true

		genres = append(genres, types.AnimeGenre{
			Name: &name,
			URL:  &absURL,
		})
	})

	return genres, nil
}

// GetAnimeDetail fetches detailed information about an anime
func (ac *AnimeClient) GetAnimeDetail(url string) (*types.Anime, error) {
	html, err := ac.FetchHTML(url)
	if err != nil {
		logger.Logger.Error("Error fetching anime detail", zap.Error(err))
		return nil, err
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		logger.Logger.Error("Error parsing anime detail HTML", zap.Error(err))
		return nil, err
	}

	return ac.scrapeAnimeDetail(doc, url), nil
}

func (ac *AnimeClient) scrapeAnimeDetail(doc *goquery.Document, url string) *types.Anime {
	id := uuid.Must(uuid.NewV4())
	anime := &types.Anime{
		ID:              id,
		OriginalPageURL: &url,
	}

	// Title
	title := doc.Find(".jdlflm").Text()
	if title == "" {
		title = doc.Find("h1").Text()
	}
	anime.Title = ac.stringPtr(strings.TrimSpace(title))

	// Get metadata from .info-anime-prod
	doc.Find(".info-anime-prod .info .set").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())

		if strings.Contains(text, "Japanese") {
			value := strings.ReplaceAll(text, "Japanese :", "")
			anime.TitleJapanese = ac.stringPtr(strings.TrimSpace(value))
		} else if strings.Contains(text, "Producer") {
			value := strings.ReplaceAll(text, "Producer :", "")
			anime.Producer = ac.stringPtr(strings.TrimSpace(value))
		} else if strings.Contains(text, "Type") {
			value := strings.ReplaceAll(text, "Type :", "")
			anime.Type = ac.stringPtr(strings.TrimSpace(value))
		} else if strings.Contains(text, "Status") {
			value := strings.ReplaceAll(text, "Status :", "")
			anime.Status = ac.stringPtr(strings.TrimSpace(value))
		} else if strings.Contains(text, "Episode") {
			value := strings.ReplaceAll(text, "Episode :", "")
			anime.TotalEpisodes = ac.stringPtr(strings.TrimSpace(value))
		} else if strings.Contains(text, "Duration") {
			value := strings.ReplaceAll(text, "Duration :", "")
			anime.Duration = ac.stringPtr(strings.TrimSpace(value))
		} else if strings.Contains(text, "Release") {
			value := strings.ReplaceAll(text, "Release :", "")
			anime.ReleaseDate = ac.stringPtr(strings.TrimSpace(value))
		} else if strings.Contains(text, "Studio") {
			value := strings.ReplaceAll(text, "Studio :", "")
			anime.Studio = ac.stringPtr(strings.TrimSpace(value))
		} else if strings.Contains(text, "Score") {
			value := strings.ReplaceAll(text, "Score :", "")
			anime.Score = ac.stringPtr(strings.TrimSpace(value))
		}
	})

	// Thumbnail
	thumb := doc.Find(".thumbz img").First()
	if src, ok := thumb.Attr("src"); ok {
		anime.Thumbnail = ac.stringPtr(src)
	} else if srcset, ok := thumb.Attr("srcset"); ok {
		parts := strings.Split(srcset, ",")
		if len(parts) > 0 {
			imgURL := strings.TrimSpace(strings.Split(parts[0], " ")[0])
			anime.Thumbnail = ac.stringPtr(imgURL)
		}
	}

	// Genres
	var genres []types.AnimeGenre
	doc.Find(".genre-info a").Each(func(i int, a *goquery.Selection) {
		href, _ := a.Attr("href")
		name := strings.TrimSpace(a.Text())
		if name != "" {
			genre := types.AnimeGenre{
				Name: &name,
			}
			if href != "" {
				genre.URL = ac.stringPtr(ac.makeAbsoluteURL(href))
			}
			genres = append(genres, genre)
		}
	})
	if len(genres) > 0 {
		anime.Genre = &genres
	}

	// Episodes
	episodes := ac.extractEpisodes(doc)
	if len(episodes) > 0 {
		anime.Episodes = &episodes
	}

	return anime
}

func (ac *AnimeClient) extractEpisodes(doc *goquery.Document) []types.Episode {
	var episodes []types.Episode

	// Try .episodelist first
	doc.Find(".episodelist .episodeditemlist").Each(func(i int, e *goquery.Selection) {
		epID := uuid.Must(uuid.NewV4())
		episode := types.Episode{ID: epID}

		// Episode number
		epNum := e.Find(".episodedit .epl-num").Text()
		episode.EpisodeNumber = ac.stringPtr(strings.TrimSpace(epNum))

		// Title
		title := e.Find(".episodedit .epl-title").Text()
		episode.Title = ac.stringPtr(strings.TrimSpace(title))

		// Release date
		date := e.Find(".episodedit .epl-date").Text()
		episode.ReleaseDate = ac.stringPtr(strings.TrimSpace(date))

		// Link
		link := e.Find(".episodedit a").First()
		if href, ok := link.Attr("href"); ok {
			episode.PageURL = ac.stringPtr(href)
		}

		episodes = append(episodes, episode)
	})

	// Also try alternative structure
	if len(episodes) == 0 {
		doc.Find(".venz ul li").Each(func(i int, li *goquery.Selection) {
			epID := uuid.Must(uuid.NewV4())
			episode := types.Episode{ID: epID}

			link := li.Find("a").First()
			if href, ok := link.Attr("href"); ok {
				episode.PageURL = ac.stringPtr(href)
			}

			title := link.Text()
			episode.Title = ac.stringPtr(strings.TrimSpace(title))

			re := regexp.MustCompile(`Episode\s*(\d+)`)
			matches := re.FindStringSubmatch(title)
			if len(matches) > 1 {
				episode.EpisodeNumber = ac.stringPtr(matches[1])
			} else {
				episode.EpisodeNumber = ac.stringPtr(strconv.Itoa(i + 1))
			}

			episodes = append(episodes, episode)
		})
	}

	return episodes
}

// extractArchiveAnimes extracts anime from archive pages
func (ac *AnimeClient) extractArchiveAnimes(doc *goquery.Document) []types.Anime {
	var animes []types.Anime

	ul := doc.Find("ul.chivsrc")
	if ul.Length() == 0 {
		return animes
	}

	ul.Find("li").Each(func(i int, li *goquery.Selection) {
		if li.Find("div.pagination").Length() > 0 {
			return
		}

		anime := ac.parseAnimeListItem(li)
		if anime.Title != nil && *anime.Title != "" {
			animes = append(animes, anime)
		}
	})

	return animes
}

func (ac *AnimeClient) parseAnimeListItem(li *goquery.Selection) types.Anime {
	id := uuid.Must(uuid.NewV4())
	anime := types.Anime{ID: id}

	// Thumbnail
	img := li.Find("img")
	if img.Length() > 0 {
		if srcset, ok := img.Attr("srcset"); ok {
			parts := strings.Split(srcset, ",")
			if len(parts) > 0 {
				thumbnail := strings.TrimSpace(strings.Split(parts[0], " ")[0])
				anime.Thumbnail = &thumbnail
			}
		} else if src, ok := img.Attr("src"); ok {
			anime.Thumbnail = &src
		}
	}

	// Title and URL
	titleElem := li.Find("h2")
	if titleElem.Length() > 0 {
		aTag := titleElem.Find("a[href]").First()
		if aTag.Length() > 0 {
			title := strings.TrimSpace(aTag.Text())
			anime.Title = &title
			if href, ok := aTag.Attr("href"); ok {
				url := ac.makeAbsoluteURL(href)
				anime.OriginalPageURL = &url
			}
		}
	}

	if anime.Title == nil || *anime.Title == "" {
		aTag := li.Find("a[href]").First()
		if aTag.Length() > 0 && aTag.Find("img").Length() == 0 {
			title := strings.TrimSpace(aTag.Text())
			if title != "" {
				anime.Title = &title
				if href, ok := aTag.Attr("href"); ok {
					url := ac.makeAbsoluteURL(href)
					anime.OriginalPageURL = &url
				}
			}
		}
	}

	if anime.Title == nil || *anime.Title == "" {
		return anime
	}

	// Release date and genres
	dateDiv := li.Find("div.set")
	if dateDiv.Length() > 0 {
		dateText := strings.TrimSpace(dateDiv.Text())

		if regexp.MustCompile(`(?i)^Genres`).MatchString(dateText) {
			genreText := regexp.MustCompile(`(?i)^Genres\s*:\s*`).ReplaceAllString(dateText, "")
			var genres []types.AnimeGenre
			for _, g := range strings.Split(genreText, ",") {
				g = strings.TrimSpace(g)
				if g != "" {
					genres = append(genres, types.AnimeGenre{Name: &g})
				}
			}
			if len(genres) > 0 {
				anime.Genre = &genres
			}
		} else {
			anime.ReleaseDate = &dateText
		}
	}

	var genres []types.AnimeGenre
	li.Find("div.genrenya a[href]").Each(func(i int, a *goquery.Selection) {
		name := strings.TrimSpace(a.Text())
		href, _ := a.Attr("href")
		if name != "" {
			genre := types.AnimeGenre{
				Name: &name,
			}
			if href != "" {
				genre.URL = ac.stringPtr(ac.makeAbsoluteURL(href))
			}
			genres = append(genres, genre)
		}
	})
	if len(genres) > 0 {
		anime.Genre = &genres
	}

	// Total episodes
	epDiv := li.Find("div.epz")
	if epDiv.Length() > 0 {
		epText := strings.TrimSpace(epDiv.Text())
		anime.TotalEpisodes = &epText
	} else {
		epSpan := li.Find("span.ep")
		if epSpan.Length() > 0 {
			epText := strings.TrimSpace(epSpan.Text())
			anime.TotalEpisodes = &epText
		}
	}

	// Rating
	ratingDiv := li.Find("div.rating")
	if ratingDiv.Length() > 0 {
		ratingText := strings.TrimSpace(ratingDiv.Text())
		anime.Rating = &ratingText
	}

	return anime
}

// extractOngoingAnimes extracts ongoing anime
func (ac *AnimeClient) extractOngoingAnimes(doc *goquery.Document) []types.Anime {
	var animes []types.Anime

	venutama := doc.Find("div.venutama")
	if venutama.Length() == 0 {
		return animes
	}

	rseries := venutama.Find("div.rseries")
	if rseries.Length() == 0 {
		return animes
	}

	rapi := rseries.Find("div.rapi")
	if rapi.Length() == 0 {
		return animes
	}

	venz := rapi.Find("div.venz")
	if venz.Length() == 0 {
		return animes
	}

	ul := venz.Find("ul")
	if ul.Length() == 0 {
		return animes
	}

	ul.Find("li").Each(func(i int, li *goquery.Selection) {
		anime := ac.parseOngoingAnimeItem(li)
		if anime.Title != nil && *anime.Title != "" {
			animes = append(animes, anime)
		}
	})

	return animes
}

func (ac *AnimeClient) parseOngoingAnimeItem(li *goquery.Selection) types.Anime {
	id := uuid.Must(uuid.NewV4())
	anime := types.Anime{ID: id}

	detpost := li.Find("div.detpost")
	if detpost.Length() == 0 {
		return anime
	}

	// Episode
	epz := detpost.Find("div.epz")
	if epz.Length() > 0 {
		epText := strings.TrimSpace(epz.Text())
		re := regexp.MustCompile(`(?i)Episode\s*(\d+(?:\.\d+)?)`)
		matches := re.FindStringSubmatch(epText)
		if len(matches) > 1 {
			anime.TotalEpisodes = &matches[1]
		} else {
			anime.TotalEpisodes = &epText
		}
	}

	// Released day
	epztipe := detpost.Find("div.epztipe")
	if epztipe.Length() > 0 {
		epztipe.Find("i").Remove()
		releasedDay := strings.TrimSpace(epztipe.Text())
		anime.ReleasedDay = &releasedDay
	}

	// Release date
	newnime := detpost.Find("div.newnime")
	if newnime.Length() > 0 {
		releaseDate := strings.TrimSpace(newnime.Text())
		anime.ReleaseDate = &releaseDate
	}

	// Thumbnail and title
	thumbDiv := detpost.Find("div.thumb")
	if thumbDiv.Length() > 0 {
		aTag := thumbDiv.Find("a[href]").First()
		if aTag.Length() > 0 {
			if href, ok := aTag.Attr("href"); ok {
				url := ac.makeAbsoluteURL(href)
				anime.OriginalPageURL = &url
			}
		}

		thumbz := thumbDiv.Find("div.thumbz")
		if thumbz.Length() > 0 {
			img := thumbz.Find("img")
			if img.Length() > 0 {
				if srcset, ok := img.Attr("srcset"); ok {
					parts := strings.Split(srcset, ",")
					if len(parts) > 0 {
						thumbnail := strings.TrimSpace(strings.Split(parts[0], " ")[0])
						anime.Thumbnail = &thumbnail
					}
				} else if src, ok := img.Attr("src"); ok {
					anime.Thumbnail = &src
				}
			}

			title := thumbz.Find("h2.jdlflm").Text()
			if title == "" {
				title = thumbz.Find("h2").Text()
			}
			anime.Title = ac.stringPtr(strings.TrimSpace(title))
		}
	}

	if anime.Title != nil && *anime.Title != "" {
		status := "Ongoing"
		anime.Status = &status
	}

	return anime
}

// extractCompleteAnimes extracts complete anime
func (ac *AnimeClient) extractCompleteAnimes(doc *goquery.Document) []types.Anime {
	return ac.extractOngoingAnimes(doc)
}

func (ac *AnimeClient) parsePaginationArchive(doc *goquery.Document, currentPageFallback int) types.PaginationAnime {
	pagination := types.PaginationAnime{
		CurrentPage: currentPageFallback,
		TotalPages:  currentPageFallback,
		PerPage:     20,
	}

	paginationDiv := doc.Find("div.pagination")
	if paginationDiv.Length() == 0 {
		return pagination
	}

	naviright := paginationDiv.Find("span.naviright")
	if naviright.Length() > 0 {
		pageText := strings.TrimSpace(naviright.Text())
		re := regexp.MustCompile(`(?i)Pages?\s*(\d+)\s*of\s*(\d+)`)
		matches := re.FindStringSubmatch(pageText)
		if len(matches) > 2 {
			current, _ := strconv.Atoi(matches[1])
			total, _ := strconv.Atoi(matches[2])
			pagination.CurrentPage = current
			pagination.TotalPages = total
		}
	}

	navileft := paginationDiv.Find("span.navileft")
	if navileft.Length() > 0 {
		// Find next link (»)
		var nextLink *goquery.Selection
		navileft.Find("a").Each(func(i int, s *goquery.Selection) {
			text := s.Text()
			if strings.Contains(text, "»") && nextLink == nil {
				nextLink = s
			}
		})
		if nextLink != nil {
			if href, ok := nextLink.Attr("href"); ok && href != "" {
				pagination.HasNext = true
				nextURL := ac.makeAbsoluteURL(href)
				pagination.NextPageURL = &nextURL
			}
		}

		// Find prev link («)
		var prevLink *goquery.Selection
		navileft.Find("a").Each(func(i int, s *goquery.Selection) {
			text := s.Text()
			if strings.Contains(text, "«") && prevLink == nil {
				prevLink = s
			}
		})
		if prevLink != nil {
			if href, ok := prevLink.Attr("href"); ok && href != "" {
				pagination.HasPrevious = true
				prevURL := ac.makeAbsoluteURL(href)
				pagination.PreviousPageURL = &prevURL
			}
		}

		navileft.Find("a").Each(func(i int, a *goquery.Selection) {
			text := strings.TrimSpace(a.Text())
			if num, err := strconv.Atoi(text); err == nil {
				pagination.PageNumbers = append(pagination.PageNumbers, num)
			}
		})
	}

	if !pagination.HasNext {
		nextLink := doc.Find("link[rel=\"next\"]")
		if href, ok := nextLink.Attr("href"); ok && href != "" {
			pagination.HasNext = true
			nextURL := ac.makeAbsoluteURL(href)
			pagination.NextPageURL = &nextURL
		}
	}

	if !pagination.HasPrevious {
		prevLink := doc.Find("link[rel=\"prev\"]")
		if href, ok := prevLink.Attr("href"); ok && href != "" {
			pagination.HasPrevious = true
			prevURL := ac.makeAbsoluteURL(href)
			pagination.PreviousPageURL = &prevURL
		}
	}

	return pagination
}

func (ac *AnimeClient) parsePaginationOngoing(doc *goquery.Document, currentPage int) types.PaginationAnime {
	pagination := types.PaginationAnime{
		CurrentPage: currentPage,
		TotalPages:  currentPage,
		PerPage:     20,
	}

	venutama := doc.Find("div.venutama")
	if venutama.Length() == 0 {
		return pagination
	}

	paginationDiv := venutama.Find("div.pagination")
	if paginationDiv.Length() == 0 {
		return pagination
	}

	pagenavix := paginationDiv.Find("div.pagenavix")
	if pagenavix.Length() == 0 {
		return pagination
	}

	currentSpan := pagenavix.Find("span.current[aria-current=\"page\"]")
	if currentSpan.Length() > 0 {
		text := strings.TrimSpace(currentSpan.Text())
		if num, err := strconv.Atoi(text); err == nil {
			pagination.CurrentPage = num
		}
	}

	pageLinks := pagenavix.Find("a.page-numbers")
	maxPage := pagination.CurrentPage
	pageNumbers := []int{}

	pageLinks.Each(func(i int, a *goquery.Selection) {
		text := strings.TrimSpace(a.Text())
		if num, err := strconv.Atoi(text); err == nil {
			if num > maxPage {
				maxPage = num
			}
			pageNumbers = append(pageNumbers, num)
		}
	})

	pagination.TotalPages = maxPage
	pagination.PageNumbers = pageNumbers

	nextLink := pagenavix.Find("a.next[href]").First()
	if href, ok := nextLink.Attr("href"); ok && href != "" {
		pagination.HasNext = true
		nextURL := ac.makeAbsoluteURL(href)
		pagination.NextPageURL = &nextURL
	}

	prevLink := pagenavix.Find("a.prev[href]").First()
	if href, ok := prevLink.Attr("href"); ok && href != "" {
		pagination.HasPrevious = true
		prevURL := ac.makeAbsoluteURL(href)
		pagination.PreviousPageURL = &prevURL
	}

	return pagination
}

// Helper functions

func (ac *AnimeClient) makeAbsoluteURL(url string) string {
	if url == "" {
		return ""
	}

	url = strings.TrimSpace(url)

	if strings.HasPrefix(url, "http") {
		return url
	}

	baseURL := ac.BaseURL
	if !strings.HasPrefix(url, "/") {
		url = "/" + url
	}

	return baseURL + url
}

func (ac *AnimeClient) stringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// FetchHTML fetches HTML content using chromedp
func (ac *AnimeClient) FetchHTML(url string) (string, error) {
	var htmlContent string

	actions := []chromedp.Action{
		chromedp.Navigate(url),
		chromedp.Sleep(3 * time.Second),
		chromedp.OuterHTML("html", &htmlContent),
	}

	err := chromedp.Run(ac.ChromeClient.ctx, actions...)
	if err != nil {
		return "", err
	}

	return htmlContent, nil
}
