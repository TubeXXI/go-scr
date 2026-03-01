package scraper

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
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

type MovieClient struct {
	ChromeClient *ChromeClient
	HTTPClient   *http.Client
}

func NewMovieClient() *MovieClient {
	return &MovieClient{
		ChromeClient: NewChromeClient(),
		HTTPClient:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (mc *MovieClient) Close() {
	if mc.ChromeClient != nil {
		mc.ChromeClient.Close()
	}
}

// GetHome scrapes the home page and returns categories with Key, Value, and ViewAllUrl
func (mc *MovieClient) GetHome() ([]types.HomeScrapperResponse, error) {
	var htmlContent string

	actions := []chromedp.Action{
		chromedp.Navigate(MovieBaseURL),
		chromedp.Sleep(3 * time.Second),
		chromedp.OuterHTML("html", &htmlContent),
	}

	err := chromedp.Run(mc.ChromeClient.ctx, actions...)
	if err != nil {
		logger.Logger.Error("Error loading home page", zap.Error(err))
		return nil, err
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		logger.Logger.Error("Error parsing home HTML", zap.Error(err))
		return nil, err
	}

	return mc.scrapeHome(doc), nil
}

func (mc *MovieClient) scrapeHome(doc *goquery.Document) []types.HomeScrapperResponse {
	var results []types.HomeScrapperResponse

	// Define category mappings with their aria-label keys
	categoryMappings := []struct {
		ariaLabel string
		key       string
	}{
		{"Film Unggulan", "Featured Movies"},
		{"Film Terbaru", "New Movies"},
		{"SERIES UNGGULAN", "Featured Series"},
		{"SERIES UPDATE", "Series Updates"},
		{"You May Also Like", "You May Also Like"},
		{"TOP BULAN INI", "Top Of The Month"},
		{"Rekomendasi Untukmu", "Recommendation For You"},
		{"Nonton Bareng Keluarga", "Watch With Family"},
		{"Action Terbaru", "Latest Action Movies"},
		{"Maraton Drakor", "Korean Drama Marathon"},
		{"Horror Terbaru", "Latest Horror Movies"},
		{"Romance Terbaru", "Latest Romance Movies"},
		{"Comedy Terbaru", "Latest Comedy Movies"},
		{"Korea Terbaru", "Latest Korean Movies"},
		{"Thailand Terbaru", "Latest Thailand Movies"},
		{"India Terbaru", "Latest Indian Movies"},
	}

	// Create a map for quick lookup
	ariaToKey := make(map[string]string)
	for _, m := range categoryMappings {
		ariaToKey[m.ariaLabel] = m.key
	}

	// Scrape from .slider-wrapper
	doc.Find(".slider-wrapper").Each(func(i int, s *goquery.Selection) {
		ariaLabel, _ := s.Attr("aria-label")
		key := ariaToKey[ariaLabel]
		if key == "" {
			return
		}

		// Get movies
		movies := mc.scrapeSliderMovies(s)
		if len(movies) == 0 {
			sliders := s.Find("ul.sliders")
			if sliders.Length() > 0 {
				movies = mc.scrapeSliderMovies(sliders)
			}
		}

		if len(movies) > 0 {
			var viewAllURL *string
			url := mc.getViewAllURL(s)

			if url != nil && *url != "" {
				absoluteURL := mc.makeAbsoluteURL(*url)
				viewAllURL = &absoluteURL
			}

			results = append(results, types.HomeScrapperResponse{
				Key:        key,
				Value:      movies,
				ViewAllUrl: viewAllURL,
			})
		}
	})

	// Also scrape from ul.sliders directly
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

		movies := mc.scrapeSliderMovies(s)
		if len(movies) > 0 {
			results = append(results, types.HomeScrapperResponse{
				Key:   key,
				Value: movies,
			})
		}
	})

	allLatest := mc.scrapeAllLatestMovies(doc)
	if len(allLatest) > 0 {
		latestURL := mc.makeAbsoluteURL("/latest")
		results = append(results, types.HomeScrapperResponse{
			Key:        "All Latest Movies",
			Value:      allLatest,
			ViewAllUrl: &latestURL,
		})
	}

	return results
}

func (mc *MovieClient) scrapeSliderMovies(s *goquery.Selection) []types.Movie {
	var movies []types.Movie

	// Handle both .slider-item and .slider classes
	s.Find(".slider-item, .slider").Each(func(i int, item *goquery.Selection) {
		movie := mc.parseArticle(item)
		if movie.Title != "" {
			movies = append(movies, *movie)
		}
	})

	return movies
}

func (mc *MovieClient) parseArticle(article *goquery.Selection) *types.Movie {
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
			originalPageURL = mc.makeAbsoluteURL(href)
		}
	})
	if originalPageURL == "" {
		article.Find("a[href]").Each(func(i int, a *goquery.Selection) {
			if href, ok := a.Attr("href"); ok && strings.Contains(href, "/movie/") {
				originalPageURL = mc.makeAbsoluteURL(href)
			}
		})
	}
	movie.OriginalPageUrl = &originalPageURL

	// Thumbnail - try picture source first
	picture := article.Find("picture").First()
	if picture.Length() > 0 {
		// Try webp first
		webp := picture.Find("source[type='image/webp']").First()
		if srcset, ok := webp.Attr("srcset"); ok {
			parts := strings.Split(srcset, ",")
			if len(parts) > 0 {
				thumbnail := strings.TrimSpace(strings.Split(parts[0], " ")[0])
				movie.Thumbnail = mc.stringPtr(thumbnail)
			}
		}
		// Fallback to jpeg
		if movie.Thumbnail == nil {
			jpeg := picture.Find("source[type='image/jpeg']").First()
			if srcset, ok := jpeg.Attr("srcset"); ok {
				parts := strings.Split(srcset, ",")
				if len(parts) > 0 {
					thumbnail := strings.TrimSpace(strings.Split(parts[0], " ")[0])
					movie.Thumbnail = mc.stringPtr(thumbnail)
				}
			}
		}
	}
	// Fallback to img tag
	if movie.Thumbnail == nil {
		if img, ok := article.Find("img[itemprop='image']").Attr("src"); ok {
			movie.Thumbnail = mc.stringPtr(img)
		}
	}

	// Year
	yearStr := article.Find(".year").Text()
	if yearStr == "" {
		yearStr = article.Find(".video-year").Text()
	}
	if yearStr != "" {
		if year, err := strconv.Atoi(strings.TrimSpace(yearStr)); err == nil {
			movie.Year = mc.int32Ptr(int32(year))
		}
	}

	// Rating
	ratingStr := article.Find("[itemprop='ratingValue']").Text()
	if ratingStr != "" {
		if rating, err := strconv.ParseFloat(strings.TrimSpace(ratingStr), 64); err == nil {
			movie.Rating = mc.float64Ptr(rating)
		}
	}

	// Duration
	durationStr := article.Find(".duration").Text()
	if durationStr != "" {
		if duration := mc.parseDuration(durationStr); duration > 0 {
			movie.Duration = mc.int64Ptr(int64(duration))
		}
	}

	// Quality
	quality := article.Find(".label").Text()
	if quality == "" {
		quality = article.Find(".episode.complete").Text()
	}
	quality = strings.ReplaceAll(quality, "strong", "")
	movie.LabelQuality = mc.stringPtr(strings.TrimSpace(quality))

	// Genre
	genre := article.Find("[itemprop='genre']").Text()
	movie.Genre = mc.stringPtr(strings.TrimSpace(genre))

	return movie
}

func (mc *MovieClient) parseDuration(durationStr string) int {
	durationStr = strings.ToLower(durationStr)

	re := regexp.MustCompile(`(\d+)h\s*(\d+)m`)
	if matches := re.FindStringSubmatch(durationStr); len(matches) > 2 {
		hours, _ := strconv.Atoi(matches[1])
		minutes, _ := strconv.Atoi(matches[2])
		return hours*60 + minutes
	}

	re = regexp.MustCompile(`(\d+)m`)
	if matches := re.FindStringSubmatch(durationStr); len(matches) > 1 {
		minutes, _ := strconv.Atoi(matches[1])
		return minutes
	}

	re = regexp.MustCompile(`(\d+)\s*menit`)
	if matches := re.FindStringSubmatch(durationStr); len(matches) > 1 {
		minutes, _ := strconv.Atoi(matches[1])
		return minutes
	}

	return 0
}

func (mc *MovieClient) GetMovieList(pathname string, page int) (*types.MovieListResponse, error) {
	var url string

	cleanPathname := mc.makeCleanPathname(pathname)

	if page > 1 {
		url = fmt.Sprintf("%s%s/page/%d/", MovieBaseURL, cleanPathname, page)
	} else {
		url = fmt.Sprintf("%s%s", MovieBaseURL, cleanPathname)
	}

	fmt.Printf("Scraping movie list from URL: %s and page: %d\n", url, page)

	var htmlContent string

	actions := []chromedp.Action{
		chromedp.Navigate(url),
		chromedp.Sleep(3 * time.Second),
		chromedp.OuterHTML("html", &htmlContent),
	}

	err := chromedp.Run(mc.ChromeClient.ctx, actions...)
	if err != nil {
		logger.Logger.Error("Error loading movie list page", zap.Error(err))
		return nil, err
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		logger.Logger.Error("Error parsing movie list HTML", zap.Error(err))
		return nil, err
	}

	return mc.scrapeMovieList(doc), nil
}

func (mc *MovieClient) scrapeMovieList(doc *goquery.Document) *types.MovieListResponse {
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
		movie := mc.parseArticle(item)
		if movie.Title != "" {
			response.Movies = append(response.Movies, *movie)
		}
	})

	pagination := mc.parsePagination(doc)

	response.Pagination = pagination
	response.Pagination.TotalItems = int64(response.Pagination.TotalPage) * int64(len(response.Movies))

	return response
}

func (mc *MovieClient) parsePagination(doc *goquery.Document) types.Pagination {
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
		absoluteURL := mc.makeAbsoluteURL(href)

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
					url := mc.makeAbsoluteURL(href)
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
func (mc *MovieClient) GetMovieDetail(pathname string) (*types.MovieDetail, error) {
	var htmlContent string
	var finalURL string

	cleanPathname := mc.makeCleanPathname(pathname)

	initialURL := fmt.Sprintf("%s%s", MovieBaseURL, cleanPathname)

	if !mc.isValidURL(initialURL) {
		return nil, fmt.Errorf("invalid URL format: %s", initialURL)
	}

	fmt.Printf("Scraping movie detail from URL: %s\n", initialURL)

	actions := []chromedp.Action{
		chromedp.Navigate(finalURL),
		chromedp.Sleep(3 * time.Second),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var currentURL string
			err := chromedp.Run(ctx,
				chromedp.Evaluate(`window.location.href`, &currentURL),
			)
			if err != nil {
				return fmt.Errorf("failed to get current URL: %w", err)
			}

			if currentURL != initialURL {
				fmt.Printf("Redirected to: %s\n", currentURL)
				finalURL = currentURL
			} else {
				finalURL = initialURL
			}

			var pageTitle string
			chromedp.Title(&pageTitle).Do(ctx)

			if strings.Contains(pageTitle, "Mengalihkan") || strings.Contains(pageTitle, "Redirect") {
				fmt.Println("Redirect page detected")

				var targetURL string
				err := chromedp.Run(ctx,
					chromedp.Evaluate(`document.getElementById('openNow')?.href`, &targetURL),
				)
				if err == nil && targetURL != "" && targetURL != "undefined" {
					fmt.Printf("Target URL: %s\n", targetURL)

					fmt.Println("Waiting for automatic redirect...")
					chromedp.Sleep(6 * time.Second).Do(ctx)

					chromedp.Evaluate(`window.location.href`, &currentURL).Do(ctx)

					if currentURL == initialURL || strings.Contains(currentURL, pathname) {
						fmt.Println("Clicking redirect button...")
						chromedp.Click("#openNow", chromedp.NodeVisible).Do(ctx)
						chromedp.Sleep(3 * time.Second).Do(ctx)
						chromedp.Evaluate(`window.location.href`, &currentURL).Do(ctx)
					}

					finalURL = currentURL
				}
			}

			return nil
		}),
		chromedp.Sleep(2 * time.Second),
		chromedp.OuterHTML("html", &htmlContent),
	}

	err := chromedp.Run(mc.ChromeClient.ctx, actions...)
	if err != nil {
		logger.Logger.Error("Error loading movie detail page", zap.Error(err))
		return nil, err
	}

	if finalURL == "" {
		finalURL = initialURL
	}

	fmt.Printf("Final URL after processing: %s\n", finalURL)

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		logger.Logger.Error("Error parsing movie detail HTML", zap.Error(err))
		return nil, err
	}

	detail := mc.scrapeMovieDetail(doc, finalURL)
	detail.SourceUrl = mc.stringPtr(finalURL)

	if mc.isSeriesByFinalURL(finalURL) {
		detail.Type = "series"
	}

	return detail, nil
}

func (mc *MovieClient) scrapeMovieDetail(doc *goquery.Document, originalURL string) *types.MovieDetail {
	detail := &types.MovieDetail{
		Type: "movie",
	}

	detail.OriginalPageUrl = &originalURL

	titleDiv := doc.Find(".movie-info")
	if titleDiv.Length() > 0 {
		h1 := titleDiv.Find("h1")
		if h1.Length() > 0 {
			rawTitle := h1.Text()
			rawTitle = strings.ReplaceAll(rawTitle, "Nonton ", "")
			rawTitle = strings.ReplaceAll(rawTitle, " Sub Indo di Lk21", "")
			detail.Title = strings.TrimSpace(rawTitle)
		}
	}

	synopsisDiv := doc.Find(".synopsis")
	if synopsisDiv.Length() > 0 {
		if synopsis, ok := synopsisDiv.Attr("data-full"); ok && synopsis != "" {
			detail.Synopsis = mc.stringPtr(synopsis)
		} else {
			detail.Synopsis = mc.stringPtr(strings.TrimSpace(synopsisDiv.Text()))
		}
	}

	infoTag := doc.Find(".info-tag")
	if infoTag.Length() > 0 {
		var qualities []string
		var duration string
		var rating float64
		var ageRating string

		infoTag.Find("span").Each(func(i int, span *goquery.Selection) {
			text := strings.TrimSpace(span.Text())
			if text == "" {
				return
			}

			span.Find("i").Remove()
			cleanText := strings.TrimSpace(span.Text())

			if strings.Contains(cleanText, ".") {
				re := regexp.MustCompile(`(\d+\.\d+)`)
				matches := re.FindStringSubmatch(cleanText)
				if len(matches) > 1 {
					if val, err := strconv.ParseFloat(matches[1], 64); err == nil {
						rating = val
						return
					}
				}
			}

			if strings.Contains(cleanText, "+") ||
				strings.Contains(cleanText, "PG") ||
				strings.Contains(cleanText, "R") {
				ageRating = cleanText
				return
			}

			if strings.ContainsAny(cleanText, "h m") &&
				(strings.Contains(cleanText, "h") || strings.Contains(cleanText, "m")) {
				duration = cleanText
				return
			}

			qualityKeywords := []string{"BluRay", "WEB-DL", "HDRip", "4K", "1080p", "720p", "480p", "CAM", "TS", "HDTC"}
			for _, keyword := range qualityKeywords {
				if strings.Contains(cleanText, keyword) {
					qualities = append(qualities, cleanText)
					return
				}
			}

			if strings.Contains(cleanText, "p") || strings.Contains(cleanText, "P") {
				qualities = append(qualities, cleanText)
				return
			}
		})

		if rating > 0 {
			detail.Rating = &rating
		}

		if ageRating != "" {
			detail.AgeRating = &ageRating
		}

		if len(qualities) > 0 {
			qualityStr := strings.Join(qualities, ", ")
			detail.LabelQuality = &qualityStr
		}

		if duration != "" {
			if durationVal := mc.parseDuration(duration); durationVal > 0 {
				detail.Duration = mc.int64Ptr(int64(durationVal))
			}
		}
	}

	if infoTag.Length() > 0 && (detail.Rating == nil || detail.LabelQuality == nil) {
		var texts []string
		infoTag.Find("span").Each(func(i int, span *goquery.Selection) {
			span.Find("i").Remove()
			texts = append(texts, strings.TrimSpace(span.Text()))
		})

		for i, text := range texts {
			if i == 0 && strings.Contains(text, ".") {
				re := regexp.MustCompile(`(\d+\.\d+)`)
				if matches := re.FindStringSubmatch(text); len(matches) > 1 {
					if val, err := strconv.ParseFloat(matches[1], 64); err == nil && val > 0 {
						detail.Rating = &val
						continue
					}
				}
			}

			if strings.Contains(text, "+") && len(text) <= 4 {
				detail.AgeRating = &text
				continue
			}

			if strings.ContainsAny(text, "h m") && (strings.Contains(text, "h") || strings.Contains(text, "m")) {
				if durationVal := mc.parseDuration(text); durationVal > 0 {
					detail.Duration = mc.int64Ptr(int64(durationVal))
				}
				continue
			}

			if text != "" {
				if detail.LabelQuality == nil {
					detail.LabelQuality = &text
				} else {
					*detail.LabelQuality = *detail.LabelQuality + ", " + text
				}
			}
		}
	}

	tagList := doc.Find(".tag-list")
	if tagList.Length() > 0 {
		var genres []string
		var genreObjs []types.Genre
		var countries []string
		var countryObjs []types.CountryMovie

		tagList.Find("a").Each(func(i int, a *goquery.Selection) {
			href, _ := a.Attr("href")
			text := strings.TrimSpace(a.Text())
			absoluteURL := mc.makeAbsoluteURL(href)

			if strings.Contains(href, "/genre/") {
				genres = append(genres, text)

				genreObjs = append(genreObjs, types.Genre{
					Name:    mc.stringPtr(text),
					PageUrl: mc.stringPtr(absoluteURL),
				})
			} else if strings.Contains(href, "/country/") {
				countries = append(countries, text)

				countryObjs = append(countryObjs, types.CountryMovie{
					Name:    mc.stringPtr(text),
					PageUrl: mc.stringPtr(absoluteURL),
				})
			}
		})

		if len(genreObjs) > 0 {
			detail.Genres = &genreObjs
		}
		if len(countryObjs) > 0 {
			detail.Countries = &countryObjs
		}

		if len(genres) > 0 {
			genresStr := strings.Join(genres, ", ")
			detail.Genre = &genresStr
		}

	}

	detailDiv := doc.Find(".detail")
	if detailDiv.Length() > 0 {
		if img, ok := detailDiv.Find("img[itemprop='image']").Attr("src"); ok {
			detail.Thumbnail = mc.stringPtr(mc.makeAbsoluteURL(img))
		}

		detailDiv.Find("p").Each(func(i int, p *goquery.Selection) {
			text := strings.TrimSpace(p.Text())

			if strings.Contains(text, "Sutradara:") {
				var directors []types.MoviePerson
				p.Find("a").Each(func(i int, a *goquery.Selection) {
					href, _ := a.Attr("href")
					directors = append(directors, types.MoviePerson{
						Name:    mc.stringPtr(strings.TrimSpace(a.Text())),
						PageUrl: mc.stringPtr(mc.makeAbsoluteURL(href)),
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
						Name:    mc.stringPtr(strings.TrimSpace(a.Text())),
						PageUrl: mc.stringPtr(mc.makeAbsoluteURL(href)),
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
						detail.Votes = mc.int64Ptr(votes)
					}
				}
			} else if strings.Contains(text, "Release:") || strings.Contains(text, "Rilis:") {
				// Extract date part after "Release:" or "Rilis:"
				parts := strings.SplitN(text, ":", 2)
				if len(parts) > 1 {
					dateStr := strings.TrimSpace(parts[1])

					formats := []string{
						"2 Jan 2006",     // "23 Mar 2016"
						"2 January 2006", // "23 March 2016"
						"2006-01-02",     // "2016-03-23"
						"02-01-2006",     // "23-03-2016"
						"01/02/2006",     // "03/23/2016"
					}

					for _, format := range formats {
						if t, err := time.Parse(format, dateStr); err == nil {
							detail.ReleaseDate = &t
							break
						}
					}

					if detail.ReleaseDate == nil {
						// Coba ekstrak pattern "DD MMM YYYY"
						re := regexp.MustCompile(`(\d{1,2})\s+([A-Za-z]+)\s+(\d{4})`)
						if matches := re.FindStringSubmatch(dateStr); len(matches) > 3 {
							// Convert month name to number
							monthStr := matches[2]
							monthMap := map[string]string{
								"Jan": "01", "Feb": "02", "Mar": "03", "Apr": "04",
								"May": "05", "Jun": "06", "Jul": "07", "Aug": "08",
								"Sep": "09", "Oct": "10", "Nov": "11", "Dec": "12",
							}
							if month, ok := monthMap[monthStr]; ok {
								formattedDate := fmt.Sprintf("%s-%s-%02s", matches[3], month, matches[1])
								if t, err := time.Parse("2006-01-02", formattedDate); err == nil {
									detail.ReleaseDate = &t
								}
							}
						}
					}
				}
			} else if strings.Contains(text, "Updated:") || strings.Contains(text, "Diperbarui:") {
				parts := strings.SplitN(text, ":", 2)
				if len(parts) > 1 {
					dateStr := strings.TrimSpace(parts[1])

					formats := []string{
						"2 Jan 2006 15:04:05",     // "05 Dec 2019 11:40:44"
						"2 January 2006 15:04:05", // "05 December 2019 11:40:44"
						"2006-01-02 15:04:05",     // "2019-12-05 11:40:44"
						"2 Jan 2006",              // "05 Dec 2019"
						"2006-01-02",              // "2019-12-05"
					}

					for _, format := range formats {
						if t, err := time.Parse(format, dateStr); err == nil {
							detail.UpdatedAt = &t
							break
						}
					}
				}
			}

		})
	}

	trailerLink := doc.Find(".action-left li a.yt-lightbox").First()
	if trailerLink.Length() > 0 {
		if trailerURL, exists := trailerLink.Attr("href"); exists && trailerURL != "" && trailerURL != "#" {
			detail.TrailerUrl = mc.stringPtr(trailerURL)
		}
	}
	if detail.TrailerUrl == nil {
		doc.Find("a").Each(func(i int, a *goquery.Selection) {
			text := strings.ToLower(strings.TrimSpace(a.Text()))
			href, exists := a.Attr("href")
			if !exists || href == "" || href == "#" {
				return
			}

			if strings.Contains(text, "trailer") || a.HasClass("yt-lightbox") {
				if strings.Contains(href, "youtube.com") || strings.Contains(href, "youtu.be") {
					detail.TrailerUrl = mc.stringPtr(href)
					return
				}
			}
		})
	}

	var playerURLs []types.PlayerUrl
	doc.Find("#player-list li a").Each(func(i int, a *goquery.Selection) {
		href, exists := a.Attr("href")
		if !exists || href == "" {
			return
		}

		serverType, _ := a.Attr("data-server")
		if serverType == "" {
			serverType = strings.TrimSpace(a.Text())
		}

		dataURL, _ := a.Attr("data-url")
		if dataURL != "" && dataURL != href {
			href = dataURL
		}

		playerURL := types.PlayerUrl{
			URL:  mc.stringPtr(mc.makeAbsoluteURL(href)),
			Type: mc.stringPtr(strings.ToUpper(serverType)),
		}

		playerURLs = append(playerURLs, playerURL)
	})
	doc.Find("#player-select option").Each(func(i int, option *goquery.Selection) {
		value, exists := option.Attr("value")
		if !exists || value == "" {
			return
		}

		serverType, _ := option.Attr("data-server")
		if serverType == "" {
			text := strings.TrimSpace(option.Text())
			if strings.HasPrefix(text, "GANTI PLAYER ") {
				serverType = strings.TrimPrefix(text, "GANTI PLAYER ")
			} else {
				serverType = text
			}
		}

		exists = false
		for _, p := range playerURLs {
			if p.URL != nil && *p.URL == mc.makeAbsoluteURL(value) {
				exists = true
				break
			}
		}

		if !exists {
			playerURLs = append(playerURLs, types.PlayerUrl{
				URL:  mc.stringPtr(mc.makeAbsoluteURL(value)),
				Type: mc.stringPtr(strings.ToUpper(serverType)),
			})
		}
	})

	if len(playerURLs) > 0 {
		detail.PlayerUrl = &playerURLs
	}

	downloadSelectors := []string{
		".movie-action a[title*='Download']",
		".movie-action a.btn[href*='dl.']",
		".movie-action a[href*='download']",
		".download-link a",
		"a.btn[title*='Download']",
		".movie-action a.btn:contains('DOWNLOAD')",
	}
	for _, selector := range downloadSelectors {
		downloadLink := doc.Find(selector).First()
		if downloadLink.Length() > 0 {
			if href, exists := downloadLink.Attr("href"); exists && href != "" && href != "#" {
				detail.DownloadLink = mc.stringPtr(href)
				break
			}
		}
	}

	similarMovies := mc.parseSimilarMovies(doc)
	if len(similarMovies) > 0 {
		detail.SimilarMovies = &similarMovies
	}

	return detail
}

func (mc *MovieClient) looksLikeSeriesPage(doc *goquery.Document) bool {
	if doc.Find("#season-data").Length() > 0 {
		return true
	}
	if doc.Find("select.season-select").Length() > 0 {
		return true
	}
	episodeCount := 0
	doc.Find("a[href]").Each(func(i int, a *goquery.Selection) {
		href, _ := a.Attr("href")
		if regexp.MustCompile(`(?i)season-\d+-episode-\d+`).MatchString(href) {
			episodeCount++
			if episodeCount >= 3 {
				return
			}
		}
	})
	return episodeCount >= 3
}

func (mc *MovieClient) GetLatest(page int) (*types.MovieListResponse, error) {
	return mc.GetMovieList(MovieBaseURL+"/latest-movies/", page)
}

func (mc *MovieClient) GetTopRated(page int) (*types.MovieListResponse, error) {
	return mc.GetMovieList(MovieBaseURL+"/top-rated/", page)
}

func (mc *MovieClient) Search(query string, page int) (*types.MovieListResponse, error) {
	var url string
	if page > 1 {
		url = fmt.Sprintf("%s/search?s=%s&page=%d", MovieBaseURL, query, page)
	} else {
		url = fmt.Sprintf("%s/search?s=%s", MovieBaseURL, query)
	}

	fmt.Printf("Scraping search results for query: %s and page: %d\n", query, page)
	fmt.Printf("Search URL: %s\n", url)

	var htmlContent string

	actions := []chromedp.Action{
		chromedp.Navigate(url),
		chromedp.Sleep(3 * time.Second),
		chromedp.OuterHTML("html", &htmlContent),
	}

	err := chromedp.Run(mc.ChromeClient.ctx, actions...)
	if err != nil {
		logger.Logger.Error("Error loading search page", zap.Error(err))
		return nil, err
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		logger.Logger.Error("Error parsing search HTML", zap.Error(err))
		return nil, err
	}

	return mc.scrapeMovieList(doc), nil
}

// Helper functions
func (mc *MovieClient) parseSimilarMovies(doc *goquery.Document) []types.Movie {
	var similarMovies []types.Movie

	doc.Find(".related-content .video-list li").Each(func(i int, li *goquery.Selection) {
		// Cari link utama
		a := li.Find("a")
		if a.Length() == 0 {
			return
		}

		// Ambil href untuk original page URL
		href, exists := a.Attr("href")
		if !exists {
			return
		}
		originalPageURL := mc.makeAbsoluteURL(href)

		// Ambil gambar
		var thumbnail string
		img := li.Find("img")
		if img.Length() > 0 {
			// Coba ambil dari srcset dulu (untuk kualitas lebih baik)
			if srcset, exists := img.Attr("srcset"); exists && srcset != "" {
				// Ambil URL pertama dari srcset
				parts := strings.Fields(srcset)
				if len(parts) > 0 {
					thumbnail = parts[0]
				}
			}

			// Fallback ke src jika srcset tidak ada
			if thumbnail == "" {
				if src, exists := img.Attr("src"); exists {
					thumbnail = src
				}
			}
		}

		// Ambil informasi dari video-info
		videoInfo := li.Find(".video-info")
		if videoInfo.Length() == 0 {
			return
		}

		// Ambil title
		title := strings.TrimSpace(videoInfo.Find(".video-title").Text())
		if title == "" {
			// Fallback ke alt gambar
			if alt, exists := img.Attr("alt"); exists {
				title = strings.TrimSpace(alt)
				// Hapus tahun jika ada di alt (format: "Title (Year)")
				title = regexp.MustCompile(`\s*\(\d{4}\)$`).ReplaceAllString(title, "")
			}
		}

		// Ambil year
		var year int32
		yearText := strings.TrimSpace(videoInfo.Find(".video-year").Text())
		if yearText != "" {
			if y, err := strconv.ParseInt(yearText, 10, 32); err == nil {
				year = int32(y)
			}
		}

		// Jika year tidak ditemukan, coba ekstrak dari alt
		if year == 0 {
			if alt, exists := img.Attr("alt"); exists {
				re := regexp.MustCompile(`\((\d{4})\)`)
				if matches := re.FindStringSubmatch(alt); len(matches) > 1 {
					if y, err := strconv.ParseInt(matches[1], 10, 32); err == nil {
						year = int32(y)
					}
				}
			}
		}

		// Buat movie object
		movie := types.Movie{
			Title:           title,
			Thumbnail:       mc.stringPtr(mc.makeAbsoluteURL(thumbnail)),
			Year:            &year,
			OriginalPageUrl: mc.stringPtr(originalPageURL),
		}

		// Hanya tambahkan jika title tidak kosong
		if movie.Title != "" {
			similarMovies = append(similarMovies, movie)
		}
	})

	// Fallback: coba selector lain jika tidak ditemukan
	if len(similarMovies) == 0 {
		doc.Find(".related-content .video-list-wrapper .video-list li").Each(func(i int, li *goquery.Selection) {
			a := li.Find("a")
			if a.Length() == 0 {
				return
			}

			href, _ := a.Attr("href")
			img := li.Find("img")

			var thumbnail string
			if img.Length() > 0 {
				if src, exists := img.Attr("src"); exists {
					thumbnail = src
				}
			}

			title := strings.TrimSpace(li.Find(".video-title").Text())
			if title == "" {
				if alt, exists := img.Attr("alt"); exists {
					title = alt
				}
			}

			var year int32
			yearText := strings.TrimSpace(li.Find(".video-year").Text())
			if yearText != "" {
				if y, err := strconv.ParseInt(yearText, 10, 32); err == nil {
					year = int32(y)
				}
			}

			if title != "" {
				similarMovies = append(similarMovies, types.Movie{
					Title:           title,
					Thumbnail:       mc.stringPtr(mc.makeAbsoluteURL(thumbnail)),
					Year:            &year,
					OriginalPageUrl: mc.stringPtr(mc.makeAbsoluteURL(href)),
				})
			}
		})
	}

	return similarMovies
}
func (mc *MovieClient) scrapeAllLatestMovies(doc *goquery.Document) []types.Movie {
	var movies []types.Movie

	headers := []string{"Daftar Lengkap Series Terbaru", "Daftar Lengkap Film Terbaru", "All Latest Movies"}

	for _, headerText := range headers {
		doc.Find("h2").Each(func(i int, h *goquery.Selection) {
			if strings.TrimSpace(h.Text()) == headerText {
				headerDiv := h.Parent()
				if headerDiv.Length() > 0 {
					gallery := headerDiv.NextFiltered(".gallery-grid")
					if gallery.Length() > 0 {
						movies = mc.scrapeGalleryMovies(gallery)
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

func (mc *MovieClient) scrapeGalleryMovies(s *goquery.Selection) []types.Movie {
	var movies []types.Movie

	s.Find("article").Each(func(i int, item *goquery.Selection) {
		movie := mc.parseArticle(item)
		if movie.Title != "" {
			movies = append(movies, *movie)
		}
	})

	return movies
}

func (mc *MovieClient) makeAbsoluteURL(url string) string {
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

	baseURL := MovieBaseURL
	if !strings.HasPrefix(url, "/") {
		url = "/" + url
	}

	if strings.Contains(url, "nontondrama?page=") {
		url = strings.Replace(url, "nontondrama?page=", "", 1)
		baseURL = SeriesBaseURL
	}

	return baseURL + url
}

func (mc *MovieClient) getViewAllURL(s *goquery.Selection) *string {
	var viewAllURL *string

	parent := s.Closest(".widget, .container").First()
	if parent.Length() == 0 {
		return nil
	}

	switch {
	case parent.HasClass("widget"):
		viewAllURL = mc.stringPtr(strings.TrimSpace(parent.Find(".header a.btn").AttrOr("href", "")))

	case parent.HasClass("container"):
		moreFeatured := parent.Find(".more-featured a.btn")
		if moreFeatured.Length() > 0 {
			viewAllURL = mc.stringPtr(strings.TrimSpace(moreFeatured.AttrOr("href", "")))
		}

		if viewAllURL == nil {
			parent.Find("a.btn").Each(func(i int, link *goquery.Selection) {
				if strings.Contains(strings.ToLower(link.Text()), "semua") {
					viewAllURL = mc.stringPtr(strings.TrimSpace(link.AttrOr("href", "")))
				}
			})
		}

	default:

		viewAllURL = mc.stringPtr(strings.TrimSpace(parent.Find("a.btn[href]").AttrOr("href", "")))
	}

	return viewAllURL
}

func (mc *MovieClient) makeCleanPathname(pathname string) string {
	pathname = strings.TrimSpace(pathname)

	pathname = strings.TrimPrefix(pathname, "/")

	pathname = url.PathEscape(pathname)

	if pathname == "" {
		pathname = "/"
	} else {
		pathname = "/" + pathname
	}

	return pathname
}
func (mc *MovieClient) extractRedirectURL(htmlContent string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return ""
	}

	if link := doc.Find("#openNow"); link.Length() > 0 {
		if href, exists := link.Attr("href"); exists {
			return href
		}
	}

	if link := doc.Find("a.primary"); link.Length() > 0 {
		if href, exists := link.Attr("href"); exists {
			return href
		}
	}

	return ""
}
func (mc *MovieClient) isSeriesByFinalURL(url string) bool {
	seriesPatterns := []string{
		"nontondrama",
		"series.",
		"/drama/",
		"episode",
		"season",
		"tv-show",
	}

	urlLower := strings.ToLower(url)
	for _, pattern := range seriesPatterns {
		if strings.Contains(urlLower, pattern) {
			return true
		}
	}
	return false
}
func (mc *MovieClient) handleRedirectPage(ctx context.Context, initialURL string) (string, error) {
	var finalURL string
	var htmlContent string

	err := chromedp.Run(ctx,
		chromedp.Navigate(initialURL),
		chromedp.Sleep(2*time.Second),
		chromedp.OuterHTML("html", &htmlContent),
	)
	if err != nil {
		return initialURL, err
	}

	targetURL := mc.extractRedirectURL(htmlContent)
	if targetURL == "" {
		return initialURL, nil
	}

	fmt.Printf("Extracted target URL: %s\n", targetURL)

	fmt.Println("Waiting for automatic redirect...")
	time.Sleep(6 * time.Second)

	chromedp.Evaluate(`window.location.href`, &finalURL).Do(ctx)

	if finalURL == initialURL || strings.Contains(finalURL, "no-tail-tell-2026") {
		fmt.Println("Automatic redirect failed, clicking button...")
		err = chromedp.Run(ctx,
			chromedp.Click("#openNow", chromedp.NodeVisible),
			chromedp.Sleep(3*time.Second),
			chromedp.Evaluate(`window.location.href`, &finalURL),
		)
		if err != nil {
			return targetURL, nil
		}
	}

	if finalURL != "" {
		return finalURL, nil
	}

	return targetURL, nil
}
func (mc *MovieClient) isValidURL(urlStr string) bool {
	if urlStr == "" {
		return false
	}

	u, err := url.Parse(urlStr)
	if err != nil {
		return false
	}

	if u.Scheme == "" || u.Host == "" {
		return false
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}

	return true
}

func (mc *MovieClient) stringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (mc *MovieClient) int32Ptr(i int32) *int32 {
	return &i
}

func (mc *MovieClient) int64Ptr(i int64) *int64 {
	return &i
}

func (mc *MovieClient) float64Ptr(f float64) *float64 {
	return &f
}
func (mc *MovieClient) timePtr(t time.Time) *time.Time {
	return &t
}

// FetchHTML fetches HTML content using chromedp
func (mc *MovieClient) FetchHTML(url string) (string, error) {
	var htmlContent string

	actions := []chromedp.Action{
		chromedp.Navigate(url),
		chromedp.Sleep(3 * time.Second),
		chromedp.OuterHTML("html", &htmlContent),
	}

	err := chromedp.Run(mc.ChromeClient.ctx, actions...)
	if err != nil {
		return "", err
	}

	return htmlContent, nil
}

// FetchHTMLWithContext fetches HTML using provided context
func (mc *MovieClient) FetchHTMLWithContext(ctx context.Context, url string) (string, error) {
	var htmlContent string

	actions := []chromedp.Action{
		chromedp.Navigate(url),
		chromedp.Sleep(3 * time.Second),
		chromedp.OuterHTML("html", &htmlContent),
	}

	err := chromedp.Run(ctx, actions...)
	if err != nil {
		return "", err
	}

	return htmlContent, nil
}
