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
	ycsdk "github.com/yandex-cloud/go-sdk"
	"github.com/yandex-cloud/go-sdk/iamkey"

	"github.com/sputnik-systems/prom-dns-http-sd/pkg/storage"
	"github.com/sputnik-systems/prom-dns-http-sd/pkg/storage/yandexcloud"
)

type SDConfig struct {
	Targets []string          `json:"targets"`
	Labels  map[string]string `json:"labels,omitempty"`
}

type SDConfigs []SDConfig

type params struct {
	mu           sync.Mutex
	config       *storage.Config
	client       storage.Client
	responseData map[string]SDConfigs

	flags flags
}

type flags struct {
	configFilePath, ycAuthJsonFilePath, dataUpdateInterval *string
}

var p = params{
	flags: flags{
		configFilePath:     flag.String("config-path", "", "application config file path"),
		ycAuthJsonFilePath: flag.String("yc-auth-json-file-path", "", "Yandex.Cloud iam.json file path"),
		dataUpdateInterval: flag.String("data-update-interval", "1h", "Interval between targets data updating"),
	},
}

func Run() error {
	flag.Parse()

	ctx := context.Background()

	interval, err := time.ParseDuration(*p.flags.dataUpdateInterval)
	if err != nil {
		return errors.New("incorrect duration format")
	}

	go responseDataUpdateTicker(ctx, interval)

	// fsnotify does not support Mac OS right now
	if runtime.GOOS != "darwin" {
		go configFileUpdater(ctx)
	}

	http.HandleFunc("/healthz", healthCheck)
	http.HandleFunc("/", giveResponse)

	return http.ListenAndServe(":8080", nil)
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
}

func giveResponse(w http.ResponseWriter, r *http.Request) {
	if body, ok := p.responseData[r.URL.Path]; ok {
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

	err = watcher.Add(*p.flags.configFilePath)
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

				if err := updateConfigAndClient(ctx); err != nil {
					log.Printf("failed to update config and client: %s", err)
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
	zones, err := p.client.ListZones(ctx, p.config.Zones...)
	if err != nil {
		log.Printf("failed to list zones: %s", err)
	}

	sds := make(map[string]SDConfigs)
	for _, rule := range p.config.Rules {
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

	p.mu.Lock()
	p.responseData = sds
	p.mu.Unlock()
}

func responseDataUpdateTicker(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	if err := updateConfigAndClient(ctx); err != nil {
		log.Printf("failed to update config and client: %s", err)
	}

	responseDataUpdater(ctx)

	for {
		select {
		case <-ticker.C:
			responseDataUpdater(ctx)
		}
	}
}

func updateConfigAndClient(ctx context.Context) error {
	var err error

	p.mu.Lock()

	p.config, err = storage.GetConfig(*p.flags.configFilePath)
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	var creds ycsdk.Credentials
	switch {
	case *p.flags.ycAuthJsonFilePath != "":
		key, err := iamkey.ReadFromJSONFile(*p.flags.ycAuthJsonFilePath)
		if err != nil {
			return err
		}

		if creds, err = ycsdk.ServiceAccountKey(key); err != nil {
			return err
		}
	default:
		creds = ycsdk.InstanceServiceAccount()
	}

	sdk, err := ycsdk.Build(ctx, ycsdk.Config{
		Credentials: creds,
	})
	if err != nil {
		return fmt.Errorf("failed to build yc sdk: %w", err)
	}

	p.client, err = yandexcloud.NewClient(ctx, sdk, p.config)
	if err != nil {
		return fmt.Errorf("failed to initialize client: %w", err)
	}

	p.mu.Unlock()

	return nil
}
