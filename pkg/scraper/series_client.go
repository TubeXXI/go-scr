package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"tubexxi/scraper/pkg/logger"
	"tubexxi/scraper/pkg/types"

	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"
	"github.com/google/uuid"
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

// GetHome scrapes the home page and returns categories with Key, Value, and ViewAllUrl
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
				absoluteURL := sc.makeAbsoluteSlugURL(*url)
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
		latestURL := sc.makeAbsoluteSlugURL("/latest")
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

	id := uuid.New()
	movie.ID = id

	title := article.Find(".poster-title").Text()
	if title == "" {
		title = article.Find(".video-title").Text()
	}
	movie.Title = strings.TrimSpace(title)

	var originalPageURL string
	article.Find("a[itemprop='url']").Each(func(i int, a *goquery.Selection) {
		if href, ok := a.Attr("href"); ok {
			originalPageURL = sc.makeAbsoluteSlugURL(href)
		}
	})
	if originalPageURL == "" {
		article.Find("a[href]").Each(func(i int, a *goquery.Selection) {
			if href, ok := a.Attr("href"); ok && strings.Contains(href, "/series/") {
				originalPageURL = sc.makeAbsoluteSlugURL(href)
			}
		})
	}
	movie.OriginalPageUrl = &originalPageURL

	if img, ok := article.Find("img[itemprop='image']").Attr("src"); ok {
		movie.Thumbnail = sc.stringPtr(sc.makeAbsoluteURL(img))
	}
	if movie.Thumbnail == nil {
		if img, ok := article.Find(".poster img").Attr("src"); ok {
			movie.Thumbnail = sc.stringPtr(img)
		}
	}

	yearStr := article.Find(".year").Text()
	if yearStr == "" {
		yearStr = article.Find(".video-year").Text()
	}
	if yearStr != "" {
		if year, err := strconv.Atoi(strings.TrimSpace(yearStr)); err == nil {
			movie.Year = sc.int32Ptr(int32(year))
		}
	}

	ratingStr := article.Find("[itemprop='ratingValue']").Text()
	if ratingStr != "" {
		if rating, err := strconv.ParseFloat(strings.TrimSpace(ratingStr), 64); err == nil {
			movie.Rating = sc.float64Ptr(rating)
		}
	}
	if movie.Rating == nil {
		rawRating := strings.TrimSpace(article.Find(".rating").Text())
		if rawRating != "" {
			re := regexp.MustCompile(`[0-9.]+`)
			match := re.FindString(rawRating)
			if match != "" {
				if val, err := strconv.ParseFloat(match, 64); err == nil {
					movie.Rating = sc.float64Ptr(val)
				}
			}
		}
	}

	durationStr := article.Find(".duration").Text()
	if durationStr != "" {
		if duration := sc.parseDuration(durationStr); duration > 0 {
			movie.Duration = sc.int64Ptr(int64(duration))
		}
	}
	if movie.Duration == nil {
		if dur := strings.TrimSpace(article.Find(".poster .duration").Text()); dur != "" {
			parts := strings.Split(dur, ":")
			if len(parts) == 2 {
				minutes, _ := strconv.Atoi(parts[0])
				seconds, _ := strconv.Atoi(parts[1])

				totalMinutes := minutes
				if seconds >= 30 {
					totalMinutes++
				}
				movie.Duration = sc.int64Ptr(int64(totalMinutes))
			}
		}
	}

	// Quality
	quality := article.Find(".label").Text()
	if quality == "" {
		quality = article.Find(".episode.complete").Text()
	}
	quality = strings.ReplaceAll(quality, "strong", "")
	movie.LabelQuality = sc.stringPtr(strings.TrimSpace(quality))
	if movie.LabelQuality == nil {
		episodeSpan := article.Find(".episode")
		if episodeSpan.Length() > 0 {
			prefix := ""
			episodeSpan.Contents().Each(func(i int, s *goquery.Selection) {
				if goquery.NodeName(s) == "#text" {
					prefix = strings.TrimSpace(s.Text())
				}
			})

			episodeNum := strings.TrimSpace(episodeSpan.Find("strong").Text())

			if prefix != "" && episodeNum != "" {
				movie.LabelQuality = sc.stringPtr(fmt.Sprintf("%s %s", prefix, episodeNum))
			} else {
				movie.LabelQuality = sc.stringPtr(strings.TrimSpace(episodeSpan.Text()))
			}
		}
	}

	// Genre
	genre := article.Find(".genre").Text()
	movie.Genre = sc.stringPtr(strings.TrimSpace(genre))

	return movie
}

// Series List
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

// Series Details
func (sc *SeriesClient) GetSeriesDetail(pathname string) (*types.SeriesDetail, error) {
	var htmlContent string
	var finalURL string

	cleanPathname := sc.makeCleanPathname(pathname)
	initialURL := fmt.Sprintf("%s%s", SeriesBaseURL, cleanPathname)

	if !sc.isValidURL(initialURL) {
		return nil, fmt.Errorf("invalid URL format: %s", initialURL)
	}

	fmt.Printf("Scraping series detail from URL: %s\n", initialURL)

	actions := []chromedp.Action{
		chromedp.Navigate(initialURL),
		sc.clickIfExist(`//a[contains(text(), "KLIK UNTUK MELANJUTKAN")]`, true),
		chromedp.WaitVisible(`.movie-info`, chromedp.ByQuery),
		chromedp.Location(&finalURL),
		chromedp.OuterHTML("html", &htmlContent),
	}

	err := chromedp.Run(sc.ChromeClient.ctx, actions...)
	if err != nil {
		logger.Logger.Error("Error loading series detail page", zap.Error(err))
		return nil, err
	}

	fmt.Printf("Final URL captured: %s\n", finalURL)

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		logger.Logger.Error("Error parsing series detail HTML", zap.Error(err))
		return nil, err
	}

	detail := sc.scrapeSeriesDetail(doc, finalURL)
	detail.ID = uuid.New()
	detail.SourceUrl = sc.stringPtr(finalURL)

	if sc.isMoviesByFinalURL(finalURL) || sc.looksLikeMoviesPage(doc) {
		detail.Type = "movie"
	}

	return detail, nil
}
func (sc *SeriesClient) scrapeSeriesDetail(doc *goquery.Document, originalURL string) *types.SeriesDetail {
	detail := &types.SeriesDetail{
		Type: "series",
	}

	detail.OriginalPageUrl = &originalURL

	movieInfo := doc.Find(".movie-info")
	if movieInfo.Length() > 0 {
		h1 := movieInfo.Find("h1")
		if h1.Length() > 0 {
			rawTitle := h1.Text()
			rawTitle = strings.ReplaceAll(rawTitle, "Nonton ", "")
			rawTitle = strings.ReplaceAll(rawTitle, " Sub Indo di Lk21", "")
			rawTitle = strings.ReplaceAll(rawTitle, " Sub Indo", "")
			detail.Title = strings.TrimSpace(rawTitle)
		}

		synopsisDiv := movieInfo.Find(".synopsis")
		if synopsisDiv.Length() > 0 {
			if synopsis, ok := synopsisDiv.Attr("data-full"); ok && synopsis != "" {
				detail.Synopsis = sc.stringPtr(synopsis)
			} else {
				detail.Synopsis = sc.stringPtr(strings.TrimSpace(synopsisDiv.Text()))
			}
		}

		// var seasonName string
		// movieInfo.Find(".meta-info p").EachWithBreak(func(i int, s *goquery.Selection) bool {
		// 	spanText := strings.TrimSpace(s.Find("span").Text())
		// 	if strings.Contains(spanText, "Terbaru:") || strings.Contains(spanText, "Terbaru") {
		// 		seasonName = strings.TrimSpace(s.Find("a").Text())
		// 		return false
		// 	}
		// 	return true
		// })

		// if seasonName != "" {
		// 	detail.SeasonName = &seasonName
		// }

		infoTag := movieInfo.Find(".info-tag")
		if infoTag.Length() > 0 {
			var status string
			var rating float64
			var releaseTimestamp *int64

			spans := infoTag.Find("span")
			spans.Each(func(i int, span *goquery.Selection) {
				text := strings.TrimSpace(span.Text())

				if text == "" {
					return
				}

				if span.Find("i.fa-star").Length() > 0 {
					cleanText := strings.TrimSpace(span.Text())
					re := regexp.MustCompile(`(\d+\.\d+)`)
					if matches := re.FindStringSubmatch(cleanText); len(matches) > 1 {
						if val, err := strconv.ParseFloat(matches[1], 64); err == nil {
							rating = val
							return
						}
					}
				}

				if strings.Contains(text, "Complete") || strings.Contains(text, "Ongoing") {
					status = text
					return
				}

				datePattern := regexp.MustCompile(`^(\d{1,2})\.(\d{1,2})\.(\d{4})$`)
				if matches := datePattern.FindStringSubmatch(text); len(matches) == 4 {
					day, _ := strconv.Atoi(matches[1])
					month, _ := strconv.Atoi(matches[2])
					year, _ := strconv.Atoi(matches[3])

					if day >= 1 && day <= 31 && month >= 1 && month <= 12 && year >= 1900 {
						releaseDate := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
						timestamp := releaseDate.Unix()
						releaseTimestamp = &timestamp
						return
					}
				}
			})

			if rating > 0 {
				detail.Rating = &rating
			}

			if status != "" {
				detail.Status = &status
			}
			if releaseTimestamp != nil {
				release := time.Unix(*releaseTimestamp, 0)
				detail.ReleaseDate = &release
				detail.DatePublished = &release
			}

		}

		tagList := movieInfo.Find(".tag-list")
		if tagList.Length() > 0 {
			var genres []string
			var genreObjs []types.Genre
			var countries []types.CountryMovie

			tagList.Find("a").Each(func(i int, a *goquery.Selection) {
				href, _ := a.Attr("href")
				text := strings.TrimSpace(a.Text())

				if strings.Contains(href, "/genre/") {
					genres = append(genres, text)
					genreObjs = append(genreObjs, types.Genre{
						Name:    sc.stringPtr(text),
						PageUrl: sc.stringPtr(sc.makeAbsoluteSlugURL(href)),
					})
				} else if strings.Contains(href, "/country/") {
					countries = append(countries, types.CountryMovie{
						Name:    sc.stringPtr(text),
						PageUrl: sc.stringPtr(sc.makeAbsoluteSlugURL(href)),
					})
				}
			})

			if len(genreObjs) > 0 {
				detail.Genres = &genreObjs
			}
			if len(countries) > 0 {
				detail.Countries = &countries
			}

			if len(genres) > 0 {
				genresStr := strings.Join(genres, ", ")
				detail.Genre = &genresStr
			}

		}

		detailDiv := movieInfo.Find(".detail")
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
							PageUrl: sc.stringPtr(sc.makeAbsoluteSlugURL(href)),
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
							PageUrl: sc.stringPtr(sc.makeAbsoluteSlugURL(href)),
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
	}

	if iframe, ok := doc.Find(".simple-box iframe").Attr("src"); ok {
		detail.TrailerUrl = sc.stringPtr(iframe)
	}

	seasonList, seasonName, _ := sc.parseSeasonList(doc)
	if len(seasonList) > 0 {
		detail.SeasonList = &seasonList
	}
	if seasonName != nil && *seasonName != "" {
		detail.SeasonName = seasonName
	}

	similarMovies := sc.parseSimilarSeries(doc)
	if len(similarMovies) > 0 {
		detail.SimilarMovies = &similarMovies
	}

	return detail
}
func (sc *SeriesClient) parseSeasonList(doc *goquery.Document) ([]types.SeasonList, *string, *string) {
	var seasonList []types.SeasonList
	var seasonName *string
	var status *string

	doc.Find("p, div, span").Each(func(i int, s *goquery.Selection) {
		text := s.Text()
		if strings.Contains(strings.ToLower(text), "status:") {
			statusText := strings.TrimSpace(strings.ReplaceAll(text, "Status:", ""))
			statusText = strings.TrimSpace(strings.ReplaceAll(statusText, "status:", ""))
			if statusText != "" {
				status = &statusText
			}
		}
	})

	seasonDataEl := doc.Find("#season-data")
	if seasonDataEl.Length() > 0 {
		jsonData := strings.TrimSpace(seasonDataEl.Text())
		if jsonData != "" {
			var seasonData map[string]interface{}
			err := json.Unmarshal([]byte(jsonData), &seasonData)
			if err == nil {
				episodesBySeason := make(map[int]map[int]string)

				for seasonKey, epsValue := range seasonData {
					seasonNum, err := strconv.Atoi(seasonKey)
					if err != nil {
						continue
					}

					eps, ok := epsValue.([]interface{})
					if !ok {
						continue
					}

					episodeMap := make(map[int]string)
					for _, epValue := range eps {
						ep, ok := epValue.(map[string]interface{})
						if !ok {
							continue
						}

						var episodeNum int
						if epNo, ok := ep["episode_no"]; ok {
							episodeNum, _ = strconv.Atoi(fmt.Sprintf("%v", epNo))
						} else if epNum, ok := ep["episode"]; ok {
							episodeNum, _ = strconv.Atoi(fmt.Sprintf("%v", epNum))
						} else {
							continue
						}

						slug, ok := ep["slug"].(string)
						if !ok || slug == "" {
							continue
						}

						url := sc.makeAbsoluteURL("/" + strings.TrimLeft(slug, "/"))
						episodeMap[episodeNum] = url
					}

					if len(episodeMap) > 0 {
						episodesBySeason[seasonNum] = episodeMap
					}
				}

				currentSeasonNum := 0
				watchDataEl := doc.Find("#watch-history-data")
				if watchDataEl.Length() > 0 {
					watchDataJson := strings.TrimSpace(watchDataEl.Text())
					if watchDataJson != "" {
						var watchData map[string]interface{}
						err = json.Unmarshal([]byte(watchDataJson), &watchData)
						if err == nil {
							if cs, ok := watchData["current_season"]; ok {
								currentSeasonNum, _ = strconv.Atoi(fmt.Sprintf("%v", cs))
							}
						}
					}
				}

				if len(episodesBySeason) > 0 {
					var seasonNums []int
					maxSeason := 0
					for seasonNum := range episodesBySeason {
						seasonNums = append(seasonNums, seasonNum)
						if seasonNum > maxSeason {
							maxSeason = seasonNum
						}
					}
					sort.Ints(seasonNums)

					totalSeasons := int32(maxSeason)

					if currentSeasonNum == 0 && len(seasonNums) > 0 {
						currentSeasonNum = seasonNums[len(seasonNums)-1] // last/max season
					}
					seasonNameStr := fmt.Sprintf("Season %d", currentSeasonNum)
					seasonName = &seasonNameStr

					for _, seasonNum := range seasonNums {
						episodeMap := episodesBySeason[seasonNum]

						var episodes []types.EpisodeList
						var episodeNums []int
						for epNum := range episodeMap {
							episodeNums = append(episodeNums, epNum)
						}
						sort.Ints(episodeNums)

						for _, epNum := range episodeNums {
							ep := types.EpisodeList{
								EpisodeNumber: sc.int32Ptr(int32(epNum)),
								EpisodeUrl:    sc.stringPtr(episodeMap[epNum]),
							}
							episodes = append(episodes, ep)
						}

						season := types.SeasonList{
							CurrentSeason: sc.int32Ptr(int32(seasonNum)),
							TotalSeason:   sc.int32Ptr(totalSeasons),
							EpisodeList:   &episodes,
						}
						seasonList = append(seasonList, season)
					}

					return seasonList, seasonName, status
				}
			}
		}
	}

	episodesBySeason := make(map[int]map[int]string)

	doc.Find("a[href*='season-'][href*='-episode-']").Each(func(i int, a *goquery.Selection) {
		href, exists := a.Attr("href")
		if !exists {
			return
		}

		re := regexp.MustCompile(`season-(\d+)-episode-(\d+)`)
		matches := re.FindStringSubmatch(href)
		if len(matches) < 3 {
			return
		}

		seasonNum, _ := strconv.Atoi(matches[1])
		episodeNum, _ := strconv.Atoi(matches[2])

		if _, ok := episodesBySeason[seasonNum]; !ok {
			episodesBySeason[seasonNum] = make(map[int]string)
		}

		episodesBySeason[seasonNum][episodeNum] = sc.makeAbsoluteURL(href)
	})

	if len(episodesBySeason) == 0 {
		currentSeasonNum := 1
		totalSeasons := 1

		seasonSelect := doc.Find("select.season-select")
		if seasonSelect.Length() > 0 {
			options := seasonSelect.Find("option")
			totalSeasons = options.Length()

			options.EachWithBreak(func(i int, opt *goquery.Selection) bool {
				if _, exists := opt.Attr("selected"); exists {
					val, _ := opt.Attr("value")
					if num, err := strconv.Atoi(val); err == nil {
						currentSeasonNum = num
					}
					seasonNameStr := strings.TrimSpace(opt.Text())
					if seasonNameStr != "" {
						seasonName = &seasonNameStr
					}
					return false
				}
				return true
			})

			if seasonName == nil && options.Length() > 0 {
				firstOpt := options.First()
				val, _ := firstOpt.Attr("value")
				if num, err := strconv.Atoi(val); err == nil {
					currentSeasonNum = num
				}
				seasonNameStr := strings.TrimSpace(firstOpt.Text())
				if seasonNameStr != "" {
					seasonName = &seasonNameStr
				}
			}
		}

		var episodes []types.EpisodeList
		episodeList := doc.Find("ul.episode-list li a")
		episodeList.Each(func(i int, a *goquery.Selection) {
			href, exists := a.Attr("href")
			if !exists {
				return
			}

			epText := strings.TrimSpace(a.Text())
			epNum, _ := strconv.Atoi(epText)

			if epNum == 0 {
				re := regexp.MustCompile(`episode-(\d+)`)
				if matches := re.FindStringSubmatch(href); len(matches) > 1 {
					epNum, _ = strconv.Atoi(matches[1])
				}
			}

			ep := types.EpisodeList{
				EpisodeNumber: sc.int32Ptr(int32(epNum)),
				EpisodeUrl:    sc.stringPtr(sc.makeAbsoluteURL(href)),
			}
			episodes = append(episodes, ep)
		})

		if len(episodes) > 0 {
			seasonList = append(seasonList, types.SeasonList{
				CurrentSeason: sc.int32Ptr(int32(currentSeasonNum)),
				TotalSeason:   sc.int32Ptr(int32(totalSeasons)),
				EpisodeList:   &episodes,
			})
		}

		return seasonList, seasonName, status
	}

	var seasonNums []int
	maxSeason := 0
	for seasonNum := range episodesBySeason {
		seasonNums = append(seasonNums, seasonNum)
		if seasonNum > maxSeason {
			maxSeason = seasonNum
		}
	}
	sort.Ints(seasonNums)

	totalSeasons := int32(maxSeason)

	if len(seasonNums) > 0 {
		seasonNameStr := fmt.Sprintf("Season %d", seasonNums[len(seasonNums)-1])
		seasonName = &seasonNameStr
	}

	for _, seasonNum := range seasonNums {
		episodeMap := episodesBySeason[seasonNum]

		var episodes []types.EpisodeList
		var episodeNums []int
		for epNum := range episodeMap {
			episodeNums = append(episodeNums, epNum)
		}
		sort.Ints(episodeNums)

		for _, epNum := range episodeNums {
			ep := types.EpisodeList{
				EpisodeNumber: sc.int32Ptr(int32(epNum)),
				EpisodeUrl:    sc.stringPtr(episodeMap[epNum]),
			}
			episodes = append(episodes, ep)
		}

		season := types.SeasonList{
			CurrentSeason: sc.int32Ptr(int32(seasonNum)),
			TotalSeason:   sc.int32Ptr(totalSeasons),
			EpisodeList:   &episodes,
		}
		seasonList = append(seasonList, season)
	}

	return seasonList, seasonName, status
}
func (sc *SeriesClient) GetEpisode(pathname string) (*types.SeriesEpisode, error) {
	var htmlContent string

	cleanPathname := sc.makeCleanPathname(pathname)
	initialURL := fmt.Sprintf("%s%s", SeriesBaseURL, cleanPathname)

	actions := []chromedp.Action{
		chromedp.Navigate(initialURL),
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

	episode := sc.scrapeEpisode(doc)
	episode.EpisodeUrl = &initialURL

	return episode, nil
}
func (sc *SeriesClient) scrapeEpisode(doc *goquery.Document) *types.SeriesEpisode {
	episode := &types.SeriesEpisode{}

	movieInfo := doc.Find(".movie-info")
	if movieInfo.Length() > 0 {
		infoTag := doc.Find(".info-tag")
		if infoTag.Length() > 0 {
			var qualities []string

			infoTag.Find("span").Each(func(i int, span *goquery.Selection) {
				text := strings.TrimSpace(span.Text())
				if text == "" {
					return
				}

				qualityKeywords := []string{"BluRay", "WEBDL", "HDRip", "4K", "1080p", "720p", "480p", "CAM", "TS", "HDTC"}
				for _, keyword := range qualityKeywords {
					if strings.Contains(text, keyword) {
						qualities = append(qualities, text)
						return
					}
				}
			})

			if len(qualities) > 0 {
				qualityStr := strings.Join(qualities, ", ")
				episode.LabelQuality = &qualityStr
			}

		}

		title := movieInfo.Find("h1").Text()
		if title != "" {
			reSes := regexp.MustCompile(`(?i)Season\s+(\d+)`)
			if matchesSes := reSes.FindStringSubmatch(title); len(matchesSes) > 1 {
				if num, err := strconv.Atoi(matchesSes[1]); err == nil {
					episode.SeasonNumber = sc.int32Ptr(int32(num))
				}
			}

			reEps := regexp.MustCompile(`(?i)Episode\s+(\d+)`)
			if matchesEps := reEps.FindStringSubmatch(title); len(matchesEps) > 1 {
				if num, err := strconv.Atoi(matchesEps[1]); err == nil {
					episode.EpisodeNumber = sc.int32Ptr(int32(num))
				}
			}

		}

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
			URL:  sc.stringPtr(sc.makeAbsoluteURL(href)),
			Type: sc.stringPtr(strings.ToUpper(serverType)),
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
			if p.URL != nil && *p.URL == sc.makeAbsoluteURL(value) {
				exists = true
				break
			}
		}

		if !exists {
			playerURLs = append(playerURLs, types.PlayerUrl{
				URL:  sc.stringPtr(sc.makeAbsoluteURL(value)),
				Type: sc.stringPtr(strings.ToUpper(serverType)),
			})
		}
	})

	if len(playerURLs) > 0 {
		episode.PlayerUrl = &playerURLs
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
				episode.DownloadUrl = sc.stringPtr(href)
				break
			}
		}
	}

	var playerPaginations []types.PlayerPagination
	items := doc.Find(".player-action .action-right li")
	items.Each(func(i int, li *goquery.Selection) {
		a := li.Find("a")
		if a.Length() > 0 {
			href, _ := a.Attr("href")

			aClone := a.Clone()
			aClone.Find("i").Remove()
			linkText := strings.TrimSpace(aClone.Text())

			episodeName := sc.parseEpisodeNumber(linkText, href)

			playPaginate := types.PlayerPagination{
				Name:    sc.stringPtr(episodeName),
				PageUrl: sc.stringPtr(sc.makeAbsoluteSlugURL(href)),
			}
			playerPaginations = append(playerPaginations, playPaginate)
		} else {
			liText := strings.TrimSpace(li.Text())

			playPaginate := types.PlayerPagination{
				Name:    sc.stringPtr(liText),
				PageUrl: nil,
			}
			playerPaginations = append(playerPaginations, playPaginate)
		}
	})
	sort.Slice(playerPaginations, func(i, j int) bool {
		re := regexp.MustCompile(`(\d+)`)
		numI := sc.extractNumber(*playerPaginations[i].Name, re)
		numJ := sc.extractNumber(*playerPaginations[j].Name, re)
		return numI < numJ
	})

	if len(playerPaginations) > 0 {
		episode.Pagination = &playerPaginations
	}

	return episode
}

// Series Search
func (sc *SeriesClient) Search(query string, page int) (*types.MovieListResponse, error) {
	var url string
	if page > 1 {
		url = fmt.Sprintf("%ssearch?s=%s&page=%d", SeriesBaseURL, query, page)
	} else {
		url = fmt.Sprintf("%ssearch?s=%s", SeriesBaseURL, query)
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

// Series Latest
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
func (sc *SeriesClient) parseSimilarSeries(doc *goquery.Document) []types.Movie {
	var similarMovies []types.Movie

	doc.Find(".mob-related-series .sliders li.slider").Each(func(i int, li *goquery.Selection) {
		a := li.Find("a")
		if a.Length() == 0 {
			return
		}

		href, exists := a.Attr("href")
		if !exists {
			return
		}
		originalPageURL := SeriesBaseURL + sc.makeCleanPathname(href)

		var thumbnail string
		img := li.Find("img")
		if img.Length() > 0 {
			if srcset, exists := img.Attr("srcset"); exists && srcset != "" {
				parts := strings.Fields(srcset)
				if len(parts) > 0 {
					thumbnail = parts[0]
				}
			}

			if thumbnail == "" {
				if src, exists := img.Attr("src"); exists {
					thumbnail = src
				}
			}
		}

		var title string
		titleText := strings.TrimSpace(li.Find(".poster-title").Text())
		if titleText != "" {
			title = titleText
		}

		var rating float64
		rawRating := strings.TrimSpace(li.Find(".rating").Text())
		re := regexp.MustCompile(`[0-9.]+`)
		match := re.FindString(rawRating)
		if match != "" {
			if val, err := strconv.ParseFloat(match, 64); err == nil {
				rating = val
			}
		}

		var year int32
		yearText := strings.TrimSpace(li.Find(".year").Text())
		if yearText != "" {
			if y, err := strconv.ParseInt(yearText, 10, 32); err == nil {
				year = int32(y)
			}
		}

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

		var genre string
		genreText := strings.TrimSpace(li.Find(".genre").Text())
		if genreText != "" {
			genre = genreText
		}

		var labelQuality string
		episodeSpan := li.Find(".episode")

		if episodeSpan.Length() > 0 {
			prefix := ""
			episodeSpan.Contents().Each(func(i int, s *goquery.Selection) {
				if goquery.NodeName(s) == "#text" {
					prefix = strings.TrimSpace(s.Text())
				}
			})

			episodeNum := strings.TrimSpace(episodeSpan.Find("strong").Text())

			if prefix != "" && episodeNum != "" {
				labelQuality = fmt.Sprintf("%s %s", prefix, episodeNum)
			} else {
				labelQuality = strings.TrimSpace(episodeSpan.Text())
			}
		}

		movie := types.Movie{
			ID:              uuid.New(),
			Title:           title,
			Thumbnail:       sc.stringPtr(sc.makeAbsoluteURL(thumbnail)),
			Year:            &year,
			Genre:           &genre,
			Rating:          &rating,
			LabelQuality:    &labelQuality,
			OriginalPageUrl: sc.stringPtr(originalPageURL),
		}

		if movie.Title != "" {
			similarMovies = append(similarMovies, movie)
		}
	})

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
					Thumbnail:       sc.stringPtr(sc.makeAbsoluteURL(thumbnail)),
					Year:            &year,
					OriginalPageUrl: sc.stringPtr(sc.makeAbsoluteURL(href)),
				})
			}
		})
	}

	return similarMovies
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
func (sc *SeriesClient) parseDuration(durationStr string) int {
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
func (sc *SeriesClient) parseEpisodeNumber(text, href string) string {
	re := regexp.MustCompile(`EPS?\s*(\d+)`)
	if matches := re.FindStringSubmatch(text); len(matches) > 1 {
		return "Episode " + matches[1]
	}

	re = regexp.MustCompile(`episode-(\d+)`)
	if matches := re.FindStringSubmatch(href); len(matches) > 1 {
		return "Episode " + matches[1]
	}

	return text
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
func (sc *SeriesClient) makeAbsoluteSlugURL(slug string) string {
	if slug == "" {
		return ""
	}

	var rawSlug string
	if strings.HasPrefix(slug, "http") {
		u, err := url.Parse(slug)
		if err != nil {
			rawSlug = slug
		} else {
			rawSlug = u.Path
		}
	} else {
		rawSlug = slug
	}

	cleanSlug := sc.makeCleanPathname(rawSlug)

	return SeriesBaseURL + cleanSlug
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
	re := regexp.MustCompile(`^/+|/+$`)
	return re.ReplaceAllString(pathname, "")
}
func (sc *SeriesClient) looksLikeMoviesPage(doc *goquery.Document) bool {
	if doc.Find("#player-select option").Length() > 0 {
		return true
	}
	if doc.Find("#player-list li a").Length() > 0 {
		return true
	}

	return false
}
func (sc *SeriesClient) isMoviesByFinalURL(url string) bool {
	seriesPatterns := []string{
		"lk21official",
		"tv8.",
	}

	urlLower := strings.ToLower(url)
	for _, pattern := range seriesPatterns {
		if strings.Contains(urlLower, pattern) {
			return true
		}
	}
	return false
}
func (sc *SeriesClient) isValidURL(urlStr string) bool {
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
func (sc *SeriesClient) clickIfExist(selector string, isXpath bool) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		var nodes []*cdp.Node
		searchType := chromedp.ByQuery
		if isXpath {
			searchType = chromedp.BySearch
		}

		subCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		chromedp.Nodes(selector, &nodes, searchType).Do(subCtx)
		if len(nodes) > 0 {
			return chromedp.Click(selector, searchType).Do(ctx)
		}
		return nil
	})
}
func (sc *SeriesClient) extractNumber(s string, re *regexp.Regexp) int {
	if matches := re.FindStringSubmatch(s); len(matches) > 1 {
		if num, err := strconv.Atoi(matches[1]); err == nil {
			return num
		}
	}
	return 0
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
