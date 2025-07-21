package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/imroc/req/v3"
	"github.com/spf13/viper"
)

type Scraper interface {
	Run(context.Context)
}

type scraper struct {
	logger *slog.Logger
	wg     *sync.WaitGroup
}

func NewScraper(logger *slog.Logger, wg *sync.WaitGroup) Scraper {
	return scraper{
		logger: logger,
		wg:     wg,
	}
}

func (s scraper) Run(ctx context.Context) {
	defer s.wg.Done()

	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		s.logger.Error("Error loading location")
		return
	}

	s.scrape()
	s.logger.Info("Initial scraping finished")

	ticker := time.NewTicker(viper.GetDuration("INTERVAL_MINUTES") * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if s.shouldScrape(loc) {
				s.logger.Debug("Scraping started")
				s.scrape()
				s.logger.Debug("Scraping finished")
			}
		case <-ctx.Done():
			s.logger.Info("Scraper stopped")
			return
		}
	}
}

func (s scraper) shouldScrape(loc *time.Location) bool {
	now := time.Now().In(loc)

	return now.Weekday() >= time.Weekday(viper.GetInt("START_DAY")) &&
		now.Weekday() <= time.Weekday(viper.GetInt("END_DAY")) &&
		now.Hour() >= viper.GetInt("START_HOUR") &&
		now.Hour() < viper.GetInt("END_HOUR")
}

func (s scraper) scrape() {
	fakeChrome := req.ImpersonateChrome()
	res := fakeChrome.Get(viper.GetString("URL_TESOURO")).Do()

	if res.Response == nil {
		s.logger.Error("Failed to fetch data", "error", res.Err)
		return
	}

	if res.Response.StatusCode != 200 {
		s.logger.Error("Failed to fetch data", "status code", res.Response.StatusCode)
		return
	}

	defer res.Body.Close()
	data, err := io.ReadAll(res.Body)
	if err != nil {
		s.logger.Error("Failed to read response body", "error", err.Error())
		return
	}

	postReq, err := http.NewRequest(http.MethodPost, viper.GetString("URL_API")+"/bonds", strings.NewReader(string(data)))
	if err != nil {
		s.logger.Error("Failed to create HTTP request", "error", err.Error())
		return
	}

	postReq.Header.Set("Content-Type", "application/json")
	postReq.Header.Set("Authorization", "Bearer "+viper.GetString("API_KEY"))

	postRes, err := http.DefaultClient.Do(postReq)
	if err != nil {
		s.logger.Error("Failed to send data", "error", err.Error())
		return
	}

	defer postRes.Body.Close()
	if postRes.StatusCode != http.StatusCreated {
		s.logger.Error("Failed to send data", "status code", postRes.StatusCode)
		return
	}
}
