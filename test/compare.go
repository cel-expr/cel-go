// Copyright 2018 Google LLC
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

package test

import (
	"fmt"
	"strings"
)

// Compare compares two strings, a for actual, e for expected, and returns true or false. The comparison is done,
// by filtering out whitespace (i.e. space, tabs and newline).
func Compare(a string, e string) bool {
	return stripWhitespace(a) == stripWhitespace(e)
}

// DiffMessage creates a diff dump message for test failures.
func DiffMessage(context string, actual any, expected any) string {
	return fmt.Sprintf("%s: \ngot %v, \nwanted %v", context, actual, expected)
}

func stripWhitespace(a string) string {
	a = strings.Replace(a, " ", "", -1)
	a = strings.Replace(a, "\n", "", -1)
	a = strings.Replace(a, "\t", "", -1)
	return strings.Replace(a, "\r", "", -1)
}
