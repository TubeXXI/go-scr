package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"tubexxi/scraper/pkg/logger"
	"tubexxi/scraper/pkg/scraper"
	"tubexxi/scraper/pkg/utils"

	"github.com/AlecAivazis/survey/v2"
	"github.com/gosimple/slug"
	"go.uber.org/zap"
)

const (
	MovieScraper  = "Movie Scraper"
	SeriesScraper = "Series Scraper"
	AnimeScraper  = "Anime Scraper"
	ClearResults  = "Clear Results"
	Exit          = "Exit"

	Home      = "Home"
	Country   = "Country"
	Year      = "Year"
	Genre     = "Genre"
	Search    = "Search"
	Featured  = "Featured"
	Quality   = "Quality"
	Director  = "Director"
	Artist    = "Artist"
	Special   = "Special"
	Detail    = "Detail"
	Episode   = "Episode"
	Latest    = "Latest"
	Ongoing   = "Ongoing"
	Completed = "Completed"
	GenreList = "Genre List"
)

func main() {
	// Initialize logger
	_, err := logger.InitJSONLogger()
	if err != nil {
		fmt.Printf("Warning: Failed to initialize logger: %v\n", err)
	}

	printBanner()

	scraper := selectScraper()
	fmt.Printf("You selected: %s\n", scraper)

	var method string

	switch scraper {
	case MovieScraper:
		method = selectMovieMethod()
		runMovieScraper(method)
	case SeriesScraper:
		method = selectSeriesMethod()
		runSeriesScraper(method)
	// case AnimeScraper:
	// 	method = selectAnimeMethod()
	case ClearResults:
		fmt.Println("Clearing results directory...")
		resultsDir := "results"
		if err := utils.ClearAllFilesInDir(resultsDir); err != nil {
			fmt.Printf("Error clearing results directory: %v\n", err)
		} else {
			fmt.Println("Results directory cleared successfully")
		}
	case Exit:
		fmt.Println("Exiting application. Goodbye!")
		os.Exit(0)
	default:
		fmt.Println("Invalid selection")
		logger.Logger.Warn("Invalid scraper selection", zap.String("selection", scraper))
	}

	logger.CloseLogger()
}

func printBanner() {
	fmt.Println("\n╔════════════════════════════════════════════════════╗")
	fmt.Println("║                                                      ║")
	fmt.Println("║      🎬 TUBEXXI SCRAPER CLI TOOL v1.0 🎬            ║")
	fmt.Println("║                                                      ║")
	fmt.Println("║          Powered by Golang + Chromdp + Goquery       ║")
	fmt.Println("║          ⭐ Now with Movies, Series and Anime!      ║")
	fmt.Println("║                                                      ║")
	fmt.Println("╚══════════════════════════════════════════════════════╝")
	fmt.Println()
}

func selectScraper() string {
	var scraper string

	scraperPrompt := &survey.Select{
		Message: "🎯 Select scrapper type",
		Options: []string{
			MovieScraper,
			SeriesScraper,
			AnimeScraper,
			ClearResults,
			Exit,
		},
		PageSize: 10,
	}

	err := survey.AskOne(scraperPrompt, &scraper)
	if err != nil {
		logger.Logger.Fatal("Error selecting scrapper type", zap.Error(err))
	}

	return scraper
}
func selectMovieMethod() string {
	var method string

	methodPrompt := &survey.Select{
		Message: "🎯 Select movie scraping method",
		Options: []string{
			Home,
			Country,
			Year,
			Genre,
			Search,
			Featured,
			Special,
			Quality,
			Director,
			Artist,
			Detail,
		},
		PageSize: 10,
	}

	if err := survey.AskOne(methodPrompt, &method); err != nil {
		logger.Logger.Fatal("Error selecting movie scraping method", zap.Error(err))
	}

	return method

}
func selectSeriesMethod() string {
	var method string

	methodPrompt := &survey.Select{
		Message: "🎯 Select series scraping method",
		Options: []string{
			Home,
			Country,
			Year,
			Genre,
			Search,
			Featured,
			Special,
			Quality,
			Director,
			Artist,
			Detail,
			Episode,
		},
		PageSize: 10,
	}

	if err := survey.AskOne(methodPrompt, &method); err != nil {
		logger.Logger.Fatal("Error selecting series scraping method", zap.Error(err))
	}

	return method

}
func selectAnimeMethod() string {
	var method string

	methodPrompt := &survey.Select{
		Message: "🎯 Select anime scraping method",
		Options: []string{
			Home,
			Latest,
			Search,
			Ongoing,
			Completed,
			GenreList,
			Genre,
			Detail,
			Episode,
		},
		PageSize: 10,
	}

	if err := survey.AskOne(methodPrompt, &method); err != nil {
		logger.Logger.Fatal("Error selecting anime scraping method", zap.Error(err))
	}

	return method

}
func runMovieScraper(method string) {
	client := scraper.NewMovieClient(true)
	defer client.Close()

	resultDir := "../results/movies"
	err := utils.EnsureDir(resultDir)
	if err != nil {
		logger.Logger.Fatal("Error creating results directory", zap.Error(err))
	}

	fmt.Printf("Running movie scraper with method: %s\n", method)

	switch method {
	case Home:
		fmt.Println("Scraping home page...")

		data, err := client.GetHome()
		if err != nil {
			logger.Logger.Error("Error getting home data", zap.Error(err))
		} else if data != nil {
			// timeMill := time.Now().UnixMilli()
			// filename := fmt.Sprintf("home_%d.json", timeMill)
			filename := "home.json"
			fullPath := filepath.Join(resultDir, filename)
			if err := saveToFile(fullPath, data); err != nil {
				logger.Logger.Error("Error saving home data", zap.Error(err))
			}
		}
	case Country:
		countryPrompt := &survey.Input{
			Message: "Enter country (e.g., USA, UK, India):",
			Default: "USA",
		}
		country := ""
		if err := survey.AskOne(countryPrompt, &country); err != nil {
			logger.Logger.Fatal("Error getting country input", zap.Error(err))
		}

		fmt.Println("Scraping by country...")
		currentPage := 1

		data, err := client.GetMovieList("/country/"+country, currentPage)

		if err != nil {
			logger.Logger.Error("Error getting country data", zap.Error(err))
		} else if data != nil {
			filename := fmt.Sprintf("country_%s_%d.json", country, currentPage)
			fullPath := filepath.Join(resultDir, filename)
			if err := saveToFile(fullPath, data); err != nil {
				logger.Logger.Error("Error saving country data", zap.Error(err))
			}
		}
	case Year:
		yearPrompt := &survey.Input{
			Message: "Enter year (e.g., 2020):",
			Default: "2020",
		}
		year := 0
		if err := survey.AskOne(yearPrompt, &year); err != nil {
			logger.Logger.Fatal("Error getting year input", zap.Error(err))
		}

		fmt.Println("Scraping by year...")

		currentPage := 1

		data, err := client.GetMovieList("/year/"+strconv.Itoa(year), currentPage)

		if err != nil {
			logger.Logger.Error("Error getting year data", zap.Error(err))
		} else if data != nil {
			filename := fmt.Sprintf("year_%d_%d.json", year, currentPage)
			fullPath := filepath.Join(resultDir, filename)
			if err := saveToFile(fullPath, data); err != nil {
				logger.Logger.Error("Error saving year data", zap.Error(err))
			}
		}
	case Genre:
		genrePrompt := &survey.Input{
			Message: "Enter genre (e.g., action, comedy, horror):",
			Default: "action",
		}
		genre := ""
		if err := survey.AskOne(genrePrompt, &genre); err != nil {
			logger.Logger.Fatal("Error getting genre input", zap.Error(err))
		}

		fmt.Println("Scraping by genre...")

		currentPage := 1

		data, err := client.GetMovieList("/genre/"+genre, currentPage)

		if err != nil {
			logger.Logger.Error("Error getting genre data", zap.Error(err))
		} else if data != nil {
			filename := fmt.Sprintf("genre_%s_%d.json", genre, currentPage)
			fullPath := filepath.Join(resultDir, filename)
			if err := saveToFile(fullPath, data); err != nil {
				logger.Logger.Error("Error saving genre data", zap.Error(err))
			}
		}
	case Search:
		queryProimpt := &survey.Input{
			Message: "Enter search query: eg. 'Batman'",
			Default: "Batman",
		}

		query := ""
		if err := survey.AskOne(queryProimpt, &query); err != nil {
			logger.Logger.Fatal("Error getting search query", zap.Error(err))
		}

		fmt.Println("Scraping by search...")
		currentPage := 1

		data, err := client.Search(query, currentPage)

		if err != nil {
			logger.Logger.Error("Error getting search data", zap.Error(err))
		} else if data != nil {
			filename := fmt.Sprintf("search_%s_%d.json", query, currentPage)
			fullPath := filepath.Join(resultDir, filename)
			if err := saveToFile(fullPath, data); err != nil {
				logger.Logger.Error("Error saving search data", zap.Error(err))
			}
		}
	case Featured:
		featurePrompt := &survey.Input{
			Message: "Enter feature pathname (e.g., latest, popular, top-movie-today):",
			Default: "latest",
		}
		feature := ""
		if err := survey.AskOne(featurePrompt, &feature); err != nil {
			logger.Logger.Fatal("Error getting feature input", zap.Error(err))
		}

		fmt.Println("Scraping by feature...")

		currentPage := 1

		data, err := client.GetMovieList("/"+feature, currentPage)

		if err != nil {
			logger.Logger.Error("Error getting feature data", zap.Error(err))
		} else if data != nil {
			filename := fmt.Sprintf("feature_%s_%d.json", feature, currentPage)
			fullPath := filepath.Join(resultDir, filename)
			if err := saveToFile(fullPath, data); err != nil {
				logger.Logger.Error("Error saving genre data", zap.Error(err))
			}
		}
	case Quality:
		qualityPrompt := &survey.Input{
			Message: "Enter quality pathname (e.g., hd, hd-1080p, 4k):",
			Default: "hd",
		}
		quality := ""
		if err := survey.AskOne(qualityPrompt, &quality); err != nil {
			logger.Logger.Fatal("Error getting quality input", zap.Error(err))
		}

		fmt.Println("Scraping by quality...")

		currentPage := 1

		data, err := client.GetMovieList("/quality/"+quality, currentPage)

		if err != nil {
			logger.Logger.Error("Error getting quality data", zap.Error(err))
		} else if data != nil {
			filename := fmt.Sprintf("quality_%s_%d.json", quality, currentPage)
			fullPath := filepath.Join(resultDir, filename)
			if err := saveToFile(fullPath, data); err != nil {
				logger.Logger.Error("Error saving quality data", zap.Error(err))
			}
		}
	case Director:
		directorPrompt := &survey.Input{
			Message: "Enter director name (e.g., Christopher Nolan, Martin Scorsese):",
			Default: "Christopher Nolan",
		}
		director := ""
		if err := survey.AskOne(directorPrompt, &director); err != nil {
			logger.Logger.Fatal("Error getting director input", zap.Error(err))
		}

		fmt.Println("Scraping by director...")

		currentPage := 1

		directorToPathname := slug.MakeLang(director, "en")
		data, err := client.GetMovieList("/director/"+directorToPathname, currentPage)

		if err != nil {
			logger.Logger.Error("Error getting director data", zap.Error(err))
		} else if data != nil {
			filename := fmt.Sprintf("director_%s_%d.json", director, currentPage)
			fullPath := filepath.Join(resultDir, filename)
			if err := saveToFile(fullPath, data); err != nil {
				logger.Logger.Error("Error saving director data", zap.Error(err))
			}
		}
	case Artist:
		artistPrompt := &survey.Input{
			Message: "Enter artist name (e.g., John Smith, Jane Doe):",
			Default: "John Smith",
		}
		artist := ""
		if err := survey.AskOne(artistPrompt, &artist); err != nil {
			logger.Logger.Fatal("Error getting artist input", zap.Error(err))
		}

		fmt.Println("Scraping by artist...")

		currentPage := 1

		artistToPathname := slug.MakeLang(artist, "en")
		data, err := client.GetMovieList("/artist/"+artistToPathname, currentPage)

		if err != nil {
			logger.Logger.Error("Error getting artist data", zap.Error(err))
		} else if data != nil {
			filename := fmt.Sprintf("artist_%s_%d.json", artist, currentPage)
			fullPath := filepath.Join(resultDir, filename)
			if err := saveToFile(fullPath, data); err != nil {
				logger.Logger.Error("Error saving artist data", zap.Error(err))
			}
		}
	case Detail:
		pathPrompt := &survey.Input{
			Message: "Enter movies path (e.g., /batman-v-superman-dawn-justice-extended-2016):",
			Default: "/batman-v-superman-dawn-justice-extended-2016",
		}
		path := ""
		if err := survey.AskOne(pathPrompt, &path); err != nil {
			logger.Logger.Fatal("Error getting movie path input", zap.Error(err))
		}

		data, err := client.GetMovieDetail(path)
		if err != nil {
			logger.Logger.Error("Error getting movie detail", zap.Error(err))
		} else if data != nil {
			filename := fmt.Sprintf("detail_%s.json", strings.ReplaceAll(strings.ReplaceAll(path, "/", "_"), "-", "_"))
			fullPath := filepath.Join(resultDir, filename)
			if err := saveToFile(fullPath, data); err != nil {
				logger.Logger.Error("Error saving movie detail", zap.Error(err))
			}
		}
	default:
		logger.Logger.Info("Method not implemented yet", zap.String("method", method))
	}
}
func runSeriesScraper(method string) {
	client := scraper.NewSeriesClient(true)
	defer client.Close()

	resultDir := "../results/series"
	err := utils.EnsureDir(resultDir)
	if err != nil {
		logger.Logger.Fatal("Error creating results directory", zap.Error(err))
	}

	fmt.Printf("Running series scraper with method: %s\n", method)

	switch method {
	case Home:
		fmt.Println("Scraping home page...")

		data, err := client.GetHome()

		if err != nil {
			logger.Logger.Error("Error getting home data", zap.Error(err))
		} else if data != nil {
			filename := "home.json"
			fullPath := filepath.Join(resultDir, filename)
			if err := saveToFile(fullPath, data); err != nil {
				logger.Logger.Error("Error saving home data", zap.Error(err))
			}
		}

	case Country:
		countryPrompt := &survey.Input{
			Message: "Enter country (e.g., USA, UK, India):",
			Default: "USA",
		}
		country := ""
		if err := survey.AskOne(countryPrompt, &country); err != nil {
			logger.Logger.Fatal("Error getting country input", zap.Error(err))
		}

		fmt.Println("Scraping by country...")
		currentPage := 1

		data, err := client.GetSeriesList("/country/"+country, currentPage)

		if err != nil {
			logger.Logger.Error("Error getting country data", zap.Error(err))
		} else if data != nil {
			filename := fmt.Sprintf("country_%s_%d.json", country, currentPage)
			fullPath := filepath.Join(resultDir, filename)
			if err := saveToFile(fullPath, data); err != nil {
				logger.Logger.Error("Error saving country data", zap.Error(err))
			}
		}
	case Year:
		yearPrompt := &survey.Input{
			Message: "Enter year (e.g., 2020):",
			Default: "2020",
		}
		year := 0
		if err := survey.AskOne(yearPrompt, &year); err != nil {
			logger.Logger.Fatal("Error getting year input", zap.Error(err))
		}

		fmt.Println("Scraping by year...")

		currentPage := 1

		data, err := client.GetSeriesList("/year/"+strconv.Itoa(year), currentPage)

		if err != nil {
			logger.Logger.Error("Error getting year data", zap.Error(err))
		} else if data != nil {
			filename := fmt.Sprintf("year_%d_%d.json", year, currentPage)
			fullPath := filepath.Join(resultDir, filename)
			if err := saveToFile(fullPath, data); err != nil {
				logger.Logger.Error("Error saving year data", zap.Error(err))
			}
		}
	case Genre:
		genrePrompt := &survey.Input{
			Message: "Enter genre (e.g., action, comedy, horror):",
			Default: "action",
		}
		genre := ""
		if err := survey.AskOne(genrePrompt, &genre); err != nil {
			logger.Logger.Fatal("Error getting genre input", zap.Error(err))
		}

		fmt.Println("Scraping by genre...")

		currentPage := 1

		data, err := client.GetSeriesList("/genre/"+genre, currentPage)

		if err != nil {
			logger.Logger.Error("Error getting genre data", zap.Error(err))
		} else if data != nil {
			filename := fmt.Sprintf("genre_%s_%d.json", genre, currentPage)
			fullPath := filepath.Join(resultDir, filename)
			if err := saveToFile(fullPath, data); err != nil {
				logger.Logger.Error("Error saving genre data", zap.Error(err))
			}
		}
	case Search:
		queryProimpt := &survey.Input{
			Message: "Enter search query: eg. 'Batman'",
			Default: "Batman",
		}

		query := ""
		if err := survey.AskOne(queryProimpt, &query); err != nil {
			logger.Logger.Fatal("Error getting search query", zap.Error(err))
		}

		fmt.Println("Scraping by search...")
		currentPage := 1

		data, err := client.Search(query, currentPage)

		if err != nil {
			logger.Logger.Error("Error getting search data", zap.Error(err))
		} else if data != nil {
			filename := fmt.Sprintf("search_%s_%d.json", query, currentPage)
			fullPath := filepath.Join(resultDir, filename)
			if err := saveToFile(fullPath, data); err != nil {
				logger.Logger.Error("Error saving search data", zap.Error(err))
			}
		}
	case Featured:
		featurePrompt := &survey.Input{
			Message: "Enter feature pathname (e.g., latest, popular, top-movie-today):",
			Default: "latest",
		}
		feature := ""
		if err := survey.AskOne(featurePrompt, &feature); err != nil {
			logger.Logger.Fatal("Error getting feature input", zap.Error(err))
		}

		fmt.Println("Scraping by feature...")

		currentPage := 1

		data, err := client.GetSeriesList("/"+feature, currentPage)

		if err != nil {
			logger.Logger.Error("Error getting feature data", zap.Error(err))
		} else if data != nil {
			filename := fmt.Sprintf("feature_%s_%d.json", feature, currentPage)
			fullPath := filepath.Join(resultDir, filename)
			if err := saveToFile(fullPath, data); err != nil {
				logger.Logger.Error("Error saving genre data", zap.Error(err))
			}
		}
	case Quality:
		qualityPrompt := &survey.Input{
			Message: "Enter quality pathname (e.g., hd, hd-1080p, 4k):",
			Default: "hd",
		}
		quality := ""
		if err := survey.AskOne(qualityPrompt, &quality); err != nil {
			logger.Logger.Fatal("Error getting quality input", zap.Error(err))
		}

		fmt.Println("Scraping by quality...")

		currentPage := 1

		data, err := client.GetSeriesList("/quality/"+quality, currentPage)

		if err != nil {
			logger.Logger.Error("Error getting quality data", zap.Error(err))
		} else if data != nil {
			filename := fmt.Sprintf("quality_%s_%d.json", quality, currentPage)
			fullPath := filepath.Join(resultDir, filename)
			if err := saveToFile(fullPath, data); err != nil {
				logger.Logger.Error("Error saving quality data", zap.Error(err))
			}
		}
	case Director:
		directorPrompt := &survey.Input{
			Message: "Enter director name (e.g., Christopher Nolan, Martin Scorsese):",
			Default: "Christopher Nolan",
		}
		director := ""
		if err := survey.AskOne(directorPrompt, &director); err != nil {
			logger.Logger.Fatal("Error getting director input", zap.Error(err))
		}

		fmt.Println("Scraping by director...")

		currentPage := 1

		directorToPathname := slug.MakeLang(director, "en")
		data, err := client.GetSeriesList("/director/"+directorToPathname, currentPage)

		if err != nil {
			logger.Logger.Error("Error getting director data", zap.Error(err))
		} else if data != nil {
			filename := fmt.Sprintf("director_%s_%d.json", director, currentPage)
			fullPath := filepath.Join(resultDir, filename)
			if err := saveToFile(fullPath, data); err != nil {
				logger.Logger.Error("Error saving director data", zap.Error(err))
			}
		}
	case Artist:
		artistPrompt := &survey.Input{
			Message: "Enter artist name (e.g., John Smith, Jane Doe):",
			Default: "John Smith",
		}
		artist := ""
		if err := survey.AskOne(artistPrompt, &artist); err != nil {
			logger.Logger.Fatal("Error getting artist input", zap.Error(err))
		}

		fmt.Println("Scraping by artist...")

		currentPage := 1

		artistToPathname := slug.MakeLang(artist, "en")
		data, err := client.GetSeriesList("/artist/"+artistToPathname, currentPage)

		if err != nil {
			logger.Logger.Error("Error getting artist data", zap.Error(err))
		} else if data != nil {
			filename := fmt.Sprintf("artist_%s_%d.json", artist, currentPage)
			fullPath := filepath.Join(resultDir, filename)
			if err := saveToFile(fullPath, data); err != nil {
				logger.Logger.Error("Error saving artist data", zap.Error(err))
			}
		}
	case Detail:
		pathPrompt := &survey.Input{
			Message: "Enter series path (e.g., /taxi-driver-mobeomtaeksi-2021):",
			Default: "/taxi-driver-mobeomtaeksi-2021",
		}
		path := ""
		if err := survey.AskOne(pathPrompt, &path); err != nil {
			logger.Logger.Fatal("Error getting series path input", zap.Error(err))
		}

		data, err := client.GetSeriesDetail(path)
		if err != nil {
			logger.Logger.Error("Error getting series detail", zap.Error(err))
		} else if data != nil {
			filename := fmt.Sprintf("detail_%s.json", strings.ReplaceAll(strings.ReplaceAll(path, "/", "_"), "-", "_"))
			fullPath := filepath.Join(resultDir, filename)
			if err := saveToFile(fullPath, data); err != nil {
				logger.Logger.Error("Error saving series detail", zap.Error(err))
			}
		}
	case Episode:
		pathPrompt := &survey.Input{
			Message: "Enter series episode path (e.g., /taxi-driver-mobeomtaeksi-season-3-episode-1-2021):",
			Default: "/taxi-driver-mobeomtaeksi-season-3-episode-1-2021",
		}
		path := ""
		if err := survey.AskOne(pathPrompt, &path); err != nil {
			logger.Logger.Fatal("Error getting series episode path input", zap.Error(err))
		}

		data, err := client.GetEpisode(path)
		if err != nil {
			logger.Logger.Error("Error getting series episode detail", zap.Error(err))
		} else if data != nil {
			filename := fmt.Sprintf("detail_%s.json", strings.ReplaceAll(strings.ReplaceAll(path, "/", "_"), "-", "_"))
			fullPath := filepath.Join(resultDir, filename)
			if err := saveToFile(fullPath, data); err != nil {
				logger.Logger.Error("Error saving series episode detail", zap.Error(err))
			}
		}
	default:
		logger.Logger.Info("Method not implemented yet", zap.String("method", method))
	}
}

func saveToFile(path string, info interface{}) error {
	// Buat directory jika belum ada
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("error creating file: %v", err)
	}
	defer file.Close()

	// Encode info ke JSON dengan indentasi
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(info); err != nil {
		return fmt.Errorf("error encoding JSON: %v", err)
	}

	fmt.Printf("Data saved to %s\n", path)

	return nil
}
