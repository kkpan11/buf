// Copyright 2020-2025 Buf Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package bufmodule

import (
	"context"
	"sync"

	"github.com/bufbuild/buf/private/pkg/standard/xslices"
	"github.com/bufbuild/buf/private/pkg/storage"
)

const (
	// licenseFilePath is the path of the license file within a Module.
	licenseFilePath = "LICENSE"
)

var (
	// orderedDocFilePaths are the potential documentation file paths for a Module.
	//
	// When creating a Module from a Bucket, we check the file paths buf.md, README.md, and README.markdown
	// to exist, in that order. The first one to exist is chosen as the documentation file that is considered
	// part of the Module, and any others are discarded.
	orderedDocFilePaths = []string{
		"buf.md",
		"README.md",
		"README.markdown",
	}

	// docFilePathMap is a map of all valid documentation file paths.
	docFilePathMap map[string]struct{}
)

func init() {
	docFilePathMap = xslices.ToStructMap(orderedDocFilePaths)
}

func getDocFilePathForStorageReadBucket(ctx context.Context, bucket storage.ReadBucket) string {
	for _, docFilePath := range orderedDocFilePaths {
		if _, err := bucket.Stat(ctx, docFilePath); err == nil {
			return docFilePath
		}
	}
	return ""
}

func getDocFilePathForModuleReadBucket(ctx context.Context, bucket ModuleReadBucket) string {
	for _, docFilePath := range orderedDocFilePaths {
		if _, err := bucket.StatFileInfo(ctx, docFilePath); err == nil {
			return docFilePath
		}
	}
	return ""
}

// getStorageMatcher gets the storage.Matcher that will filter the storage.ReadBucket down to specifically
// the files that are relevant to a module.
func getStorageMatcher(ctx context.Context, bucket storage.ReadBucket) storage.Matcher {
	return storage.MatchOr(
		storage.MatchPathExt(".proto"),
		storage.MatchPathEqual(licenseFilePath),
		storage.MatchPathEqual(getDocFilePathForStorageReadBucket(ctx, bucket)),
	)
}

// getSyncOnceValuesGetBucketWithStorageMatcherApplied wraps the getBucket function with sync.OnceValues
// and getStorageMatcher applied.
//
// This is used when constructing moduleReadBuckets in moduleSetBuilder, and when getting a bucket for
// module digest calculations in moduleData.
func getSyncOnceValuesGetBucketWithStorageMatcherApplied(
	ctx context.Context,
	getBucket func() (storage.ReadBucket, error),
) func() (storage.ReadBucket, error) {
	return sync.OnceValues(
		func() (storage.ReadBucket, error) {
			bucket, err := getBucket()
			if err != nil {
				return nil, err
			}
			return storage.FilterReadBucket(bucket, getStorageMatcher(ctx, bucket)), nil
		},
	)
}
