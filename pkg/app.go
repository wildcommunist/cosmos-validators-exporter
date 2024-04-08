package pkg

import (
	"fmt"
	coingeckoPkg "main/pkg/price_fetchers/coingecko"
	dexScreenerPkg "main/pkg/price_fetchers/dex_screener"
	"main/pkg/types"
	"net/http"
	"sync"
	"time"

	"main/pkg/config"
	loggerPkg "main/pkg/logger"
	queriersPkg "main/pkg/queriers"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
)

type (
	AppPayload struct {
		Version string `json:"version"`
		Commit  string `json:"commit"`
		Hash    string `json:"hash"`
	}

	App struct {
		Payload  *AppPayload
		Config   *config.Config
		Logger   *zerolog.Logger
		Queriers []types.Querier
	}
)

func NewApp(configPath string, payload AppPayload) *App {
	appConfig, err := config.GetConfig(configPath)
	if err != nil {
		loggerPkg.GetDefaultLogger().Fatal().Err(err).Msg("Could not load config")
	}

	if err = appConfig.Validate(); err != nil {
		loggerPkg.GetDefaultLogger().Fatal().Err(err).Msg("Provided config is invalid!")
	}

	logger := loggerPkg.GetLogger(appConfig.LogConfig)
	appConfig.DisplayWarnings(logger)

	coingecko := coingeckoPkg.NewCoingecko(appConfig, logger)
	dexScreener := dexScreenerPkg.NewDexScreener(logger)

	queriers := []types.Querier{
		queriersPkg.NewCommissionQuerier(logger, appConfig),
		queriersPkg.NewDelegationsQuerier(logger, appConfig),
		queriersPkg.NewUnbondsQuerier(logger, appConfig),
		queriersPkg.NewSelfDelegationsQuerier(logger, appConfig),
		queriersPkg.NewPriceQuerier(logger, appConfig, coingecko, dexScreener),
		queriersPkg.NewRewardsQuerier(logger, appConfig),
		queriersPkg.NewWalletQuerier(logger, appConfig),
		queriersPkg.NewSlashingParamsQuerier(logger, appConfig),
		queriersPkg.NewValidatorQuerier(logger, appConfig),
		queriersPkg.NewDenomCoefficientsQuerier(logger, appConfig),
		queriersPkg.NewSigningInfoQuerier(logger, appConfig),
		queriersPkg.NewUptimeQuerier(),
	}

	return &App{
		Payload:  &payload,
		Logger:   logger,
		Config:   appConfig,
		Queriers: queriers,
	}
}

func (a *App) Start() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		a.BaseHandler(w, r)
	})

	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		a.Handler(w, r)
	})

	a.Logger.Info().Str("addr", a.Config.ListenAddress).Msg("Listening")
	err := http.ListenAndServe(a.Config.ListenAddress, nil)
	if err != nil {
		a.Logger.Fatal().Err(err).Msg("Could not start application")
	}
}

func (a *App) BaseHandler(w http.ResponseWriter, r *http.Request) {
	sublogger := a.Logger.With().
		Str("request-id", uuid.New().String()).
		Logger()
	sublogger.Info().Msg("Serving homepage.")

	var chains []string
	for _, c := range a.Config.Chains {
		chains = append(chains, fmt.Sprintf("%s:%s", c.Name, c.BaseDenom))
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf(
		"Build version: %s\nCommit: %s\nHash: %s\n\n"+
			"Number of chains: %d (%v)\n\n"+
			"<a href=\"/metrics\">View metrics</a>",
		a.Payload.Version,
		a.Payload.Commit,
		a.Payload.Hash,
		len(chains),
		chains,
	)))
}

func (a *App) Handler(w http.ResponseWriter, r *http.Request) {
	requestStart := time.Now()

	sublogger := a.Logger.With().
		Str("request-id", uuid.New().String()).
		Logger()

	registry := prometheus.NewRegistry()

	var wg sync.WaitGroup
	var mutex sync.Mutex

	var queryInfos []*types.QueryInfo

	for _, querierExt := range a.Queriers {
		wg.Add(1)

		go func(querier types.Querier) {
			defer wg.Done()
			collectors, querierQueryInfos := querier.GetMetrics()

			mutex.Lock()
			defer mutex.Unlock()

			for _, collector := range collectors {
				registry.MustRegister(collector)
			}

			queryInfos = append(queryInfos, querierQueryInfos...)
		}(querierExt)
	}

	wg.Wait()

	queriesQuerier := queriersPkg.NewQueriesQuerier(a.Config, queryInfos)
	queriesMetrics, _ := queriesQuerier.GetMetrics()

	for _, metric := range queriesMetrics {
		registry.MustRegister(metric)
	}

	h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	h.ServeHTTP(w, r)

	sublogger.Info().
		Str("method", http.MethodGet).
		Str("endpoint", "/metrics").
		Float64("request-time", time.Since(requestStart).Seconds()).
		Msg("Request processed")
}
