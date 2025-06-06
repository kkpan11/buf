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

// Package normalpath provides functions similar to filepath.
//
// A normalized path is a cleaned and to-slash'ed path.
// A validated path validates that a path is relative and does not jump context.
package normalpath

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bufbuild/buf/private/pkg/standard/xslices"
)

const (
	// Relative is the PathType for normalized and validated paths.
	Relative PathType = 1
	// Absolute is the PathType for normalized and absolute paths.
	Absolute PathType = 2

	stringOSPathSeparator = string(os.PathSeparator)
	// This has to be with "/" instead of os.PathSeparator as we use this on normalized paths
	normalizedRelPathJumpContextPrefix = "../"
)

var (
	// errNotRelative is the error returned if the path is not relative.
	errNotRelative = errors.New("expected to be relative")
	// errOutsideContextDir is the error returned if the path is outside the context directory.
	errOutsideContextDir = errors.New("is outside the context directory")
)

// PathType is a terminate type for path comparisons.
type PathType int

// Separator gets the string value of the separator.
//
// TODO: rename to Terminator if we keep this
// TODO: we should probably refactor so we never need to use absolute paths at all
// this could be accomplished if we could for ExternalPathToRelPath on buckets
func (t PathType) Separator() string {
	switch t {
	case Relative:
		return "."
	case Absolute:
		return "/"
	default:
		return ""
	}
}

// Error is a path error.
type Error struct {
	Path string
	Err  error
}

// NewError returns a new Error.
func NewError(path string, err error) *Error {
	return &Error{
		Path: path,
		Err:  err,
	}
}

// Error implements error.
func (e *Error) Error() string {
	errString := ""
	if e.Err != nil {
		errString = e.Err.Error()
	}
	if errString == "" {
		errString = "error"
	}
	return e.Path + ": " + errString
}

// Unwrap implements errors.Unwrap for Error.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// Normalize normalizes the given path.
//
// This calls filepath.Clean and filepath.ToSlash on the path.
// If the path is "" or ".", this returns ".".
func Normalize(path string) string {
	return filepath.ToSlash(filepath.Clean(path))
}

// NormalizeAndAbsolute normalizes the path and makes it absolute.
func NormalizeAndAbsolute(path string) (string, error) {
	absPath, err := filepath.Abs(Unnormalize(path))
	if err != nil {
		return "", err
	}
	return Normalize(absPath), nil
}

// NormalizeAndTransformForPathType calls NormalizeAndValidate for relative
// paths, and NormalizeAndAbsolute for absolute paths.
func NormalizeAndTransformForPathType(path string, pathType PathType) (string, error) {
	switch pathType {
	case Relative:
		return NormalizeAndValidate(path)
	case Absolute:
		return NormalizeAndAbsolute(path)
	default:
		return "", fmt.Errorf("unknown PathType: %v", pathType)
	}
}

// Unnormalize unnormalizes the given path.
//
// This calls filepath.FromSlash on the path.
// If the path is "", this returns "".
func Unnormalize(path string) string {
	return filepath.FromSlash(path)
}

// Base is equivalent to filepath.Base.
//
// Normalizes before returning.
func Base(path string) string {
	return Normalize(filepath.Base(Unnormalize(path)))
}

// Dir is equivalent to filepath.Dir.
//
// Normalizes before returning.
func Dir(path string) string {
	return Normalize(filepath.Dir(Unnormalize(path)))
}

// Ext is equivalent to filepath.Ext.
//
// Can return empty string.
func Ext(path string) string {
	return filepath.Ext(Unnormalize(path))
}

// Join is equivalent to filepath.Join.
//
// Empty strings are ignored,
// Can return empty string.
//
// Normalizes before returning otherwise.
func Join(paths ...string) string {
	unnormalized := make([]string, len(paths))
	for i, path := range paths {
		unnormalized[i] = Unnormalize(path)
	}
	value := filepath.Join(unnormalized...)
	if value == "" {
		return ""
	}
	return Normalize(value)
}

// Rel is equivalent to filepath.Rel.
//
// Can return empty string, especially on error.
//
// Normalizes before returning otherwise.
func Rel(basepath string, targpath string) (string, error) {
	path, err := filepath.Rel(Unnormalize(basepath), Unnormalize(targpath))
	if path == "" {
		return "", err
	}
	return Normalize(path), err
}

// ByDir maps the paths into a map from directory via Dir to the original paths.
//
// The paths for each value slice will be sorted.
//
// The path is expected to be normalized.
func ByDir(paths ...string) map[string][]string {
	m := make(map[string][]string)
	for _, path := range paths {
		path = Normalize(path)
		dir := filepath.Dir(path)
		m[dir] = append(m[dir], path)
	}
	for _, dirPaths := range m {
		sort.Strings(dirPaths)
	}
	return m
}

// ChunkByDir splits paths into chunks of around the suggestedChunkSize while keeping
// directory affinity, i.e. paths in the same directory will be in the same chunk.
//
// Paths are expected to be normalized.
// If paths is nil or empty, returns empty.
// If suggestedChunkSize is <=0, returns [][]string{paths}.
func ChunkByDir(paths []string, suggestedChunkSize int) [][]string {
	var chunks [][]string
	if len(paths) == 0 {
		return chunks
	}
	if suggestedChunkSize <= 0 {
		return [][]string{paths}
	}

	dirToPaths := ByDir(paths...)
	// Get the keys and sort them to keep the output of this stable
	dirs := make([]string, 0, len(dirToPaths))
	for dir := range dirToPaths {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	// Create a slice of the per-directory path slices
	pathsByDir := make([][]string, 0, len(dirToPaths))
	for _, dir := range dirs {
		pathsByDir = append(pathsByDir, dirToPaths[dir])
	}
	// Sort the per-directory path slices by length, from greatest to smallest
	// We do greatest to smallest because we pop the greatest off first, and
	// This will approximate keeping things in alphabetical order given that
	// pathsByDir is in alphabetical order of directories.
	sort.SliceStable(
		pathsByDir,
		func(i int, j int) bool {
			return len(pathsByDir[i]) > len(pathsByDir[j])
		},
	)

	for len(pathsByDir) > 0 {
		// Take the largest set of paths and remove from pathsByDir
		chunk := pathsByDir[0]
		pathsByDir = pathsByDir[1:]
		// While our current chunk is less than the suggestedChunkSize, take the smallest
		// set of paths and append them to the chunk, and then remove them from pathsByDir
		//
		// This will mean that sometimes we have chunks larger than suggestedChunkSize, but
		// this is best effort. We could likely use more-complicated algorithms here that
		// smartly combine per-directory path sets that minimize the distance from
		// suggestedChunkSize, as there are adversary cases to this algorithm, but we
		// are not diving into that at the moment unless there is a need.
		for len(chunk) < suggestedChunkSize && len(pathsByDir) > 0 {
			chunk = append(chunk, pathsByDir[len(pathsByDir)-1]...)
			pathsByDir = pathsByDir[:len(pathsByDir)-1]
		}
		// Append the chunk to the list of chunks
		chunks = append(chunks, chunk)
	}

	return chunks
}

// ContainsPath returns true if the dirPath contains the path.
//
// The path and value are expected to be normalized and validated if Relative is used.
// The path and value are expected to be normalized and absolute if Absolute is used.
//
// For a given dirPath:
//
//   - If path == PathType, dirPath does not contain the path.
//   - If dirPath == PathType, the dirPath contains the path.
//   - If dirPath is a directory that contains path, this returns true.
func ContainsPath(dirPath string, path string, pathType PathType) bool {
	if dirPath == path {
		return false
	}
	return EqualsOrContainsPath(dirPath, Dir(path), pathType)
}

// MapAllEqualOrContainingPaths returns the matching paths in the map in a sorted slice.
//
// The path and keys in m are expected to be normalized and validated if Relative is used.
// The path and keys in m are expected to be normalized and absolute if Absolute is used.
//
// If the map is empty, returns nil.
func MapAllEqualOrContainingPaths(m map[string]struct{}, path string, pathType PathType) []string {
	if len(m) == 0 {
		return nil
	}
	return xslices.MapKeysToSortedSlice(MapAllEqualOrContainingPathMap(m, path, pathType))
}

// StripComponents strips the specified number of components.
//
// Path expected to be normalized.
// Returns false if the path does not have more than the specified number of components.
func StripComponents(path string, countUint32 uint32) (string, bool) {
	count := int(countUint32)
	if count == 0 {
		return path, true
	}
	components := Components(path)
	if len(components) <= count {
		return "", false
	}
	return Join(components[count:]...), true
}

// ValidatePathComponent validates that the string is a valid
// component of a path, e.g. it can be Joined and form a valid path.
func ValidatePathComponent(component string) error {
	if component == "" {
		return errors.New("path component must not be empty")
	}
	if strings.ContainsRune(component, '/') {
		return errors.New(`path component must not contain "/" `)
	}
	if strings.Contains(component, "..") {
		return errors.New(`path component must not contain ".."`)
	}
	if url.PathEscape(component) != component {
		return fmt.Errorf(
			"path component must match its URL escaped version: %q did not match %q",
			component,
			url.PathEscape(component),
		)
	}
	return nil
}

// ValidatePathComponents validates that all the strings are valid
// components of a path, e.g. they can be Joined and form a valid path.
func ValidatePathComponents(components ...string) error {
	for _, component := range components {
		if err := ValidatePathComponent(component); err != nil {
			return err
		}
	}
	return nil
}

// ValidatePathsNormalizedValidatedUnique validates the file or directory paths are normalized
// and validated, and not duplicated.
func ValidatePathsNormalizedValidatedUnique(paths []string) error {
	pathMap := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		if path == "" {
			return errors.New("path is empty")
		}
		normalized, err := NormalizeAndValidate(path)
		if err != nil {
			return fmt.Errorf("path had normalization error: %w", err)
		}
		if path != normalized {
			return fmt.Errorf("path %s was not normalized to %s", path, normalized)
		}
		if _, ok := pathMap[path]; ok {
			return fmt.Errorf("duplicate path: %s", path)
		}
		pathMap[path] = struct{}{}
	}
	return nil
}
