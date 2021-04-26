/*
 *     Copyright 2020 The Dragonfly Authors
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

package local

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"time"

	"d7y.io/dragonfly/v2/cdnsystem/cdnerrors"
	"d7y.io/dragonfly/v2/cdnsystem/storedriver"
	"d7y.io/dragonfly/v2/pkg/synclock"
	"d7y.io/dragonfly/v2/pkg/unit"
	"d7y.io/dragonfly/v2/pkg/util/fileutils"
	"d7y.io/dragonfly/v2/pkg/util/statutils"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

func init() {
	// Ensure that storage implements the StorageDriver interface
	var storage *diskStorage = nil
	var _ storedriver.Driver = storage
}

const StorageDriver = "disk"

const MemoryStorageDriver = "memory"

var fileLocker = synclock.NewLockerPool()

func init() {
	storedriver.Register(StorageDriver, NewStorage)
	storedriver.Register(MemoryStorageDriver, NewStorage)
}

// diskStorage is one of the implementations of StorageDriver using local disk file system.
type diskStorage struct {
	// BaseDir is the dir that local storage driver will store content based on it.
	BaseDir string
	// GcConfig
	GcConfig *storedriver.GcConfig
}

// NewStorage performs initialization for disk Storage and return a StorageDriver.
func NewStorage(conf interface{}) (storedriver.Driver, error) {
	cfg := &diskStorage{}
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook: decodeHock(
			reflect.TypeOf(time.Second),
			reflect.TypeOf(unit.B)),
		Result: cfg,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create decoder: %v", err)
	}
	err = decoder.Decode(conf)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %v", err)
	}
	// prepare the base dir
	if !filepath.IsAbs(cfg.BaseDir) {
		return nil, fmt.Errorf("not absolute path: %s", cfg.BaseDir)
	}
	if err := fileutils.MkdirAll(cfg.BaseDir); err != nil {
		return nil, fmt.Errorf("failed to create baseDir%s: %v", cfg.BaseDir, err)
	}

	return &diskStorage{
		BaseDir:  cfg.BaseDir,
		GcConfig: cfg.GcConfig,
	}, nil
}

func decodeHock(types ...reflect.Type) mapstructure.DecodeHookFunc {
	return func(f, t reflect.Type, data interface{}) (interface{}, error) {
		for _, typ := range types {
			if t == typ {
				b, _ := yaml.Marshal(data)
				v := reflect.New(t)
				return v.Interface(), yaml.Unmarshal(b, v.Interface())
			}
		}
		return data, nil
	}
}

func (ds *diskStorage) GetTotalSpace(ctx context.Context) (unit.Bytes, error) {
	path := ds.BaseDir
	lock(path, -1, true)
	defer unLock(path, -1, true)
	return fileutils.GetTotalSpace(path)
}

func (ds *diskStorage) GetHomePath(ctx context.Context) string {
	return ds.BaseDir
}

func (ds *diskStorage) GetGcConfig(ctx context.Context) *storedriver.GcConfig {
	return ds.GcConfig
}

func (ds *diskStorage) CreateBaseDir(ctx context.Context) error {
	return os.MkdirAll(ds.BaseDir, os.ModePerm)
}

func (ds *diskStorage) MoveFile(src string, dst string) error {
	return fileutils.MoveFile(src, dst)
}

// Get the content of key from storage and return in io stream.
func (ds *diskStorage) Get(ctx context.Context, raw *storedriver.Raw) (io.ReadCloser, error) {
	path, info, err := ds.statPath(raw.Bucket, raw.Key)
	if err != nil {
		return nil, err
	}

	if err := storedriver.CheckGetRaw(raw, info.Size()); err != nil {
		return nil, err
	}

	r, w := io.Pipe()
	go func(w *io.PipeWriter) {
		defer w.Close()

		lock(path, raw.Offset, true)
		defer unLock(path, raw.Offset, true)

		f, err := os.Open(path)
		if err != nil {
			return
		}
		defer f.Close()

		f.Seek(raw.Offset, io.SeekStart)
		var reader io.Reader
		reader = f
		if raw.Length > 0 {
			reader = io.LimitReader(f, raw.Length)
		}
		buf := make([]byte, 256*1024)
		io.CopyBuffer(w, reader, buf)
	}(w)
	return r, nil
}

// GetBytes gets the content of key from storage and return in bytes.
func (ds *diskStorage) GetBytes(ctx context.Context, raw *storedriver.Raw) (data []byte, err error) {
	path, info, err := ds.statPath(raw.Bucket, raw.Key)
	if err != nil {
		return nil, err
	}

	if err := storedriver.CheckGetRaw(raw, info.Size()); err != nil {
		return nil, err
	}

	lock(path, raw.Offset, true)
	defer unLock(path, raw.Offset, true)

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	f.Seek(raw.Offset, io.SeekStart)
	if raw.Length == 0 {
		data, err = ioutil.ReadAll(f)
	} else {
		data = make([]byte, raw.Length)
		_, err = f.Read(data)
	}
	if err != nil {
		return nil, err
	}
	return data, nil
}

// Put reads the content from reader and put it into storage.
func (ds *diskStorage) Put(ctx context.Context, raw *storedriver.Raw, data io.Reader) error {
	if err := storedriver.CheckPutRaw(raw); err != nil {
		return err
	}

	path, err := ds.preparePath(raw.Bucket, raw.Key)
	if err != nil {
		return err
	}

	if data == nil {
		return nil
	}

	lock(path, raw.Offset, false)
	defer unLock(path, raw.Offset, false)

	var f *os.File
	if raw.Trunc {
		if err = storedriver.CheckTrunc(raw); err != nil {
			return err
		}
		f, err = fileutils.OpenFile(path, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
	} else if raw.Append {
		f, err = fileutils.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	} else {
		f, err = fileutils.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0644)
	}
	if err != nil {
		return err
	}
	defer f.Close()
	if raw.Trunc {
		if err = f.Truncate(raw.TruncSize); err != nil {
			return err
		}
	}
	f.Seek(raw.Offset, io.SeekStart)
	if raw.Length > 0 {
		if _, err = io.CopyN(f, data, raw.Length); err != nil {
			return err
		}
		return nil
	}

	buf := make([]byte, 256*1024)
	if _, err = io.CopyBuffer(f, data, buf); err != nil {
		return err
	}

	return nil
}

// PutBytes puts the content of key from storage with bytes.
func (ds *diskStorage) PutBytes(ctx context.Context, raw *storedriver.Raw, data []byte) error {
	if err := storedriver.CheckPutRaw(raw); err != nil {
		return err
	}

	path, err := ds.preparePath(raw.Bucket, raw.Key)
	if err != nil {
		return err
	}

	lock(path, raw.Offset, false)
	defer unLock(path, raw.Offset, false)

	var f *os.File
	if raw.Trunc {
		f, err = fileutils.OpenFile(path, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
	} else if raw.Append {
		f, err = fileutils.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	} else {
		f, err = fileutils.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0644)
	}
	if err != nil {
		return err
	}
	defer f.Close()
	if raw.Trunc {
		if err = f.Truncate(raw.TruncSize); err != nil {
			return err
		}
	}
	f.Seek(raw.Offset, io.SeekStart)
	if raw.Length > 0 {
		if _, err := f.Write(data[:raw.Length]); err != nil {
			return err
		}
		return nil
	}
	if _, err := f.Write(data); err != nil {
		return err
	}
	return nil
}

//// AppendBytes append the content to end of storage file.
//func (ds *diskStorage) AppendBytes(ctx context.Context, raw *store.Raw, data []byte) error {
//	if err := store.CheckPutRaw(raw); err != nil {
//		return err
//	}
//
//	path, err := ds.preparePath(raw.Bucket, raw.Key)
//	if err != nil {
//		return err
//	}
//
//	lock(path, raw.Offset, false)
//	defer unLock(path, raw.Offset, false)
//
//	f, err := fileutils.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
//	if err != nil {
//		return err
//	}
//	defer f.Close()
//	if raw.Length == 0 {
//		if _, err := f.Write(data); err != nil {
//			return err
//		}
//		return nil
//	}
//
//	if _, err := f.Write(data[:raw.Length]); err != nil {
//		return err
//	}
//	return nil
//}

// Stat determines whether the file exists.
func (ds *diskStorage) Stat(ctx context.Context, raw *storedriver.Raw) (*storedriver.StorageInfo, error) {
	_, fileInfo, err := ds.statPath(raw.Bucket, raw.Key)
	if err != nil {
		return nil, err
	}
	return &storedriver.StorageInfo{
		Path:       filepath.Join(raw.Bucket, raw.Key),
		Size:       fileInfo.Size(),
		CreateTime: statutils.Ctime(fileInfo),
		ModTime:    fileInfo.ModTime(),
	}, nil
}

// Exits if filepath exists, include symbol link
func (ds *diskStorage) Exits(ctx context.Context, raw *storedriver.Raw) bool {
	filePath := filepath.Join(ds.BaseDir, raw.Bucket, raw.Key)
	return fileutils.PathExist(filePath)
}

// Remove delete a file or dir.
// It will force delete the file or dir when the raw.Trunc is true.
func (ds *diskStorage) Remove(ctx context.Context, raw *storedriver.Raw) error {
	path, info, err := ds.statPath(raw.Bucket, raw.Key)
	if err != nil {
		return err
	}

	lock(path, -1, false)
	defer unLock(path, -1, false)

	if raw.Trunc || !info.IsDir() {
		return os.RemoveAll(path)
	}
	empty, err := fileutils.IsEmptyDir(path)
	if empty {
		return os.RemoveAll(path)
	}
	return err
}

// GetAvailSpace returns the available disk space in Byte.
func (ds *diskStorage) GetAvailSpace(ctx context.Context) (unit.Bytes, error) {
	path := ds.BaseDir
	lock(path, -1, true)
	defer unLock(path, -1, true)
	return fileutils.GetFreeSpace(path)
}

func (ds *diskStorage) GetTotalAndFreeSpace(ctx context.Context) (unit.Bytes, unit.Bytes, error) {
	path := ds.BaseDir
	lock(path, -1, true)
	defer unLock(path, -1, true)
	return fileutils.GetTotalAndFreeSpace(path)
}

// Walk walks the file tree rooted at root which determined by raw.Bucket and raw.Key,
// calling walkFn for each file or directory in the tree, including root.
func (ds *diskStorage) Walk(ctx context.Context, raw *storedriver.Raw) error {
	path, _, err := ds.statPath(raw.Bucket, raw.Key)
	if err != nil {
		return err
	}

	lock(path, -1, true)
	defer unLock(path, -1, true)

	return filepath.Walk(path, raw.WalkFn)
}

func (ds *diskStorage) GetPath(raw *storedriver.Raw) string {
	return filepath.Join(ds.BaseDir, raw.Bucket, raw.Key)
}

// helper function

// preparePath gets the target path and creates the upper directory if it does not exist.
func (ds *diskStorage) preparePath(bucket, key string) (string, error) {
	dir := filepath.Join(ds.BaseDir, bucket)
	if err := fileutils.MkdirAll(dir); err != nil {
		return "", err
	}
	target := filepath.Join(dir, key)
	return target, nil
}

// statPath determines whether the target file exists and returns an fileMutex if so.
func (ds *diskStorage) statPath(bucket, key string) (string, os.FileInfo, error) {
	filePath := filepath.Join(ds.BaseDir, bucket, key)
	f, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil, errors.Wrapf(cdnerrors.ErrFileNotExist, "no such file or directory:%s exists", filePath)
		}
		return "", nil, err
	}
	return filePath, f, nil
}

func lock(path string, offset int64, ro bool) {
	if offset != -1 {
		fileLocker.Lock(LockKey(path, -1), true)
	}

	fileLocker.Lock(LockKey(path, offset), ro)
}

func unLock(path string, offset int64, ro bool) {
	if offset != -1 {
		fileLocker.UnLock(LockKey(path, -1), true)
	}

	fileLocker.UnLock(LockKey(path, offset), ro)
}

func LockKey(path string, offset int64) string {
	return fmt.Sprintf("%s:%d", path, offset)
}
