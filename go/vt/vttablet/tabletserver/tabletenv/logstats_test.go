/*
Copyright 2019 The Vitess Authors.

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

package tabletenv

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/safehtml/testconversions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"vitess.io/vitess/go/sqltypes"
	"vitess.io/vitess/go/streamlog"
	"vitess.io/vitess/go/vt/callinfo"
	"vitess.io/vitess/go/vt/callinfo/fakecallinfo"
	querypb "vitess.io/vitess/go/vt/proto/query"
)

func TestLogStats(t *testing.T) {
	logStats := NewLogStats(context.Background(), "test", streamlog.QueryLogConfig{})
	logStats.AddRewrittenSQL("sql1", time.Now())

	if !strings.Contains(logStats.RewrittenSQL(), "sql1") {
		t.Fatalf("RewrittenSQL should contains sql: sql1")
	}

	if logStats.SizeOfResponse() != 0 {
		t.Fatalf("there is no rows in log stats, estimated size should be 0 bytes")
	}

	logStats.Rows = [][]sqltypes.Value{{sqltypes.NewVarBinary("a")}}
	if logStats.SizeOfResponse() <= 0 {
		t.Fatalf("log stats has some rows, should have positive response size")
	}
}

func testFormat(stats *LogStats, params url.Values) string {
	var b strings.Builder
	stats.Logf(&b, params)
	return b.String()
}

func TestLogStatsFormat(t *testing.T) {
	logStats := NewLogStats(context.Background(), "test", streamlog.NewQueryLogConfigForTest())
	logStats.StartTime = time.Date(2017, time.January, 1, 1, 2, 3, 0, time.UTC)
	logStats.EndTime = time.Date(2017, time.January, 1, 1, 2, 4, 1234, time.UTC)
	logStats.OriginalSQL = "sql"
	logStats.BindVariables = map[string]*querypb.BindVariable{"intVal": sqltypes.Int64BindVariable(1)}
	logStats.AddRewrittenSQL("sql with pii", time.Now())
	logStats.MysqlResponseTime = 0
	logStats.TransactionID = 12345
	logStats.Rows = [][]sqltypes.Value{{sqltypes.NewVarBinary("a")}}
	params := map[string][]string{"full": {}}

	got := testFormat(logStats, params)
	want := "test\t\t\t''\t''\t2017-01-01 01:02:03.000000\t2017-01-01 01:02:04.000001\t1.000001\t\t\"sql\"\t{\"intVal\": {\"type\": \"INT64\", \"value\": 1}}\t1\t\"sql with pii\"\tmysql\t0.000000\t0.000000\t0\t12345\t1\t\"\"\t\n"
	assert.Equal(t, want, got)

	logStats.Config.RedactDebugUIQueries = true

	got = testFormat(logStats, params)
	want = "test\t\t\t''\t''\t2017-01-01 01:02:03.000000\t2017-01-01 01:02:04.000001\t1.000001\t\t\"sql\"\t\"[REDACTED]\"\t1\t\"[REDACTED]\"\tmysql\t0.000000\t0.000000\t0\t12345\t1\t\"\"\t\n"
	assert.Equal(t, want, got)

	logStats.Config.RedactDebugUIQueries = false
	logStats.Config.Format = streamlog.QueryLogFormatJSON

	got = testFormat(logStats, params)
	var parsed map[string]any
	err := json.Unmarshal([]byte(got), &parsed)
	if err != nil {
		t.Errorf("logstats format: error unmarshaling json: %v -- got:\n%v", err, got)
	}
	formatted, err := json.MarshalIndent(parsed, "", "    ")
	require.NoError(t, err)
	want = "{\n    \"BindVars\": {\n        \"intVal\": {\n            \"type\": \"INT64\",\n            \"value\": 1\n        }\n    },\n    \"CallInfo\": \"\",\n    \"ConnWaitTime\": 0,\n    \"Effective Caller\": \"\",\n    \"End\": \"2017-01-01 01:02:04.000001\",\n    \"Error\": \"\",\n    \"ImmediateCaller\": \"\",\n    \"Method\": \"test\",\n    \"MysqlTime\": 0,\n    \"OriginalSQL\": \"sql\",\n    \"PlanType\": \"\",\n    \"Queries\": 1,\n    \"QuerySources\": \"mysql\",\n    \"ResponseSize\": 1,\n    \"RewrittenSQL\": \"sql with pii\",\n    \"RowsAffected\": 0,\n    \"Start\": \"2017-01-01 01:02:03.000000\",\n    \"TotalTime\": 1.000001,\n    \"TransactionID\": 12345,\n    \"Username\": \"\"\n}"
	assert.Equal(t, want, string(formatted))

	logStats.Config.RedactDebugUIQueries = true
	logStats.Config.Format = streamlog.QueryLogFormatJSON

	got = testFormat(logStats, params)
	err = json.Unmarshal([]byte(got), &parsed)
	require.NoError(t, err)
	formatted, err = json.MarshalIndent(parsed, "", "    ")
	require.NoError(t, err)
	want = "{\n    \"BindVars\": \"[REDACTED]\",\n    \"CallInfo\": \"\",\n    \"ConnWaitTime\": 0,\n    \"Effective Caller\": \"\",\n    \"End\": \"2017-01-01 01:02:04.000001\",\n    \"Error\": \"\",\n    \"ImmediateCaller\": \"\",\n    \"Method\": \"test\",\n    \"MysqlTime\": 0,\n    \"OriginalSQL\": \"sql\",\n    \"PlanType\": \"\",\n    \"Queries\": 1,\n    \"QuerySources\": \"mysql\",\n    \"ResponseSize\": 1,\n    \"RewrittenSQL\": \"[REDACTED]\",\n    \"RowsAffected\": 0,\n    \"Start\": \"2017-01-01 01:02:03.000000\",\n    \"TotalTime\": 1.000001,\n    \"TransactionID\": 12345,\n    \"Username\": \"\"\n}"
	assert.Equal(t, want, string(formatted))

	// Make sure formatting works for string bind vars. We can't do this as part of a single
	// map because the output ordering is undefined.
	logStats.BindVariables = map[string]*querypb.BindVariable{"strVal": sqltypes.StringBindVariable("abc")}
	logStats.Config.RedactDebugUIQueries = false
	logStats.Config.Format = streamlog.QueryLogFormatText

	got = testFormat(logStats, params)
	want = "test\t\t\t''\t''\t2017-01-01 01:02:03.000000\t2017-01-01 01:02:04.000001\t1.000001\t\t\"sql\"\t{\"strVal\": {\"type\": \"VARCHAR\", \"value\": \"abc\"}}\t1\t\"sql with pii\"\tmysql\t0.000000\t0.000000\t0\t12345\t1\t\"\"\t\n"
	assert.Equal(t, want, got)

	logStats.Config.RedactDebugUIQueries = false
	logStats.Config.Format = streamlog.QueryLogFormatJSON

	got = testFormat(logStats, params)
	err = json.Unmarshal([]byte(got), &parsed)
	require.NoError(t, err)
	formatted, err = json.MarshalIndent(parsed, "", "    ")
	require.NoError(t, err)
	want = "{\n    \"BindVars\": {\n        \"strVal\": {\n            \"type\": \"VARCHAR\",\n            \"value\": \"abc\"\n        }\n    },\n    \"CallInfo\": \"\",\n    \"ConnWaitTime\": 0,\n    \"Effective Caller\": \"\",\n    \"End\": \"2017-01-01 01:02:04.000001\",\n    \"Error\": \"\",\n    \"ImmediateCaller\": \"\",\n    \"Method\": \"test\",\n    \"MysqlTime\": 0,\n    \"OriginalSQL\": \"sql\",\n    \"PlanType\": \"\",\n    \"Queries\": 1,\n    \"QuerySources\": \"mysql\",\n    \"ResponseSize\": 1,\n    \"RewrittenSQL\": \"sql with pii\",\n    \"RowsAffected\": 0,\n    \"Start\": \"2017-01-01 01:02:03.000000\",\n    \"TotalTime\": 1.000001,\n    \"TransactionID\": 12345,\n    \"Username\": \"\"\n}"
	assert.Equal(t, want, string(formatted))
}

func TestLogStatsFilter(t *testing.T) {
	logStats := NewLogStats(context.Background(), "test", streamlog.NewQueryLogConfigForTest())
	logStats.StartTime = time.Date(2017, time.January, 1, 1, 2, 3, 0, time.UTC)
	logStats.EndTime = time.Date(2017, time.January, 1, 1, 2, 4, 1234, time.UTC)
	logStats.OriginalSQL = "sql /* LOG_THIS_QUERY */"
	logStats.BindVariables = map[string]*querypb.BindVariable{"intVal": sqltypes.Int64BindVariable(1)}
	logStats.AddRewrittenSQL("sql with pii", time.Now())
	logStats.MysqlResponseTime = 0
	logStats.Rows = [][]sqltypes.Value{{sqltypes.NewVarBinary("a")}}
	params := map[string][]string{"full": {}}

	got := testFormat(logStats, params)
	want := "test\t\t\t''\t''\t2017-01-01 01:02:03.000000\t2017-01-01 01:02:04.000001\t1.000001\t\t\"sql /* LOG_THIS_QUERY */\"\t{\"intVal\": {\"type\": \"INT64\", \"value\": 1}}\t1\t\"sql with pii\"\tmysql\t0.000000\t0.000000\t0\t0\t1\t\"\"\t\n"
	if got != want {
		t.Errorf("logstats format: got:\n%q\nwant:\n%q\n", got, want)
	}

	logStats.Config.FilterTag = "LOG_THIS_QUERY"
	got = testFormat(logStats, params)
	want = "test\t\t\t''\t''\t2017-01-01 01:02:03.000000\t2017-01-01 01:02:04.000001\t1.000001\t\t\"sql /* LOG_THIS_QUERY */\"\t{\"intVal\": {\"type\": \"INT64\", \"value\": 1}}\t1\t\"sql with pii\"\tmysql\t0.000000\t0.000000\t0\t0\t1\t\"\"\t\n"
	if got != want {
		t.Errorf("logstats format: got:\n%q\nwant:\n%q\n", got, want)
	}

	logStats.Config.FilterTag = "NOT_THIS_QUERY"
	got = testFormat(logStats, params)
	want = ""
	if got != want {
		t.Errorf("logstats format: got:\n%q\nwant:\n%q\n", got, want)
	}
}

func TestLogStatsFormatQuerySources(t *testing.T) {
	logStats := NewLogStats(context.Background(), "test", streamlog.NewQueryLogConfigForTest())
	if logStats.FmtQuerySources() != "none" {
		t.Fatalf("should return none since log stats does not have any query source, but got: %s", logStats.FmtQuerySources())
	}

	logStats.QuerySources |= QuerySourceMySQL
	if !strings.Contains(logStats.FmtQuerySources(), "mysql") {
		t.Fatalf("'mysql' should be in formatted query sources")
	}

	logStats.QuerySources |= QuerySourceConsolidator
	if !strings.Contains(logStats.FmtQuerySources(), "consolidator") {
		t.Fatalf("'consolidator' should be in formatted query sources")
	}
}

func TestLogStatsContextHTML(t *testing.T) {
	html := "HtmlContext"
	callInfo := &fakecallinfo.FakeCallInfo{
		Html: testconversions.MakeHTMLForTest(html),
	}
	ctx := callinfo.NewContext(context.Background(), callInfo)
	logStats := NewLogStats(ctx, "test", streamlog.NewQueryLogConfigForTest())
	if logStats.ContextHTML().String() != html {
		t.Fatalf("expect to get html: %s, but got: %s", html, logStats.ContextHTML().String())
	}
}

func TestLogStatsErrorStr(t *testing.T) {
	logStats := NewLogStats(context.Background(), "test", streamlog.NewQueryLogConfigForTest())
	if logStats.ErrorStr() != "" {
		t.Fatalf("should not get error in stats, but got: %s", logStats.ErrorStr())
	}
	errStr := "unknown error"
	logStats.Error = errors.New(errStr)
	if !strings.Contains(logStats.ErrorStr(), errStr) {
		t.Fatalf("expect string '%s' in error message, but got: %s", errStr, logStats.ErrorStr())
	}
}

func TestLogStatsCallInfo(t *testing.T) {
	logStats := NewLogStats(context.Background(), "test", streamlog.NewQueryLogConfigForTest())
	caller, user := logStats.CallInfo()
	if caller != "" {
		t.Fatalf("caller should be empty")
	}
	if user != "" {
		t.Fatalf("username should be empty")
	}

	remoteAddr := "1.2.3.4"
	username := "vt"
	callInfo := &fakecallinfo.FakeCallInfo{
		Remote: remoteAddr,
		Method: "FakeExecute",
		User:   username,
	}
	ctx := callinfo.NewContext(context.Background(), callInfo)
	logStats = NewLogStats(ctx, "test", streamlog.NewQueryLogConfigForTest())
	caller, user = logStats.CallInfo()
	wantCaller := remoteAddr + ":FakeExecute(fakeRPC)"
	if caller != wantCaller {
		t.Fatalf("expected to get caller: %s, but got: %s", wantCaller, caller)
	}
	if user != username {
		t.Fatalf("expected to get username: %s, but got: %s", username, user)
	}
}

// TestLogStatsErrorsOnly tests that LogStats only logs errors when the query log mode is set to errors only for VTTablet.
func TestLogStatsErrorsOnly(t *testing.T) {
	logStats := NewLogStats(context.Background(), "test", streamlog.NewQueryLogConfigForTest())
	logStats.Config.Mode = streamlog.QueryLogModeError

	// no error, should not log
	logOutput := testFormat(logStats, url.Values{})
	assert.Empty(t, logOutput)

	// error, should log
	logStats.Error = errors.New("test error")
	logOutput = testFormat(logStats, url.Values{})
	assert.Contains(t, logOutput, "test error")
}
