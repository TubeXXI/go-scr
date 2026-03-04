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
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type AnimeClient struct {
	chromeClientStealth *ChromeClientStealth
	hTTPClient          *http.Client
}

func NewAnimeClient(useProxy bool) *AnimeClient {
	return &AnimeClient{
		chromeClientStealth: NewChromeClientStealth(useProxy),
		hTTPClient:          &http.Client{Timeout: 30 * time.Second},
	}
}

func (ac *AnimeClient) Close() {
	if ac.chromeClientStealth != nil {
		ac.chromeClientStealth.Close()
	}
}

func (ac *AnimeClient) GetLatest(page int) (*types.ScrapeResult, error) {
	var url string
	if page > 1 {
		url = fmt.Sprintf("%spage/%d/?post_type=anime", AnimeBaseURL, page)
	} else {
		url = fmt.Sprintf("%s?post_type=anime", AnimeBaseURL)
	}

	fmt.Printf("Scraping anime latest from URL: %s and page: %d\n", url, page)

	htmlContent, err := ac.chromeClientStealth.cb.Execute(func() (string, error) {
		return ac.chromeClientStealth.NavigateWithRetry(url, 3*time.Second, 3)
	})

	if err != nil {
		logger.Logger.Error("Error loading anime latest", zap.Error(err))
		return nil, err
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
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
	pagination.TotalItems = int64(pagination.TotalPages) * int64(len(animes))

	return &types.ScrapeResult{
		Animes:     animes,
		Pagination: pagination,
	}, nil
}
func (ac *AnimeClient) GetOngoing(page int) (*types.ScrapeResult, error) {
	var url string
	if page > 1 {
		url = fmt.Sprintf("%songoing-anime/page/%d/", AnimeBaseURL, page)

	} else {
		url = fmt.Sprintf("%songoing-anime/", AnimeBaseURL)
	}

	fmt.Printf("Scraping anime ongoing from URL: %s and page: %d\n", url, page)

	htmlContent, err := ac.chromeClientStealth.cb.Execute(func() (string, error) {
		return ac.chromeClientStealth.NavigateWithRetry(url, 3*time.Second, 3)
	})

	if err != nil {
		logger.Logger.Error("Error fetching ongoing anime", zap.Error(err))
		return nil, err
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
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
	pagination.TotalItems = int64(pagination.TotalPages) * int64(len(animes))

	return &types.ScrapeResult{
		Animes:     animes,
		Pagination: pagination,
	}, nil
}
func (ac *AnimeClient) GetComplete(page int) (*types.ScrapeResult, error) {
	var url string
	if page > 1 {
		url = fmt.Sprintf("%scomplete-anime/page/%d/", AnimeBaseURL, page)
	} else {
		url = fmt.Sprintf("%scomplete-anime/", AnimeBaseURL)
	}

	fmt.Printf("Scraping anime complete from URL: %s and page: %d\n", url, page)

	htmlContent, err := ac.chromeClientStealth.cb.Execute(func() (string, error) {
		return ac.chromeClientStealth.NavigateWithRetry(url, 3*time.Second, 3)
	})

	if err != nil {
		logger.Logger.Error("Error fetching complete anime", zap.Error(err))
		return nil, err
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
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
	pagination.TotalItems = int64(pagination.TotalPages) * int64(len(animes))

	return &types.ScrapeResult{
		Animes:     animes,
		Pagination: pagination,
	}, nil
}
func (ac *AnimeClient) Search(query string, page int) (*types.ScrapeResult, error) {
	var url string
	if page > 1 {
		url = fmt.Sprintf("%s?s=%s&post_type=anime&page=%d", AnimeBaseURL, query, page)
	} else {
		url = fmt.Sprintf("%s?s=%s&post_type=anime", AnimeBaseURL, query)
	}

	htmlContent, err := ac.chromeClientStealth.cb.Execute(func() (string, error) {
		return ac.chromeClientStealth.NavigateWithRetry(url, 3*time.Second, 3)
	})

	if err != nil {
		logger.Logger.Error("Error searching anime", zap.Error(err))
		return nil, err
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
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
	pagination.TotalItems = int64(pagination.TotalPages) * int64(len(animes))

	return &types.ScrapeResult{
		Animes:     animes,
		Pagination: pagination,
		Query:      &query,
	}, nil
}
func (ac *AnimeClient) GetGenres() ([]types.AnimeGenre, error) {
	url := fmt.Sprintf("%sgenre-list/", AnimeBaseURL)

	htmlContent, err := ac.chromeClientStealth.cb.Execute(func() (string, error) {
		return ac.chromeClientStealth.NavigateWithRetry(url, 3*time.Second, 3)
	})
	if err != nil {
		logger.Logger.Error("Error fetching genres", zap.Error(err))
		return nil, err
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
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
		absURL := ac.makeAbsoluteSlugURL(href)

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
func (ac *AnimeClient) GetByGenre(name string, page int) (*types.ScrapeResult, error) {
	var url string
	if page > 1 {
		url = fmt.Sprintf("%sgenres/%s/page/%d/", AnimeBaseURL, name, page)
	} else {
		url = fmt.Sprintf("%sgenres/%s", AnimeBaseURL, name)
	}

	fmt.Printf("Scraping series latest from URL: %s and page: %d\n", url, page)

	htmlContent, err := ac.chromeClientStealth.cb.Execute(func() (string, error) {
		return ac.chromeClientStealth.NavigateWithRetry(url, 3*time.Second, 3)
	})

	if err != nil {
		logger.Logger.Error("Error fetching complete anime", zap.Error(err))
		return nil, err
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		logger.Logger.Error("Error parsing complete anime HTML", zap.Error(err))
		return nil, err
	}

	animes := ac.extractByGenreAnimes(doc)
	pagination := ac.parsePaginationByGenre(doc, page)
	pagination.PerPage = len(animes)
	if pagination.PerPage == 0 {
		pagination.PerPage = 20
	}
	pagination.TotalItems = int64(pagination.TotalPages) * int64(len(animes))

	return &types.ScrapeResult{
		Animes:     animes,
		Pagination: pagination,
		Query:      &name,
	}, nil
}
func (ac *AnimeClient) GetAnimeDetail(pathname string) (*types.Anime, error) {
	cleanPathname := ac.makeCleanPathname(pathname)
	initialURL := fmt.Sprintf("%sanime/%s", AnimeBaseURL, cleanPathname)

	fmt.Printf("Scraping anime details from URL: %s\n", initialURL)

	htmlContent, err := ac.chromeClientStealth.cb.Execute(func() (string, error) {
		return ac.chromeClientStealth.NavigateWithRetry(initialURL, 3*time.Second, 3)
	})
	if err != nil {
		logger.Logger.Error("Error fetching anime detail", zap.Error(err))
		return nil, err
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		logger.Logger.Error("Error parsing anime detail HTML", zap.Error(err))
		return nil, err
	}

	return ac.scrapeAnimeDetail(doc, initialURL), nil
}
func (ac *AnimeClient) GetAnimeEpisode(pathname string) (*types.Episode, error) {
	cleanPathname := ac.makeCleanPathname(pathname)
	initialURL := fmt.Sprintf("%sepisode/%s", AnimeBaseURL, cleanPathname)

	fmt.Printf("Scraping anime episode detail from URL: %s\n", initialURL)

	htmlContent, err := ac.chromeClientStealth.cb.Execute(func() (string, error) {
		return ac.chromeClientStealth.NavigateWithRetry(initialURL, 3*time.Second, 3)
	})
	if err != nil {
		logger.Logger.Error("Error fetching anime episode detail", zap.Error(err))
		return nil, err
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		logger.Logger.Error("Error parsing anime episode detail HTML", zap.Error(err))
		return nil, err
	}

	return ac.scrapeEpisodeDetail(doc, initialURL), nil
}

// Extracts anime from archive pages
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
	id := uuid.New()
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

// Extracts by ongoing anime
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
	id := uuid.New()
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

// Extracts by complete anime
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

// extracts By genre name anime
func (ac *AnimeClient) extractByGenreAnimes(doc *goquery.Document) []types.Anime {
	var animes []types.Anime

	doc.Find("div.col-anime").Each(func(i int, li *goquery.Selection) {
		anime := ac.parseByGenreAnimeItem(li)
		if anime.Title != nil && *anime.Title != "" {
			animes = append(animes, anime)
		}
	})

	return animes
}
func (ac *AnimeClient) parseByGenreAnimeItem(doc *goquery.Selection) types.Anime {
	id := uuid.New()
	anime := types.Anime{ID: id}

	colTitle := doc.Find(".col-anime-title")
	if colTitle.Length() > 0 {
		title := colTitle.Find("a").Text()
		if title != "" {
			anime.Title = &title
		}

		aTag := colTitle.Find("a[href]").First()
		if aTag.Length() > 0 {
			if href, ok := aTag.Attr("href"); ok {
				url := ac.makeAbsoluteURL(href)
				anime.OriginalPageURL = &url
			}
		}
	}

	studio := doc.Find(".col-anime-studio").Text()
	if studio != "" {
		anime.Studio = &studio
	}

	epz := doc.Find(".col-anime-eps")
	if epz.Length() > 0 {
		epText := strings.TrimSpace(epz.Text())

		re := regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*Eps|Unknown\s*Eps`)
		matches := re.FindStringSubmatch(epText)

		if len(matches) > 1 && matches[1] != "" {
			anime.TotalEpisodes = &matches[1]
		} else {
			anime.TotalEpisodes = &epText
		}
	}

	rating := doc.Find(".col-anime-rating").Text()
	if rating != "" {
		cleanRating := strings.TrimSpace(rating)
		if cleanRating != "" {
			anime.Rating = &cleanRating
		}
	}

	thumbDiv := doc.Find(".col-anime-cover")
	if thumbDiv.Length() > 0 {
		img := thumbDiv.Find("img")
		if img.Length() > 0 {
			if src, ok := img.Attr("src"); ok {
				anime.Thumbnail = &src
			}
		}
	}

	var genres []types.AnimeGenre
	doc.Find(".col-anime-genre a").Each(func(i int, g *goquery.Selection) {
		var genre types.AnimeGenre

		name := g.Text()
		if name != "" {
			genre.Name = &name
		}

		if href, ok := g.Attr("href"); ok {
			url := ac.makeAbsoluteURL(href)
			genre.URL = &url
		}

		genres = append(genres, genre)
	})

	releaseDate := doc.Find(".col-anime-date").Text()
	if releaseDate != "" {
		cleanDate := strings.TrimSpace(releaseDate)
		anime.ReleaseDate = &cleanDate
	}

	if len(genres) > 0 {
		anime.Genre = &genres
	}

	return anime
}
func (ac *AnimeClient) parsePaginationByGenre(doc *goquery.Document, currentPage int) types.PaginationAnime {
	pagination := types.PaginationAnime{
		CurrentPage: currentPage,
		TotalPages:  1,
		PerPage:     20,
	}

	paginationDiv := doc.Find("div.pagination")
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
		if text != "Next" && text != "Previous" && text != "Berikutnya" && text != "Sebelumnya" {
			if num, err := strconv.Atoi(text); err == nil {
				if num > maxPage {
					maxPage = num
				}
				pageNumbers = append(pageNumbers, num)
			}
		}
	})

	dots := pagenavix.Find("span.page-numbers.dots")
	if dots.Length() > 0 && maxPage == pagination.CurrentPage {
		lastLink := pagenavix.Find("a.page-numbers").Last()
		if lastLink.Length() > 0 {
			lastText := strings.TrimSpace(lastLink.Text())
			if num, err := strconv.Atoi(lastText); err == nil {
				maxPage = num
			}
		}
	}

	pagination.TotalPages = maxPage
	pagination.PageNumbers = pageNumbers

	nextLink := pagenavix.Find("a.next.page-numbers").First()
	if href, ok := nextLink.Attr("href"); ok && href != "" {
		pagination.HasNext = true
		nextURL := ac.makeAbsoluteURL(href)
		pagination.NextPageURL = &nextURL
	}

	prevLink := pagenavix.Find("a.prev.page-numbers").First()
	if href, ok := prevLink.Attr("href"); ok && href != "" {
		pagination.HasPrevious = true
		prevURL := ac.makeAbsoluteURL(href)
		pagination.PreviousPageURL = &prevURL
	}
	return pagination
}

// Extracts detailed information about an anime
func (ac *AnimeClient) scrapeAnimeDetail(doc *goquery.Document, url string) *types.Anime {
	id := uuid.New()
	anime := &types.Anime{
		ID:              id,
		OriginalPageURL: &url,
	}

	title := doc.Find("h1.entry-title, .venser h1").First().Text()
	if title == "" {
		title = doc.Find("title").Text()
		if strings.Contains(title, "Nonton Anime") {
			title = strings.ReplaceAll(title, "Nonton Anime", "")
			title = strings.ReplaceAll(title, "Sub Indo", "")
			title = strings.TrimSpace(title)
		}
	}
	if title != "" {
		anime.Title = &title
	}

	fotoAnime := doc.Find(".fotoanime")
	if fotoAnime.Length() > 0 {
		img := fotoAnime.Find("img").First()
		if src, ok := img.Attr("src"); ok && src != "" {
			anime.Thumbnail = &src
		} else if srcset, ok := img.Attr("srcset"); ok {
			parts := strings.Split(srcset, ",")
			if len(parts) > 0 {
				imgURL := strings.TrimSpace(strings.Split(parts[0], " ")[0])
				anime.Thumbnail = &imgURL
			}
		}

		synopsis := fotoAnime.Find(".sinopc").Text()
		if synopsis != "" {
			cleanSynopsis := strings.TrimSpace(synopsis)
			anime.Description = &cleanSynopsis
		}

		fotoAnime.Find(".infozin .infozingle p").Each(func(i int, p *goquery.Selection) {
			text := strings.TrimSpace(p.Text())

			parts := strings.SplitN(text, ":", 2)
			if len(parts) != 2 {
				return
			}

			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			value = ac.stripHTML(value)

			switch {
			case strings.Contains(key, "Judul") || strings.Contains(key, "Title"):
				if anime.Title == nil || *anime.Title == "" {
					anime.Title = &value
				}
			case strings.Contains(key, "Japanese"):
				anime.TitleJapanese = &value
			case strings.Contains(key, "Skor") || strings.Contains(key, "Score"):
				anime.Score = &value
			case strings.Contains(key, "Produser") || strings.Contains(key, "Producer"):
				anime.Producer = &value
			case strings.Contains(key, "Tipe") || strings.Contains(key, "Type"):
				anime.Type = &value
			case strings.Contains(key, "Status"):
				anime.Status = &value
			case strings.Contains(key, "Total Episode") || strings.Contains(key, "Episode"):
				anime.TotalEpisodes = &value
			case strings.Contains(key, "Durasi") || strings.Contains(key, "Duration"):
				anime.Duration = &value
				if minutes := ac.parseDurationToMinutes(value); minutes != nil {
					durationMinutes := fmt.Sprintf("%.0f", *minutes)
					anime.Duration = &durationMinutes
				}
			case strings.Contains(key, "Tanggal Rilis") || strings.Contains(key, "Release"):
				anime.ReleaseDate = &value
			case strings.Contains(key, "Studio"):
				anime.Studio = &value
			}
		})
	}

	var genres []types.AnimeGenre
	doc.Find(".infozin .infozingle p:contains('Genre') a").Each(func(i int, a *goquery.Selection) {
		name := strings.TrimSpace(a.Text())
		if name == "" {
			return
		}

		genre := types.AnimeGenre{
			Name: &name,
		}

		if href, ok := a.Attr("href"); ok && href != "" {
			url := ac.makeAbsoluteURL(href)
			genre.URL = &url
		}

		genres = append(genres, genre)
	})

	if len(genres) > 0 {
		anime.Genre = &genres
	}

	episodes := ac.extractEpisodes(doc)
	if len(episodes) > 0 {
		anime.Episodes = &episodes
		anime.TotalEpisodes = ac.stringPtr(strconv.Itoa(len(episodes)))
	}

	similarAnime := ac.extractSimilarAnime(doc)
	if len(similarAnime) > 0 {
		anime.SimilarAnime = &similarAnime
	}

	return anime
}
func (ac *AnimeClient) extractEpisodes(doc *goquery.Document) []types.Episode {
	var episodes []types.Episode

	doc.Find(".episodelist ul li").Each(func(i int, li *goquery.Selection) {
		parentSection := li.ParentsFiltered(".episodelist")
		sectionTitle := parentSection.Find(".monktit").Text()
		if strings.Contains(sectionTitle, "Batch") || strings.Contains(sectionTitle, "Lengkap") {
			return
		}

		link := li.Find("a").First()
		href, exists := link.Attr("href")
		if !exists || href == "" {
			return
		}

		title := strings.TrimSpace(link.Text())

		episodeNum := ac.extractEpisodeNumber(title, href)

		dateSpan := li.Find(".zeebr")
		releaseDate := strings.TrimSpace(dateSpan.Text())

		epID := uuid.New()
		episode := types.Episode{
			ID:            epID,
			Title:         &title,
			PageURL:       &href,
			EpisodeNumber: &episodeNum,
		}

		if releaseDate != "" {
			episode.ReleaseDate = &releaseDate
		}

		episodes = append(episodes, episode)
	})

	for i, j := 0, len(episodes)-1; i < j; i, j = i+1, j-1 {
		episodes[i], episodes[j] = episodes[j], episodes[i]
	}

	return episodes
}
func (ac *AnimeClient) extractEpisodeNumber(title, href string) string {
	re := regexp.MustCompile(`(?i)Episode\s*(\d+)`)
	if matches := re.FindStringSubmatch(title); len(matches) > 1 {
		return matches[1]
	}

	re = regexp.MustCompile(`episode-(\d+)`)
	if matches := re.FindStringSubmatch(href); len(matches) > 1 {
		return matches[1]
	}

	return "0"
}
func (ac *AnimeClient) extractSimilarAnime(doc *goquery.Document) []types.Anime {
	var similarAnimes []types.Anime

	doc.Find("#recommend-anime-series .isi-konten").Each(func(i int, konten *goquery.Selection) {
		animeID := uuid.New()
		anime := types.Anime{ID: animeID}

		link := konten.Find(".isi-anime a").First()
		if href, ok := link.Attr("href"); ok && href != "" {
			anime.OriginalPageURL = &href
		}

		title := konten.Find(".judul-anime a").Text()
		if title != "" {
			anime.Title = &title
		}

		img := konten.Find("img").First()
		if src, ok := img.Attr("src"); ok && src != "" {
			anime.Thumbnail = &src
		}

		similarAnimes = append(similarAnimes, anime)
	})

	return similarAnimes
}
func (ac *AnimeClient) stripHTML(s string) string {
	re := regexp.MustCompile(`<[^>]*>`)
	return strings.TrimSpace(re.ReplaceAllString(s, ""))
}

// Extracts detailed information about an episode anime
func (ac *AnimeClient) scrapeEpisodeDetail(doc *goquery.Document, url string) *types.Episode {
	id := uuid.New()
	episode := &types.Episode{
		ID:      id,
		PageURL: &url,
	}

	title := doc.Find("h1.posttl").Text()
	if title != "" {
		episode.Title = ac.stringPtr(strings.TrimSpace(title))

		episode.EpisodeNumber = ac.parseEpisodeNumberFromTitle(title)
	}

	doc.Find(".kategoz span").Each(func(i int, span *goquery.Selection) {
		text := strings.TrimSpace(span.Text())

		if strings.Contains(text, "Posted by") {
			postedBy := strings.ReplaceAll(text, "Posted by", "")
			episode.PostedBy = ac.stringPtr(strings.TrimSpace(postedBy))
		} else if strings.Contains(text, "Release on") {
			releaseTime := strings.ReplaceAll(text, "Release on", "")
			episode.ReleaseTime = ac.stringPtr(strings.TrimSpace(releaseTime))
		}
	})

	iframe := doc.Find("#embed_holder iframe").First()
	if src, ok := iframe.Attr("src"); ok && src != "" {
		episode.PlayerURL = &src
	}

	var playerPaginations []types.PlayerPagination
	doc.Find(".prevnext .flir a").Each(func(i int, a *goquery.Selection) {
		var paginate types.PlayerPagination
		actionName := a.Text()
		if actionName != "" {
			paginate.Name = &actionName
		}

		if href, ok := a.Attr("href"); ok && href != "" {
			paginate.PageUrl = &href
		}

		playerPaginations = append(playerPaginations, paginate)
	})

	if len(playerPaginations) > 0 {
		episode.Pagination = &playerPaginations
	}

	prevNext := doc.Find(".prevnext")
	if prevNext.Length() > 0 {

		prevLink := prevNext.Find(".flir a[title='Episode Sebelumnya']")
		if href, ok := prevLink.Attr("href"); ok && href != "" && href != "#" {
			episode.PreviousEpisodeURL = ac.stringPtr(ac.makeAbsoluteURL(href))
		}

		seeAllLink := prevNext.Find(".flir a:contains('See All Episodes')")
		if href, ok := seeAllLink.Attr("href"); ok && href != "" && href != "#" {
			episode.SeeAllEpisodesURL = ac.stringPtr(ac.makeAbsoluteURL(href))
		}
	}

	var listEpisodes []types.ListOfEpisode
	doc.Find("#selectcog option").Each(func(i int, opt *goquery.Selection) {
		if i == 0 {
			return
		}

		name := strings.TrimSpace(opt.Text())
		if value, ok := opt.Attr("value"); ok && value != "" && value != "0" {
			listEpisodes = append(listEpisodes, types.ListOfEpisode{
				Name:    &name,
				PageUrl: ac.stringPtr(ac.makeAbsoluteURL(value)),
			})
		}
	})

	if len(listEpisodes) > 0 {
		episode.ListEpisode = &listEpisodes
	}

	var downloadLinks []types.DownloadLink

	doc.Find(".download ul li").Each(func(i int, li *goquery.Selection) {
		strong := li.Find("strong").First()
		qualityFormat := strings.TrimSpace(strong.Text())

		parts := strings.Fields(qualityFormat)
		var format, quality string
		if len(parts) >= 2 {
			format = parts[0]
			quality = strings.Join(parts[1:], " ")
		} else {
			quality = qualityFormat
		}

		li.Find("a").Each(func(j int, a *goquery.Selection) {
			name := strings.TrimSpace(a.Text())
			href, exists := a.Attr("href")
			if !exists || href == "" || href == "#" {
				return
			}

			downloadLink := types.DownloadLink{
				Name:    &name,
				URL:     &href,
				Quality: &quality,
			}

			if format != "" {
				downloadLink.Format = &format
			}

			downloadLinks = append(downloadLinks, downloadLink)
		})

		size := li.Find("i").Text()
		if size != "" {
			size = strings.TrimSpace(size)
			for j := range downloadLinks {
				if j >= len(downloadLinks)-li.Find("a").Length() {
					downloadLinks[j].Size = &size
				}
			}
		}
	})

	if len(downloadLinks) > 0 {
		episode.DownloadLinks = &downloadLinks
	}

	releaseDate := doc.Find(".infozingle p:contains('Release') span").Text()
	if releaseDate != "" {
		cleanDate := strings.TrimSpace(releaseDate)
		episode.ReleaseDate = &cleanDate
	}

	return episode
}
func (ac *AnimeClient) parseEpisodeNumberFromTitle(title string) *string {
	re := regexp.MustCompile(`(?i)Episode\s*(\d+)`)
	if matches := re.FindStringSubmatch(title); len(matches) > 1 {
		return &matches[1]
	}
	return nil
}

// Helper functions
func (ac *AnimeClient) parseDurationToMinutes(duration string) *float64 {
	if duration == "" {
		return nil
	}

	// Bersihkan string
	duration = strings.TrimSpace(duration)
	duration = strings.ToLower(duration)

	// Pola-pola duration yang umum
	patterns := []struct {
		regex   *regexp.Regexp
		handler func([]string) float64
	}{
		// Format: "24 Menit", "24 min", "24m"
		{
			regex: regexp.MustCompile(`(\d+(?:\.\d+)?)\s*(?:menit|min|m)\b`),
			handler: func(matches []string) float64 {
				val, _ := strconv.ParseFloat(matches[1], 64)
				return val
			},
		},
		// Format: "1 jam 24 menit", "1h 24m"
		{
			regex: regexp.MustCompile(`(?:(\d+)\s*(?:jam|h))?\s*(?:(\d+)\s*(?:menit|min|m))?`),
			handler: func(matches []string) float64 {
				var total float64
				if matches[1] != "" {
					jam, _ := strconv.ParseFloat(matches[1], 64)
					total += jam * 60
				}
				if matches[2] != "" {
					menit, _ := strconv.ParseFloat(matches[2], 64)
					total += menit
				}
				return total
			},
		},
		// Format: "1:24" (jam:menit)
		{
			regex: regexp.MustCompile(`(\d+):(\d+)`),
			handler: func(matches []string) float64 {
				jam, _ := strconv.ParseFloat(matches[1], 64)
				menit, _ := strconv.ParseFloat(matches[2], 64)
				return (jam * 60) + menit
			},
		},
	}

	for _, pattern := range patterns {
		if matches := pattern.regex.FindStringSubmatch(duration); len(matches) > 0 {
			total := pattern.handler(matches)
			if total > 0 {
				return &total
			}
		}
	}

	return nil
}
func (ac *AnimeClient) formatMinutes(minutes float64) string {
	if minutes < 60 {
		return fmt.Sprintf("%.0f Menit", minutes)
	}

	jam := int(minutes) / 60
	sisaMenit := int(minutes) % 60

	if sisaMenit == 0 {
		return fmt.Sprintf("%d Jam", jam)
	}
	return fmt.Sprintf("%d Jam %d Menit", jam, sisaMenit)
}
func (ac *AnimeClient) makeAbsoluteURL(url string) string {
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

	return AnimeBaseURL + url
}
func (ac *AnimeClient) makeAbsoluteSlugURL(slug string) string {
	if slug == "" {
		return ""
	}

	cleanSlug := ac.makeCleanPathname(slug)

	return AnimeBaseURL + cleanSlug
}
func (ac *AnimeClient) makeCleanPathname(pathname string) string {
	re := regexp.MustCompile(`^/+|/+$`)
	return re.ReplaceAllString(pathname, "")
}
func (ac *AnimeClient) stringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
