/*
Copyright 2023 The Vitess Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// This codebase originates from https://github.com/github/freno, See https://github.com/github/freno/blob/master/LICENSE
/*
	MIT License

	Copyright (c) 2017 GitHub

	Permission is hereby granted, free of charge, to any person obtaining a copy
	of this software and associated documentation files (the "Software"), to deal
	in the Software without restriction, including without limitation the rights
	to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
	copies of the Software, and to permit persons to whom the Software is
	furnished to do so, subject to the following conditions:

	The above copyright notice and this permission notice shall be included in all
	copies or substantial portions of the Software.

	THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
	IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
	FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
	AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
	LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
	OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
	SOFTWARE.
*/

package throttle

import (
	"net/http"

	"vitess.io/vitess/go/vt/vttablet/tabletserver/throttle/base"
)

type MetricResult struct {
	StatusCode int     `json:"StatusCode"`
	Scope      string  `json:"Scope"`
	Value      float64 `json:"Value"`
	Threshold  float64 `json:"Threshold"`
	Error      error   `json:"-"`
	Message    string  `json:"Message"`
}

// CheckResult is the result for an app inquiring on a metric. It also exports as JSON via the API
type CheckResult struct {
	StatusCode      int                      `json:"StatusCode"`
	Value           float64                  `json:"Value"`
	Threshold       float64                  `json:"Threshold"`
	Error           error                    `json:"-"`
	Message         string                   `json:"Message"`
	RecentlyChecked bool                     `json:"RecentlyChecked"`
	Metrics         map[string]*MetricResult `json:"Metrics"` // New in multi-metrics support. Will eventually replace the above fields.
}

// NewCheckResult returns a CheckResult
func NewCheckResult(statusCode int, value float64, threshold float64, err error) *CheckResult {
	result := &CheckResult{
		StatusCode: statusCode,
		Value:      value,
		Threshold:  threshold,
		Error:      err,
	}
	if err != nil {
		result.Message = err.Error()
	}
	return result
}

func (c *CheckResult) IsOK() bool {
	return c.StatusCode == http.StatusOK
}

// NewErrorCheckResult returns a check result that indicates an error
func NewErrorCheckResult(statusCode int, err error) *CheckResult {
	return NewCheckResult(statusCode, 0, 0, err)
}

// NoSuchMetricCheckResult is a result returns when a metric is unknown
var NoSuchMetricCheckResult = NewErrorCheckResult(http.StatusNotFound, base.ErrNoSuchMetric)

var okMetricCheckResult = NewCheckResult(http.StatusOK, 0, 0, nil)

var invalidCheckTypeCheckResult = NewErrorCheckResult(http.StatusInternalServerError, base.ErrInvalidCheckType)
