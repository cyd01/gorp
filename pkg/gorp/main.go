package gorp

import (
	"context"
	"flag"
	"log"
	"os"
	"time"

	"github.com/cyd01/gorp/pkg/config"
	"github.com/cyd01/gorp/pkg/server"
)

func Main() {
	configFile := flag.String("config", initFromEnv("", "CONFIG", "GORP_CONFIG", "PROXY_CONFIG"), "configuration file")
	configDir := flag.String("config-dir", "", "directory containing YAML configuration fragments")
	flag.Parse()

	if *configFile == "" && *configDir == "" {
		log.Fatal("missing -config or -config-dir flag")
	}
	if *configFile != "" && *configDir != "" {
		log.Fatal("use either -config or -config-dir, not both")
	}
	if *configDir != "" {
		StartDir(*configDir)
		return
	}
	Start(*configFile)
}

func Start(configFile string) {
	cfg, err := config.Load(configFile)
	if err != nil {
		log.Fatal(err)
	}
	startWithConfig(configFile, cfg)
}

func StartDir(configDir string) {
	cfg, err := config.LoadDir(configDir)
	if err != nil {
		log.Fatal(err)
	}
	startWithConfig(configDir, cfg)
}

func startWithConfig(configSource string, cfg *config.Config) {
	srv, err := server.Build(cfg)
	if err != nil {
		log.Fatal(err)
	}

	reload := make(chan struct{}, 1)
	go config.Watch(configSource, func() {
		select {
		case reload <- struct{}{}:
		default:
		}
	})

	for {
		runResult := make(chan error, 1)
		go func() {
			runResult <- srv.Run()
		}()

		select {
		case err := <-runResult:
			if err != nil {
				log.Fatal(err)
			}
			return

		case <-reload:
			log.Println("configuration reloading")
			var cfg2 *config.Config
			if info, err := os.Stat(configSource); err == nil && info.IsDir() {
				cfg2, err = config.LoadDir(configSource)
			} else {
				cfg2, err = config.Load(configSource)
			}
			if err != nil {
				log.Printf("failed to reload configuration: %v\n", err)
				continue
			}
			next, err := server.Build(cfg2)
			if err != nil {
				log.Printf("failed to build reloaded configuration: %v\n", err)
				continue
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			srv.Shutdown(ctx)
			cancel()
			if err := <-runResult; err != nil {
				log.Printf("server shutdown during reload: %v\n", err)
			}
			cfg = cfg2
			srv = next
		}
	}
}

func initFromEnv(def string, args ...string) string {
	for _, v := range args {
		if len(os.Getenv(v)) > 0 {
			return os.Getenv(v)
		}
	}
	return def
}
