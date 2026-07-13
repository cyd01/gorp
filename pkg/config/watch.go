package config

import (
	"log"
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
)

func Watch(file string, reload func()) error {
	info, err := os.Stat(file)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return WatchDir(file, reload)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	err = watcher.Add(file)
	if err != nil {
		return err
	}
	go func() {
		for {
			select {
			case event := <-watcher.Events:
				if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Remove) {
					log.Println("config file change detected, reloading...")
					reload()
				}
			case err := <-watcher.Errors:
				log.Println(err)
			}
		}
	}()
	return nil
}

func WatchDir(dir string, reload func()) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	if err := watcher.Add(dir); err != nil {
		return err
	}
	go func() {
		for {
			select {
			case event := <-watcher.Events:
				if filepath.Ext(event.Name) != ".yaml" && filepath.Ext(event.Name) != ".yml" {
					continue
				}
				if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
					log.Println("config directory change detected, reloading...")
					reload()
				}
			case err := <-watcher.Errors:
				log.Println(err)
			}
		}
	}()
	return nil
}
