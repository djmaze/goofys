// Copyright 2019 Ka-Hing Cheung
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

package internal

import (
	"fmt"
	"net/url"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
)

// GCS variant of S3
type GCS3 struct {
	*S3Backend
}

func NewGCS3(fs *Goofys, bucket string, awsConfig *aws.Config, flags *FlagStorage) *GCS3 {
	s := &GCS3{
		S3Backend: NewS3(fs, bucket, awsConfig, flags),
	}
	s.S3Backend.gcs = true
	return s
}

func (s *GCS3) DeleteBlobs(param *DeleteBlobsInput) (*DeleteBlobsOutput, error) {
	// GCS does not have multi-delete
	var wg sync.WaitGroup
	var overallErr error

	for _, key := range param.Items {
		wg.Add(1)
		go func() {
			_, err := s.DeleteBlob(&DeleteBlobInput{
				Key: key,
			})
			if err != nil {
				overallErr = err
			}
			wg.Done()
		}()
	}
	wg.Wait()
	if overallErr != nil {
		return nil, mapAwsError(overallErr)
	}

	return &DeleteBlobsOutput{}, nil
}

func (s *GCS3) MultipartBlobBegin(param *MultipartBlobBeginInput) (*MultipartBlobCommitInput, error) {
	mpu := s3.CreateMultipartUploadInput{
		Bucket:       &s.bucket,
		Key:          &param.Key,
		StorageClass: &s.flags.StorageClass,
		ContentType:  param.ContentType,
	}

	if s.flags.UseSSE {
		mpu.ServerSideEncryption = &s.sseType
		if s.flags.UseKMS && s.flags.KMSKeyID != "" {
			mpu.SSEKMSKeyId = &s.flags.KMSKeyID
		}
	}

	if s.flags.ACL != "" {
		mpu.ACL = &s.flags.ACL
	}

	req, _ := s.CreateMultipartUploadRequest(&mpu)
	// get rid of ?uploads=
	req.HTTPRequest.URL.RawQuery = ""
	req.HTTPRequest.Header.Set("x-goog-resumable", "start")

	err := req.Send()
	if err != nil {
		s3Log.Errorf("CreateMultipartUpload %v = %v", param.Key, err)
		return nil, mapAwsError(err)
	}

	location := req.HTTPResponse.Header.Get("Location")
	_, err = url.Parse(location)
	if err != nil {
		s3Log.Errorf("CreateMultipartUpload %v = %v", param.Key, err)
		return nil, mapAwsError(err)
	}

	return &MultipartBlobCommitInput{
		Key:      &param.Key,
		Metadata: param.Metadata,
		UploadId: &location,
		Parts:    make([]*string, 10000), // at most 10K parts
	}, nil
}

func (s *GCS3) MultipartBlobAdd(param *MultipartBlobAddInput) (*MultipartBlobAddOutput, error) {
	atomic.AddUint32(&param.Commit.NumParts, 1)

	// the mpuId serves as authentication token so
	// technically we don't need to sign this anymore and
	// can just use a plain HTTP request, but going
	// through aws-sdk-go anyway to get retry handling
	params := &s3.PutObjectInput{
		Bucket: &s.bucket,
		Key:    param.Commit.Key,
		Body:   param.Body,
	}

	s3Log.Debug(params)

	req, resp := s.PutObjectRequest(params)
	req.HTTPRequest.URL, _ = url.Parse(*param.Commit.UploadId)

	atomic.AddUint64(&param.Commit.Size, param.Size)

	start := param.Commit.Size - param.Size
	end := param.Commit.Size - 1
	var size string
	if param.Last {
		size = strconv.FormatUint(param.Commit.Size, 10)
	} else {
		size = "*"
	}

	contentRange := fmt.Sprintf("bytes %v-%v/%v", start, end, size)

	req.HTTPRequest.Header.Set("Content-Length", strconv.FormatUint(param.Size, 10))
	req.HTTPRequest.Header.Set("Content-Range", contentRange)

	err := req.Send()
	if err != nil {
		// status indicating that we need more parts to finish this
		if req.HTTPResponse.StatusCode == 308 {
			err = nil
		} else {
			return nil, mapAwsError(err)
		}
	}

	if param.Last {
		param.Commit.ETag = resp.ETag
	}

	return &MultipartBlobAddOutput{}, nil
}

func (s *GCS3) MultipartBlobCommit(param *MultipartBlobCommitInput) (*MultipartBlobCommitOutput, error) {
	// nothing, we already uploaded last part
	return &MultipartBlobCommitOutput{
		ETag: param.ETag,
	}, nil
}