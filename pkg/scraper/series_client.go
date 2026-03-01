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

type SeriesClient struct {
	ChromeClient *ChromeClient
	HTTPClient   *http.Client
}

func NewSeriesClient() *SeriesClient {
	return &SeriesClient{
		ChromeClient: NewChromeClient(),
		HTTPClient:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (sc *SeriesClient) Close() {
	if sc.ChromeClient != nil {
		sc.ChromeClient.Close()
	}
}

func (sc *SeriesClient) GetHome() ([]types.HomeScrapperResponse, error) {
	var htmlContent string

	fmt.Printf("Scraper series home url: %s\n", SeriesBaseURL)

	actions := []chromedp.Action{
		chromedp.Navigate(SeriesBaseURL),
		chromedp.Sleep(3 * time.Second),
		chromedp.OuterHTML("html", &htmlContent),
	}

	err := chromedp.Run(sc.ChromeClient.ctx, actions...)
	if err != nil {
		logger.Logger.Error("Error loading series home page", zap.Error(err))
		return nil, err
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		logger.Logger.Error("Error parsing series home HTML", zap.Error(err))
		return nil, err
	}

	return sc.scrapeHome(doc), nil
}

func (sc *SeriesClient) scrapeHome(doc *goquery.Document) []types.HomeScrapperResponse {
	var results []types.HomeScrapperResponse

	categoryMappings := []struct {
		ariaLabel string
		key       string
	}{
		{"Film Unggulan", "Featured Series"},
		{"TERBARU", "New Series"},
		{"LK21 TERBARU", "Latest Featured Series"},
		{"SERIES UPDATE", "Series Updates"},
		{"You May Also Like", "You May Also Like"},
		{"TOP BULAN INI", "Top Of The Month"},
		{"Rekomendasi Untukmu", "Recommendation For You"},
		{"Nonton Bareng Keluarga", "Watch With Family"},
		{"Action Terbaru", "Latest Action Series"},
		{"Maraton Drakor", "Korean Drama Marathon"},
		{"Horror Terbaru", "Latest Horror Series"},
		{"Romance Terbaru", "Latest Romance Series"},
		{"Comedy Terbaru", "Latest Comedy Series"},
		{"Korea Terbaru", "Latest Korean Series"},
		{"China Terbaru", "Latest China Series"},
		{"Thailand Terbaru", "Latest Thailand Series"},
	}

	ariaToKey := make(map[string]string)
	for _, m := range categoryMappings {
		ariaToKey[m.ariaLabel] = m.key
	}

	doc.Find(".slider-wrapper").Each(func(i int, s *goquery.Selection) {
		ariaLabel, _ := s.Attr("aria-label")
		key := ariaToKey[ariaLabel]
		if key == "" {
			return
		}

		movies := sc.scrapeSliderSeries(s)
		if len(movies) == 0 {
			sliders := s.Find("ul.sliders")
			if sliders.Length() > 0 {
				movies = sc.scrapeSliderSeries(sliders)
			}
		}

		if len(movies) > 0 {
			var viewAllURL *string
			url := sc.getViewAllURL(s)

			if url != nil && *url != "" {
				absoluteURL := sc.makeAbsoluteURL(*url)
				viewAllURL = &absoluteURL
			}

			results = append(results, types.HomeScrapperResponse{
				Key:        key,
				Value:      movies,
				ViewAllUrl: viewAllURL,
			})
		}
	})

	doc.Find("ul.sliders").Each(func(i int, s *goquery.Selection) {
		parent := s.Parent()
		ariaLabel, _ := parent.Attr("aria-label")
		if ariaLabel == "" {
			ariaLabel = parent.Find(".slider-wrapper").First().AttrOr("aria-label", "")
		}

		key := ariaToKey[ariaLabel]
		if key == "" {
			return
		}

		// Check if already added
		alreadyAdded := false
		for _, r := range results {
			if r.Key == key {
				alreadyAdded = true
				break
			}
		}
		if alreadyAdded {
			return
		}

		movies := sc.scrapeSliderSeries(s)
		if len(movies) > 0 {
			results = append(results, types.HomeScrapperResponse{
				Key:   key,
				Value: movies,
			})
		}
	})

	allLatest := sc.scrapeAllLatestSeries(doc)
	if len(allLatest) > 0 {
		latestURL := sc.makeAbsoluteURL("/latest")
		results = append(results, types.HomeScrapperResponse{
			Key:        "All Latest Series",
			Value:      allLatest,
			ViewAllUrl: &latestURL,
		})
	}

	return results
}

func (sc *SeriesClient) scrapeSliderSeries(s *goquery.Selection) []types.Movie {
	var movies []types.Movie

	s.Find(".slider-item, .slider").Each(func(i int, item *goquery.Selection) {
		movie := sc.parseSeriesArticle(item)
		if movie.Title != "" {
			movies = append(movies, *movie)
		}
	})

	return movies
}

func (sc *SeriesClient) scrapeGalleryMovies(s *goquery.Selection) []types.Movie {
	var movies []types.Movie

	s.Find("article").Each(func(i int, item *goquery.Selection) {
		movie := sc.parseSeriesArticle(item)
		if movie.Title != "" {
			movies = append(movies, *movie)
		}
	})

	return movies
}

func (sc *SeriesClient) parseSeriesArticle(article *goquery.Selection) *types.Movie {
	movie := &types.Movie{}

	// ID
	id := uuid.Must(uuid.NewV4())
	movie.ID = id

	// Title
	title := article.Find(".poster-title").Text()
	if title == "" {
		title = article.Find(".video-title").Text()
	}
	movie.Title = strings.TrimSpace(title)

	// URL
	var originalPageURL string
	article.Find("a[itemprop='url']").Each(func(i int, a *goquery.Selection) {
		if href, ok := a.Attr("href"); ok {
			originalPageURL = sc.makeAbsoluteURL(href)
		}
	})
	if originalPageURL == "" {
		article.Find("a[href]").Each(func(i int, a *goquery.Selection) {
			if href, ok := a.Attr("href"); ok && strings.Contains(href, "/series/") {
				originalPageURL = sc.makeAbsoluteURL(href)
			}
		})
	}
	movie.OriginalPageUrl = &originalPageURL

	// Thumbnail
	if img, ok := article.Find("img[itemprop='image']").Attr("src"); ok {
		movie.Thumbnail = sc.stringPtr(sc.makeAbsoluteURL(img))
	}

	// Year
	yearStr := article.Find(".year").Text()
	if yearStr != "" {
		if year, err := strconv.Atoi(strings.TrimSpace(yearStr)); err == nil {
			movie.Year = sc.int32Ptr(int32(year))
		}
	}

	// Rating
	ratingStr := article.Find("[itemprop='ratingValue']").Text()
	if ratingStr != "" {
		if rating, err := strconv.ParseFloat(strings.TrimSpace(ratingStr), 64); err == nil {
			movie.Rating = sc.float64Ptr(rating)
		}
	}

	// Quality
	quality := article.Find(".label").Text()
	movie.LabelQuality = sc.stringPtr(strings.TrimSpace(quality))

	// Genre
	genre := article.Find("[itemprop='genre']").Text()
	movie.Genre = sc.stringPtr(strings.TrimSpace(genre))

	return movie
}

func (sc *SeriesClient) GetSeriesList(pathname string, page int) (*types.MovieListResponse, error) {
	var url string

	cleanPathname := sc.makeCleanPathname(pathname)

	if page > 1 {
		url = fmt.Sprintf("%s%s/page/%d/", SeriesBaseURL, cleanPathname, page)
	} else {
		url = fmt.Sprintf("%s%s", SeriesBaseURL, cleanPathname)
	}

	fmt.Printf("Scraping series list from URL: %s and page: %d\n", url, page)

	var htmlContent string

	actions := []chromedp.Action{
		chromedp.Navigate(url),
		chromedp.Sleep(3 * time.Second),
		chromedp.OuterHTML("html", &htmlContent),
	}

	err := chromedp.Run(sc.ChromeClient.ctx, actions...)
	if err != nil {
		logger.Logger.Error("Error loading series list page", zap.Error(err))
		return nil, err
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		logger.Logger.Error("Error parsing series list HTML", zap.Error(err))
		return nil, err
	}

	return sc.scrapeSeriesList(doc), nil
}

func (sc *SeriesClient) scrapeSeriesList(doc *goquery.Document) *types.MovieListResponse {
	response := &types.MovieListResponse{
		Pagination: types.Pagination{
			CurrentPage: 1,
			TotalPage:   1,
			HasNext:     false,
			HasPrev:     false,
		},
	}

	gallery := doc.Find(".gallery-grid")
	if gallery.Length() == 0 {
		return response
	}

	gallery.Find("article").Each(func(i int, item *goquery.Selection) {
		movie := sc.parseSeriesArticle(item)
		if movie.Title != "" {
			response.Movies = append(response.Movies, *movie)
		}
	})

	pagination := sc.parsePagination(doc)
	response.Pagination = pagination
	response.Pagination.TotalItems = int64(response.Pagination.TotalPage) * int64(len(response.Movies))

	return response
}

func (sc *SeriesClient) parsePagination(doc *goquery.Document) types.Pagination {
	pagination := types.Pagination{
		CurrentPage: 1,
		TotalPage:   1,
		HasNext:     false,
		HasPrev:     false,
	}

	var paginationEl *goquery.Selection

	wrapper := doc.Find("nav.pagination-wrapper")
	if wrapper.Length() > 0 {
		paginationEl = wrapper.Find("ul.pagination")
	}

	if paginationEl == nil || paginationEl.Length() == 0 {
		paginationEl = doc.Find("ul.pagination")
	}

	if paginationEl == nil || paginationEl.Length() == 0 {
		paginationEl = doc.Find(".pagination")
	}

	if paginationEl == nil || paginationEl.Length() == 0 {
		return pagination
	}

	pageRegex := regexp.MustCompile(`/page/(\d+)/?`)
	var maxPage int = 1
	var currentPage int = 1
	pageToURL := make(map[int]string)

	paginationEl.Find("li a").Each(func(i int, a *goquery.Selection) {
		href, exists := a.Attr("href")
		if !exists {
			return
		}

		text := strings.TrimSpace(a.Text())
		absoluteURL := sc.makeAbsoluteURL(href)

		// Extract page number from URL
		matches := pageRegex.FindStringSubmatch(href)
		if len(matches) > 1 {
			if page, err := strconv.Atoi(matches[1]); err == nil {
				pageToURL[page] = absoluteURL

				// Update maxPage - including the "»" link because it contains the largest page number
				if page > maxPage {
					maxPage = page
				}
			}
		}

		// Check if this is current page
		if a.Parent().HasClass("active") {
			// Try to get page from text first
			if page, err := strconv.Atoi(text); err == nil {
				currentPage = page
			} else if len(matches) > 1 {
				if page, err := strconv.Atoi(matches[1]); err == nil {
					currentPage = page
				}
			}
		}
	})

	// Set values
	pagination.CurrentPage = int32(currentPage)
	pagination.TotalPage = int32(maxPage)
	pagination.HasNext = currentPage < maxPage
	pagination.HasPrev = currentPage > 1

	// Set next URL (can be from page+1 or from link "»")
	if nextURL, ok := pageToURL[currentPage+1]; ok {
		pagination.NextPageUrl = &nextURL
	} else if pagination.HasNext {
		// Fallback: search for links with text "»"
		paginationEl.Find("li a").Each(func(i int, a *goquery.Selection) {
			if strings.TrimSpace(a.Text()) == "»" {
				if href, ok := a.Attr("href"); ok {
					url := sc.makeAbsoluteURL(href)
					pagination.NextPageUrl = &url
				}
			}
		})
	}

	// Set prev URL
	if prevURL, ok := pageToURL[currentPage-1]; ok {
		pagination.PrevPageUrl = &prevURL
	}

	return pagination
}

func (sc *SeriesClient) GetSeriesDetail(url string) (*types.SeriesDetail, error) {
	var htmlContent string

	actions := []chromedp.Action{
		chromedp.Navigate(url),
		chromedp.Sleep(3 * time.Second),
		chromedp.OuterHTML("html", &htmlContent),
	}

	err := chromedp.Run(sc.ChromeClient.ctx, actions...)
	if err != nil {
		logger.Logger.Error("Error loading series detail page", zap.Error(err))
		return nil, err
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		logger.Logger.Error("Error parsing series detail HTML", zap.Error(err))
		return nil, err
	}

	detail := sc.scrapeSeriesDetail(doc, url)
	detail.SourceUrl = sc.stringPtr(url)

	return detail, nil
}

func (sc *SeriesClient) scrapeSeriesDetail(doc *goquery.Document, originalURL string) *types.SeriesDetail {
	detail := &types.SeriesDetail{
		Type: "series",
	}

	// Title
	titleDiv := doc.Find(".movie-info")
	if titleDiv.Length() > 0 {
		h1 := titleDiv.Find("h1")
		if h1.Length() > 0 {
			rawTitle := h1.Text()
			rawTitle = strings.ReplaceAll(rawTitle, "Nonton ", "")
			rawTitle = strings.ReplaceAll(rawTitle, " Sub Indo di Lk21", "")
			rawTitle = strings.ReplaceAll(rawTitle, " Sub Indo", "")
			detail.Title = strings.TrimSpace(rawTitle)
		}
	}

	// Synopsis
	synopsisDiv := doc.Find(".synopsis")
	if synopsisDiv.Length() > 0 {
		if synopsis, ok := synopsisDiv.Attr("data-full"); ok && synopsis != "" {
			detail.Synopsis = sc.stringPtr(synopsis)
		} else {
			detail.Synopsis = sc.stringPtr(strings.TrimSpace(synopsisDiv.Text()))
		}
	}

	// Meta info
	infoTag := doc.Find(".info-tag")
	if infoTag.Length() > 0 {
		spans := infoTag.Find("span")
		spans.Each(func(i int, span *goquery.Selection) {
			text := strings.TrimSpace(span.Text())
			switch i {
			case 0:
				if rating, err := strconv.ParseFloat(text, 64); err == nil {
					detail.Rating = sc.float64Ptr(rating)
				}
			case 1:
				detail.LabelQuality = sc.stringPtr(text)
			case 3:
				if duration := sc.parseDuration(text); duration > 0 {
					detail.Duration = sc.int64Ptr(int64(duration))
				}
			}
		})
	}

	// Genres & Countries
	tagList := doc.Find(".tag-list")
	if tagList.Length() > 0 {
		var genres []types.Genre
		var countries []types.CountryMovie

		tagList.Find("a").Each(func(i int, a *goquery.Selection) {
			href, _ := a.Attr("href")
			text := strings.TrimSpace(a.Text())

			if strings.Contains(href, "/genre/") {
				genres = append(genres, types.Genre{
					Name:    sc.stringPtr(text),
					PageUrl: sc.stringPtr(sc.makeAbsoluteURL(href)),
				})
			} else if strings.Contains(href, "/country/") {
				countries = append(countries, types.CountryMovie{
					Name:    sc.stringPtr(text),
					PageUrl: sc.stringPtr(sc.makeAbsoluteURL(href)),
				})
			}
		})

		if len(genres) > 0 {
			detail.Genres = &genres
		}
		if len(countries) > 0 {
			detail.Countries = &countries
		}
	}

	// Detailed info
	detailDiv := doc.Find(".detail")
	if detailDiv.Length() > 0 {
		if img, ok := detailDiv.Find("img[itemprop='image']").Attr("src"); ok {
			detail.Thumbnail = sc.stringPtr(sc.makeAbsoluteURL(img))
		}

		detailDiv.Find("p").Each(func(i int, p *goquery.Selection) {
			text := strings.TrimSpace(p.Text())

			if strings.Contains(text, "Sutradara:") {
				var directors []types.MoviePerson
				p.Find("a").Each(func(i int, a *goquery.Selection) {
					href, _ := a.Attr("href")
					directors = append(directors, types.MoviePerson{
						Name:    sc.stringPtr(strings.TrimSpace(a.Text())),
						PageUrl: sc.stringPtr(sc.makeAbsoluteURL(href)),
					})
				})
				if len(directors) > 0 {
					detail.Director = &directors
				}
			} else if strings.Contains(text, "Bintang Film:") {
				var stars []types.MoviePerson
				p.Find("a").Each(func(i int, a *goquery.Selection) {
					href, _ := a.Attr("href")
					stars = append(stars, types.MoviePerson{
						Name:    sc.stringPtr(strings.TrimSpace(a.Text())),
						PageUrl: sc.stringPtr(sc.makeAbsoluteURL(href)),
					})
				})
				if len(stars) > 0 {
					detail.MovieStar = &stars
				}
			} else if strings.Contains(text, "Votes:") {
				re := regexp.MustCompile(`Votes:\s*([\d,]+)`)
				matches := re.FindStringSubmatch(text)
				if len(matches) > 1 {
					votesStr := strings.ReplaceAll(matches[1], ",", "")
					if votes, err := strconv.ParseInt(votesStr, 10, 64); err == nil {
						detail.Votes = sc.int64Ptr(votes)
					}
				}
			} else if strings.Contains(text, "Status:") {
				status := strings.ReplaceAll(text, "Status:", "")
				detail.Status = sc.stringPtr(strings.TrimSpace(status))
			}
		})
	}

	// Parse season list
	seasonList := sc.parseSeasonList(doc)
	if len(seasonList) > 0 {
		detail.SeasonList = &seasonList
	}

	// Similar series
	var similarMovies []types.Movie
	doc.Find(".similar-movies article, .related-movies article").Each(func(i int, item *goquery.Selection) {
		movie := sc.parseSeriesArticle(item)
		if movie.Title != "" {
			similarMovies = append(similarMovies, *movie)
		}
	})
	if len(similarMovies) > 0 {
		detail.SimilarMovies = &similarMovies
	}

	return detail
}

func (sc *SeriesClient) parseSeasonList(doc *goquery.Document) []types.SeasonList {
	var seasons []types.SeasonList

	// Try #season-data first
	seasonData := doc.Find("#season-data")
	if seasonData.Length() > 0 {
		seasonData.Find(".season-item, .season").Each(func(i int, s *goquery.Selection) {
			season := types.SeasonList{}

			// Current season number
			seasonNum := int32(i + 1)
			season.CurrentSeason = &seasonNum

			// Total seasons - try to find total
			totalSeasons := int32(1)
			allSeasons := doc.Find(".season-item, .season")
			total := allSeasons.Length()
			if total > 0 {
				totalSeasons = int32(total)
			}
			season.TotalSeason = &totalSeasons

			// Parse episodes for this season
			var episodes []types.EpisodeList
			s.Find("a[href*='episode']").Each(func(j int, a *goquery.Selection) {
				href, _ := a.Attr("href")
				text := strings.TrimSpace(a.Text())

				epNum := int32(j + 1)
				ep := types.EpisodeList{
					EpisodeNumber: &epNum,
					EpisodeUrl:    sc.stringPtr(sc.makeAbsoluteURL(href)),
				}
				_ = text
				episodes = append(episodes, ep)
			})

			if len(episodes) > 0 {
				season.EpisodeList = &episodes
			}

			seasons = append(seasons, season)
		})
	}

	// Try season-select dropdown
	seasonSelect := doc.Find("select.season-select")
	if seasonSelect.Length() > 0 && len(seasons) == 0 {
		options := seasonSelect.Find("option")
		total := options.Length()

		options.Each(func(i int, opt *goquery.Selection) {
			season := types.SeasonList{}
			seasonNum := int32(i + 1)
			season.CurrentSeason = &seasonNum
			totalSeasons := int32(total)
			season.TotalSeason = &totalSeasons

			seasons = append(seasons, season)
		})
	}

	// Try episodes section with season-X-episode-Y links
	if len(seasons) == 0 {
		episodeLinks := doc.Find("a[href*='season-'][href*='-episode-']")
		if episodeLinks.Length() > 0 {
			seasonMap := make(map[int][]types.EpisodeList)

			episodeLinks.Each(func(i int, a *goquery.Selection) {
				href, _ := a.Attr("href")

				re := regexp.MustCompile(`season-(\d+)-episode-(\d+)`)
				matches := re.FindStringSubmatch(href)
				if len(matches) > 2 {
					seasonNum, _ := strconv.Atoi(matches[1])
					epNum, _ := strconv.Atoi(matches[2])

					ep := types.EpisodeList{
						EpisodeNumber: sc.int32Ptr(int32(epNum)),
						EpisodeUrl:    sc.stringPtr(sc.makeAbsoluteURL(href)),
					}
					seasonMap[seasonNum] = append(seasonMap[seasonNum], ep)
				}
			})

			// Convert map to slice
			maxSeason := 0
			for k := range seasonMap {
				if k > maxSeason {
					maxSeason = k
				}
			}

			for i := 1; i <= maxSeason; i++ {
				season := types.SeasonList{}
				seasonNum := int32(i)
				season.CurrentSeason = &seasonNum
				totalSeasons := int32(maxSeason)
				season.TotalSeason = &totalSeasons
				epList := seasonMap[i]
				season.EpisodeList = &epList

				seasons = append(seasons, season)
			}
		}
	}

	return seasons
}

func (sc *SeriesClient) GetEpisode(url string) (*types.SeriesEpisode, error) {
	var htmlContent string

	actions := []chromedp.Action{
		chromedp.Navigate(url),
		chromedp.Sleep(3 * time.Second),
		chromedp.OuterHTML("html", &htmlContent),
	}

	err := chromedp.Run(sc.ChromeClient.ctx, actions...)
	if err != nil {
		logger.Logger.Error("Error loading episode page", zap.Error(err))
		return nil, err
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		logger.Logger.Error("Error parsing episode HTML", zap.Error(err))
		return nil, err
	}

	return sc.scrapeEpisode(doc), nil
}

func (sc *SeriesClient) scrapeEpisode(doc *goquery.Document) *types.SeriesEpisode {
	episode := &types.SeriesEpisode{}

	// Episode URL
	episode.EpisodeUrl = sc.stringPtr(doc.Url.String())

	// Extract episode number from title or URL
	title := doc.Find("h1").Text()
	re := regexp.MustCompile(`Episode\s*(\d+)`)
	matches := re.FindStringSubmatch(title)
	if len(matches) > 1 {
		if epNum, err := strconv.Atoi(matches[1]); err == nil {
			episode.EpisodeNumber = sc.int32Ptr(int32(epNum))
		}
	}

	// Player URLs
	var playerURLs []types.PlayerUrl
	doc.Find(".player-selector a, .embed-options a").Each(func(i int, a *goquery.Selection) {
		href, _ := a.Attr("href")
		text := strings.TrimSpace(a.Text())
		playerURLs = append(playerURLs, types.PlayerUrl{
			URL:  sc.stringPtr(href),
			Type: sc.stringPtr(text),
		})
	})
	if len(playerURLs) > 0 {
		episode.PlayerUrl = &playerURLs
	}

	// Download URL
	downloadLink := doc.Find(".download-link a").First()
	if href, ok := downloadLink.Attr("href"); ok {
		episode.DownloadUrl = sc.stringPtr(href)
	}

	return episode
}

func (sc *SeriesClient) Search(query string, page int) (*types.MovieListResponse, error) {
	var url string
	if page > 1 {
		url = fmt.Sprintf("%s/search?s=%s&page=%d", SeriesBaseURL, query, page)
	} else {
		url = fmt.Sprintf("%s/search?s=%s", SeriesBaseURL, query)
	}

	fmt.Printf("Scraping search results for query: %s and page: %d\n", query, page)
	fmt.Printf("Search URL: %s\n", url)

	var htmlContent string

	actions := []chromedp.Action{
		chromedp.Navigate(url),
		chromedp.Sleep(3 * time.Second),
		chromedp.OuterHTML("html", &htmlContent),
	}

	err := chromedp.Run(sc.ChromeClient.ctx, actions...)
	if err != nil {
		logger.Logger.Error("Error loading search page", zap.Error(err))
		return nil, err
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		logger.Logger.Error("Error parsing search HTML", zap.Error(err))
		return nil, err
	}

	return sc.scrapeSeriesList(doc), nil
}

func (sc *SeriesClient) GetLatest(page int) (*types.MovieListResponse, error) {
	return sc.GetSeriesList(SeriesBaseURL+"/latest-series/", page)
}

// Helper functions
func (sc *SeriesClient) scrapeAllLatestSeries(doc *goquery.Document) []types.Movie {
	var movies []types.Movie

	headers := []string{"Daftar Lengkap Series Terbaru", "Episode Terbaru", "Latest Series"}

	for _, headerText := range headers {
		doc.Find("h2").Each(func(i int, h *goquery.Selection) {
			if strings.TrimSpace(h.Text()) == headerText {
				headerDiv := h.Parent()
				if headerDiv.Length() > 0 {
					gallery := headerDiv.NextFiltered(".gallery-grid")
					if gallery.Length() > 0 {
						movies = sc.scrapeGalleryMovies(gallery)
					}
				}
			}
		})
		if len(movies) > 0 {
			break
		}
	}

	return movies
}

func (sc *SeriesClient) parseDuration(dur string) int {
	dur = strings.TrimSpace(dur)
	parts := strings.Split(dur, ":")

	var totalSeconds int

	if len(parts) == 2 {
		if minutes, err := strconv.Atoi(parts[0]); err == nil {
			totalSeconds += minutes * 60
		}
		if seconds, err := strconv.Atoi(parts[1]); err == nil {
			totalSeconds += seconds
		}
	} else if len(parts) == 3 {
		if hours, err := strconv.Atoi(parts[0]); err == nil {
			totalSeconds += hours * 3600
		}
		if minutes, err := strconv.Atoi(parts[1]); err == nil {
			totalSeconds += minutes * 60
		}
		if seconds, err := strconv.Atoi(parts[2]); err == nil {
			totalSeconds += seconds
		}
	}

	return totalSeconds
}

func (sc *SeriesClient) makeAbsoluteURL(url string) string {
	if url == "" {
		return ""
	}

	url = strings.TrimSpace(url)

	if strings.HasPrefix(url, "http") {
		return url
	}

	if strings.HasPrefix(url, "//") {
		return "https:" + url
	}

	if !strings.HasPrefix(url, "/") {
		url = "/" + url
	}
	return SeriesBaseURL + url
}

func (sc *SeriesClient) getViewAllURL(s *goquery.Selection) *string {
	var viewAllURL *string

	parent := s.Closest(".widget, .container").First()
	if parent.Length() == 0 {
		return nil
	}

	switch {
	case parent.HasClass("widget"):
		viewAllURL = sc.stringPtr(strings.TrimSpace(parent.Find(".header a.btn").AttrOr("href", "")))

	case parent.HasClass("container"):
		moreFeatured := parent.Find(".more-featured a.btn")
		if moreFeatured.Length() > 0 {
			viewAllURL = sc.stringPtr(strings.TrimSpace(moreFeatured.AttrOr("href", "")))
		}

		if viewAllURL == nil {
			parent.Find("a.btn").Each(func(i int, link *goquery.Selection) {
				if strings.Contains(strings.ToLower(link.Text()), "semua") {
					viewAllURL = sc.stringPtr(strings.TrimSpace(link.AttrOr("href", "")))
				}
			})
		}

	default:

		viewAllURL = sc.stringPtr(strings.TrimSpace(parent.Find("a.btn[href]").AttrOr("href", "")))
	}

	return viewAllURL
}

func (sc *SeriesClient) makeCleanPathname(pathname string) string {
	if strings.HasPrefix(pathname, "/") {
		return strings.Replace(pathname, "/", "", 1)
	}
	return pathname
}

func (sc *SeriesClient) stringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (sc *SeriesClient) int32Ptr(i int32) *int32 {
	return &i
}

func (sc *SeriesClient) int64Ptr(i int64) *int64 {
	return &i
}

func (sc *SeriesClient) float64Ptr(f float64) *float64 {
	return &f
}

// FetchHTML fetches HTML content using chromedp
func (sc *SeriesClient) FetchHTML(url string) (string, error) {
	var htmlContent string

	actions := []chromedp.Action{
		chromedp.Navigate(url),
		chromedp.Sleep(3 * time.Second),
		chromedp.OuterHTML("html", &htmlContent),
	}

	err := chromedp.Run(sc.ChromeClient.ctx, actions...)
	if err != nil {
		return "", err
	}

	return htmlContent, nil
}
