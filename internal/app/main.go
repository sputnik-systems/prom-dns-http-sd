package app

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/sputnik-systems/prom-dns-http-sd/pkg/storage"
	"github.com/sputnik-systems/prom-dns-http-sd/pkg/storage/yandexcloud"
)

type SDConfig struct {
	Targets []string          `json:"targets"`
	Labels  map[string]string `json:"labels,omitempty"`
}

type SDConfigs []SDConfig

var mu sync.Mutex
var config *storage.Config
var client storage.Client
var responseData map[string]SDConfigs

var configFilePath = flag.String("config-path", "", "application config file path")
var ycAuthJsonFilePath = flag.String("yc-auth-json-file-path", "", "Yandex.Cloud iam.json file path")
var dataUpdateInterval = flag.String("data-update-interval", "5m", "Interval between targets data updating")

func Run() error {
	flag.Parse()

	ctx := context.Background()

	interval, err := time.ParseDuration(*dataUpdateInterval)
	if err != nil {
		return errors.New("incorrect duration format")
	}

	go responseDataUpdateTicker(ctx, interval)

	// fsnotify does not support Mac OS right now
	if runtime.GOOS != "darwin" {
		go configFileUpdater(ctx)
	}

	http.HandleFunc("/", giveResponse)

	return http.ListenAndServe(":8080", nil)
}

func giveResponse(w http.ResponseWriter, r *http.Request) {
	if body, ok := responseData[r.URL.Path]; ok {
		resp, err := json.Marshal(body)
		if err != nil {
			w.WriteHeader(404)
			return
		}

		fmt.Fprintf(w, string(resp))
		return
	}

	w.WriteHeader(404)
}

func configFileUpdater(ctx context.Context) {
	var err error

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	err = watcher.Add(*configFilePath)
	if err != nil {
		log.Fatal(err)
	}

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			log.Println("event:", event)
			if event.Has(fsnotify.Write) {
				log.Println("modified file:", event.Name)

				config, err = storage.GetConfig(*configFilePath)
				if err != nil {
					log.Printf("failed to get config: %s", err)
				}

				client, err = yandexcloud.NewClient(ctx, "./iam.json", config)
				if err != nil {
					log.Printf("failed to initialize client: %s", err)
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Println("error:", err)
		}
	}

}

func responseDataUpdater(ctx context.Context) {
	zones, err := client.ListZones(ctx, config.Zones...)
	if err != nil {
		log.Printf("failed to list zones: %s", err)
	}

	sds := make(map[string]SDConfigs)
	for _, rule := range config.Rules {
		if _, ok := sds[rule.Path]; !ok {
			sds[rule.Path] = make(SDConfigs, 0)
		}

		for _, zone := range zones {
			records, err := zone.ListRecords(ctx, rule.Filters...)
			if err != nil {
				log.Printf("failed to list records: %s", err)
			}

			targets := make([]string, 0)
			for _, record := range records {
				targets = append(targets, fmt.Sprintf("%s:%d", record.GetName(), rule.Port))
			}

			sds[rule.Path] = append(sds[rule.Path], SDConfig{
				Targets: targets,
				Labels:  rule.Labels,
			})
		}
	}

	mu.Lock()
	responseData = sds
	mu.Unlock()
}

func responseDataUpdateTicker(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var err error
	config, err = storage.GetConfig(*configFilePath)
	if err != nil {
		log.Printf("failed to get config: %s", err)
	}

	client, err = yandexcloud.NewClient(ctx, *ycAuthJsonFilePath, config)
	if err != nil {
		log.Printf("failed to initialize client: %s", err)
	}

	responseDataUpdater(ctx)

	for {
		select {
		case <-ticker.C:
			responseDataUpdater(ctx)
		}
	}
}
