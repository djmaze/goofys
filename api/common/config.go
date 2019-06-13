// Copyright 2015 - 2019 Ka-Hing Cheung
// Copyright 2015 - 2017 Google Inc. All Rights Reserved.
// Copyright 2019 Databricks
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package common

import (
	"mime"
	"os"
	"strings"
	"time"
)

type FlagStorage struct {
	// File system
	MountOptions      map[string]string
	MountPoint        string
	MountPointArg     string
	MountPointCreated string

	Cache    []string
	DirMode  os.FileMode
	FileMode os.FileMode
	Uid      uint32
	Gid      uint32

	// S3
	Endpoint       string
	Region         string
	AccessKey      string
	SecretKey      string
	RequesterPays  bool
	RegionSet      bool
	StorageClass   string
	Profile        string
	UseContentType bool
	UseSSE         bool
	UseKMS         bool
	KMSKeyID       string
	ACL            string
	Subdomain      bool

	Backend interface{}

	// Tuning
	Cheap        bool
	ExplicitDir  bool
	StatCacheTTL time.Duration
	TypeCacheTTL time.Duration
	HTTPTimeout  time.Duration

	// Debugging
	DebugFuse  bool
	DebugS3    bool
	Foreground bool
}

func (flags *FlagStorage) GetMimeType(fileName string) (retMime *string) {
	if flags.UseContentType {
		dotPosition := strings.LastIndex(fileName, ".")
		if dotPosition == -1 {
			return nil
		}
		mimeType := mime.TypeByExtension(fileName[dotPosition:])
		if mimeType == "" {
			return nil
		}
		semicolonPosition := strings.LastIndex(mimeType, ";")
		if semicolonPosition == -1 {
			return &mimeType
		}
		s := mimeType[:semicolonPosition]
		retMime = &s
	}

	return
}

func (flags *FlagStorage) Cleanup() {
	if flags.MountPointCreated != "" && flags.MountPointCreated != flags.MountPointArg {
		err := os.Remove(flags.MountPointCreated)
		if err != nil {
			log.Errorf("rmdir %v = %v", flags.MountPointCreated, err)
		}
	}
}
