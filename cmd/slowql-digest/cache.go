package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"time"
)

// results is the datastrcucture that will be saved on disk
type results struct {
	File string       `json:"file"`
	Date time.Time    `json:"date"`
	Data []statistics `json:"data"`
}

// findCache looks a for a cache file stored in the same directory than the slow
// query log. It returns true if the cache file exists, false otherwise
func findCache(f string) bool {
	if _, err := os.Stat(f + ".cache"); os.IsNotExist(err) {
		return false
	}
	return true
}

// restoreCache reads the cache and returns its contents
func restoreCache(f string) (results, error) {
	var r results
	cache, err := os.Open(f + ".cache")
	if err != nil {
		return r, err
	}
	defer cache.Close()

	rawBytes, err := ioutil.ReadAll(cache)
	if err != nil {
		return r, err
	}

	if err := json.Unmarshal(rawBytes, &r); err != nil {
		return r, err
	}

	return r, nil
}

// saveCache saves a cache in the same directory than the slow query log
func saveCache(r results) error {
	file, err := json.Marshal(r)
	if err != nil {
		return err
	}

	if err = ioutil.WriteFile(r.File+".cache", file, 0644); err != nil {
		return err
	}

	return nil
}
