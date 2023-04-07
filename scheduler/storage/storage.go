/*
 *     Copyright 2022 The Dragonfly Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

//go:generate mockgen -destination mocks/storage_mock.go -source storage.go -package mocks

package storage

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"
	"time"

	"github.com/gocarina/gocsv"

	logger "d7y.io/dragonfly/v2/internal/dflog"
	pkgio "d7y.io/dragonfly/v2/pkg/io"
)

const (
	// RecordFilePrefix is prefix of record file name.
	RecordFilePrefix = "record"

	// RecordFileExt is extension of record file name.
	RecordFileExt = "csv"
)

const (
	// megabyte is the converted factor of MaxSize and bytes.
	megabyte = 1024 * 1024

	// backupTimeFormat is the timestamp format of backup filename.
	backupTimeFormat = "2006-01-02T15-04-05.000"
)

// Storage is the interface used for storage.
type Storage interface {
	// Create inserts the record into csv file.
	Create(Download) error

	// List returns all of records in csv file.
	List() ([]Download, error)

	// Count returns the count of records.
	Count() int64

	// Open opens storage for read, it returns io.ReadCloser of storage files.
	Open() (io.ReadCloser, error)

	// Clear removes all record files.
	Clear() error
}

// storage provides storage function.
type storage struct {
	baseDir    string
	filename   string
	maxSize    int64
	maxBackups int
	buffer     []Download
	bufferSize int
	count      int64
	mu         *sync.RWMutex
}

// New returns a new Storage instence.
func New(baseDir string, maxSize, maxBackups, bufferSize int) (Storage, error) {
	s := &storage{
		baseDir:    baseDir,
		filename:   filepath.Join(baseDir, fmt.Sprintf("%s.%s", RecordFilePrefix, RecordFileExt)),
		maxSize:    int64(maxSize * megabyte),
		maxBackups: maxBackups,
		buffer:     make([]Download, 0, bufferSize),
		bufferSize: bufferSize,
		mu:         &sync.RWMutex{},
	}

	file, err := os.OpenFile(s.filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return nil, err
	}
	file.Close()

	return s, nil
}

// Create inserts the record into csv file.
func (s *storage) Create(record Download) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Write without buffer.
	if s.bufferSize == 0 {
		if err := s.create(s.buffer...); err != nil {
			return err
		}

		// Update record count.
		s.count++
		return nil
	}

	// Write records to file.
	if len(s.buffer) >= s.bufferSize {
		if err := s.create(s.buffer...); err != nil {
			return err
		}

		// Update record count.
		s.count += int64(s.bufferSize)

		// Keep allocated memory.
		s.buffer = s.buffer[:0]
	}

	// Write records to buffer.
	s.buffer = append(s.buffer, record)
	return nil
}

// List returns all of records in csv file.
func (s *storage) List() ([]Download, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	fileInfos, err := s.backups()
	if err != nil {
		return nil, err
	}

	var readers []io.Reader
	var readClosers []io.ReadCloser
	defer func() {
		for _, readCloser := range readClosers {
			if err := readCloser.Close(); err != nil {
				logger.Error(err)
			}
		}
	}()

	for _, fileInfo := range fileInfos {
		file, err := os.Open(filepath.Join(s.baseDir, fileInfo.Name()))
		if err != nil {
			return nil, err
		}

		readers = append(readers, file)
		readClosers = append(readClosers, file)
	}

	var records []Download
	if err := gocsv.UnmarshalWithoutHeaders(io.MultiReader(readers...), &records); err != nil {
		return nil, err
	}

	return records, nil
}

// Count returns the count of records.
func (s *storage) Count() int64 {
	return s.count
}

// Open opens storage for read, it returns io.ReadCloser of storage files.
func (s *storage) Open() (io.ReadCloser, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	fileInfos, err := s.backups()
	if err != nil {
		return nil, err
	}

	var readClosers []io.ReadCloser
	for _, fileInfo := range fileInfos {
		file, err := os.Open(filepath.Join(s.baseDir, fileInfo.Name()))
		if err != nil {
			return nil, err
		}

		readClosers = append(readClosers, file)
	}

	return pkgio.MultiReadCloser(readClosers...), nil
}

// Clear removes all records.
func (s *storage) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	fileInfos, err := s.backups()
	if err != nil {
		return err
	}

	for _, fileInfo := range fileInfos {
		filename := filepath.Join(s.baseDir, fileInfo.Name())
		if err := os.Remove(filename); err != nil {
			return err
		}
	}

	return nil
}

// create inserts the records into csv file.
func (s *storage) create(records ...Download) error {
	file, err := s.openFile()
	if err != nil {
		return err
	}
	defer file.Close()

	if err := gocsv.MarshalWithoutHeaders(records, file); err != nil {
		return err
	}

	return nil
}

// openFile opens the record file and removes record files that exceed the total size.
func (s *storage) openFile() (*os.File, error) {
	fileInfo, err := os.Stat(s.filename)
	if err != nil {
		return nil, err
	}

	if s.maxSize <= fileInfo.Size() {
		if err := os.Rename(s.filename, s.backupFilename()); err != nil {
			return nil, err
		}
	}

	fileInfos, err := s.backups()
	if err != nil {
		return nil, err
	}

	if s.maxBackups < len(fileInfos)+1 {
		filename := filepath.Join(s.baseDir, fileInfos[0].Name())
		if err := os.Remove(filename); err != nil {
			return nil, err
		}
	}

	file, err := os.OpenFile(s.filename, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		return nil, err
	}

	return file, nil
}

// backupFilename generates file name of backup files.
func (s *storage) backupFilename() string {
	timestamp := time.Now().Format(backupTimeFormat)
	return filepath.Join(s.baseDir, fmt.Sprintf("%s-%s.%s", RecordFilePrefix, timestamp, RecordFileExt))
}

// backupFilename returns backup file information.
func (s *storage) backups() ([]fs.FileInfo, error) {
	fileInfos, err := ioutil.ReadDir(s.baseDir)
	if err != nil {
		return nil, err
	}

	var backups []fs.FileInfo
	regexp := regexp.MustCompile(RecordFilePrefix)
	for _, fileInfo := range fileInfos {
		if !fileInfo.IsDir() && regexp.MatchString(fileInfo.Name()) {
			backups = append(backups, fileInfo)
		}
	}

	if len(backups) <= 0 {
		return nil, errors.New("backup does not exist")
	}

	sort.Slice(backups, func(i, j int) bool {
		return backups[i].ModTime().Before(backups[j].ModTime())
	})

	return backups, nil
}
