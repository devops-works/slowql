package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"time"
)

// results is the datastrcucture that will be saved on disk
type results struct {
	File          string        `json:"file"`
	Date          time.Time     `json:"date"`
	TotalDuration time.Duration `json:"total_duration"`
	Hash          string        `json:"hash"`
	ServerMeta    serverMeta    `json:"server_meta"`
	Data          []statistics  `json:"data"`
}

// findCache looks a for a cache file stored in the same directory than the slow
// query log. It returns true if the cache file exists, false otherwise
func findCache(f string) bool {
	if _, err := os.Stat(f + ".cache"); os.IsNotExist(err) {
		return false
	}
	return true
}

// restoreCache reads the cache and returns its contents if the SHA-256 of the
// file and the one stored in the cache match
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

	hash, err := getSha256(f)
	if err != nil {
		return r, err
	}

	if hash != r.Hash {
		return r, errors.New("hashes does not match, log file must have changed since cache creation")
	}
	return r, nil
}

// saveCache saves a cache in the same directory than the slow query log
func saveCache(r results) error {
	var err error
	r.Hash, err = getSha256(r.File)
	if err != nil {
		return err
	}

	file, err := json.Marshal(r)
	if err != nil {
		return err
	}

	if err = ioutil.WriteFile(r.File+".cache", file, 0644); err != nil {
		return err
	}

	return nil
}

// getSha256 returns the hash of a file
func getSha256(f string) (string, error) {
	fd, err := os.Open(f)
	if err != nil {
		return "", err
	}
	defer fd.Close()

	h := sha256.New()
	if _, err := io.Copy(h, fd); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
