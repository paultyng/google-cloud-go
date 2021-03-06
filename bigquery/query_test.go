// Copyright 2015 Google Inc. All Rights Reserved.
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

package bigquery

import (
	"testing"

	"cloud.google.com/go/internal/testutil"

	bq "google.golang.org/api/bigquery/v2"
)

func defaultQueryJob() *bq.Job {
	return &bq.Job{
		JobReference: &bq.JobReference{JobId: "RANDOM", ProjectId: "client-project-id"},
		Configuration: &bq.JobConfiguration{
			Query: &bq.JobConfigurationQuery{
				DestinationTable: &bq.TableReference{
					ProjectId: "client-project-id",
					DatasetId: "dataset-id",
					TableId:   "table-id",
				},
				Query: "query string",
				DefaultDataset: &bq.DatasetReference{
					ProjectId: "def-project-id",
					DatasetId: "def-dataset-id",
				},
				UseLegacySql:    false,
				ForceSendFields: []string{"UseLegacySql"},
			},
		},
	}
}

func TestQuery(t *testing.T) {
	defer fixRandomID("RANDOM")()
	c := &Client{
		projectID: "client-project-id",
	}
	testCases := []struct {
		dst         *Table
		src         *QueryConfig
		jobIDConfig JobIDConfig
		want        *bq.Job
	}{
		{
			dst:  c.Dataset("dataset-id").Table("table-id"),
			src:  defaultQuery,
			want: defaultQueryJob(),
		},
		{
			dst: c.Dataset("dataset-id").Table("table-id"),
			src: &QueryConfig{
				Q: "query string",
			},
			want: func() *bq.Job {
				j := defaultQueryJob()
				j.Configuration.Query.DefaultDataset = nil
				return j
			}(),
		},
		{
			dst:         c.Dataset("dataset-id").Table("table-id"),
			jobIDConfig: JobIDConfig{JobID: "jobID", AddJobIDSuffix: true},
			src:         &QueryConfig{Q: "query string"},
			want: func() *bq.Job {
				j := defaultQueryJob()
				j.Configuration.Query.DefaultDataset = nil
				j.JobReference.JobId = "jobID-RANDOM"
				return j
			}(),
		},
		{
			dst: &Table{},
			src: defaultQuery,
			want: func() *bq.Job {
				j := defaultQueryJob()
				j.Configuration.Query.DestinationTable = nil
				return j
			}(),
		},
		{
			dst: c.Dataset("dataset-id").Table("table-id"),
			src: &QueryConfig{
				Q: "query string",
				TableDefinitions: map[string]ExternalData{
					"atable": func() *GCSReference {
						g := NewGCSReference("uri")
						g.AllowJaggedRows = true
						g.AllowQuotedNewlines = true
						g.Compression = Gzip
						g.Encoding = UTF_8
						g.FieldDelimiter = ";"
						g.IgnoreUnknownValues = true
						g.MaxBadRecords = 1
						g.Quote = "'"
						g.SkipLeadingRows = 2
						g.Schema = Schema([]*FieldSchema{
							{Name: "name", Type: StringFieldType},
						})
						return g
					}(),
				},
			},
			want: func() *bq.Job {
				j := defaultQueryJob()
				j.Configuration.Query.DefaultDataset = nil
				td := make(map[string]bq.ExternalDataConfiguration)
				quote := "'"
				td["atable"] = bq.ExternalDataConfiguration{
					Compression:         "GZIP",
					IgnoreUnknownValues: true,
					MaxBadRecords:       1,
					SourceFormat:        "CSV", // must be explicitly set.
					SourceUris:          []string{"uri"},
					CsvOptions: &bq.CsvOptions{
						AllowJaggedRows:     true,
						AllowQuotedNewlines: true,
						Encoding:            "UTF-8",
						FieldDelimiter:      ";",
						SkipLeadingRows:     2,
						Quote:               &quote,
					},
					Schema: &bq.TableSchema{
						Fields: []*bq.TableFieldSchema{
							{Name: "name", Type: "STRING"},
						},
					},
				}
				j.Configuration.Query.TableDefinitions = td
				return j
			}(),
		},
		{
			dst: &Table{
				ProjectID: "project-id",
				DatasetID: "dataset-id",
				TableID:   "table-id",
			},
			src: &QueryConfig{
				Q:                 "query string",
				DefaultProjectID:  "def-project-id",
				DefaultDatasetID:  "def-dataset-id",
				CreateDisposition: CreateNever,
				WriteDisposition:  WriteTruncate,
			},
			want: func() *bq.Job {
				j := defaultQueryJob()
				j.Configuration.Query.DestinationTable.ProjectId = "project-id"
				j.Configuration.Query.WriteDisposition = "WRITE_TRUNCATE"
				j.Configuration.Query.CreateDisposition = "CREATE_NEVER"
				return j
			}(),
		},
		{
			dst: c.Dataset("dataset-id").Table("table-id"),
			src: &QueryConfig{
				Q:                 "query string",
				DefaultProjectID:  "def-project-id",
				DefaultDatasetID:  "def-dataset-id",
				DisableQueryCache: true,
			},
			want: func() *bq.Job {
				j := defaultQueryJob()
				f := false
				j.Configuration.Query.UseQueryCache = &f
				return j
			}(),
		},
		{
			dst: c.Dataset("dataset-id").Table("table-id"),
			src: &QueryConfig{
				Q:                 "query string",
				DefaultProjectID:  "def-project-id",
				DefaultDatasetID:  "def-dataset-id",
				AllowLargeResults: true,
			},
			want: func() *bq.Job {
				j := defaultQueryJob()
				j.Configuration.Query.AllowLargeResults = true
				return j
			}(),
		},
		{
			dst: c.Dataset("dataset-id").Table("table-id"),
			src: &QueryConfig{
				Q:                       "query string",
				DefaultProjectID:        "def-project-id",
				DefaultDatasetID:        "def-dataset-id",
				DisableFlattenedResults: true,
			},
			want: func() *bq.Job {
				j := defaultQueryJob()
				f := false
				j.Configuration.Query.FlattenResults = &f
				j.Configuration.Query.AllowLargeResults = true
				return j
			}(),
		},
		{
			dst: c.Dataset("dataset-id").Table("table-id"),
			src: &QueryConfig{
				Q:                "query string",
				DefaultProjectID: "def-project-id",
				DefaultDatasetID: "def-dataset-id",
				Priority:         QueryPriority("low"),
			},
			want: func() *bq.Job {
				j := defaultQueryJob()
				j.Configuration.Query.Priority = "low"
				return j
			}(),
		},
		{
			dst: c.Dataset("dataset-id").Table("table-id"),
			src: &QueryConfig{
				Q:                "query string",
				DefaultProjectID: "def-project-id",
				DefaultDatasetID: "def-dataset-id",
				MaxBillingTier:   3,
				MaxBytesBilled:   5,
			},
			want: func() *bq.Job {
				j := defaultQueryJob()
				tier := int64(3)
				j.Configuration.Query.MaximumBillingTier = &tier
				j.Configuration.Query.MaximumBytesBilled = 5
				return j
			}(),
		},
		{
			dst: c.Dataset("dataset-id").Table("table-id"),
			src: &QueryConfig{
				Q:                "query string",
				DefaultProjectID: "def-project-id",
				DefaultDatasetID: "def-dataset-id",
				MaxBytesBilled:   -1,
			},
			want: defaultQueryJob(),
		},
		{
			dst: c.Dataset("dataset-id").Table("table-id"),
			src: &QueryConfig{
				Q:                "query string",
				DefaultProjectID: "def-project-id",
				DefaultDatasetID: "def-dataset-id",
				UseStandardSQL:   true,
			},
			want: defaultQueryJob(),
		},
		{
			dst: c.Dataset("dataset-id").Table("table-id"),
			src: &QueryConfig{
				Q:                "query string",
				DefaultProjectID: "def-project-id",
				DefaultDatasetID: "def-dataset-id",
				UseLegacySQL:     true,
			},
			want: func() *bq.Job {
				j := defaultQueryJob()
				j.Configuration.Query.UseLegacySql = true
				j.Configuration.Query.ForceSendFields = nil
				return j
			}(),
		},
	}
	for i, tc := range testCases {
		query := c.Query("")
		query.JobIDConfig = tc.jobIDConfig
		query.QueryConfig = *tc.src
		query.Dst = tc.dst
		got, err := query.newJob()
		if err != nil {
			t.Errorf("#%d: err calling query: %v", i, err)
			continue
		}
		checkJob(t, i, got, tc.want)
	}
}

func TestConfiguringQuery(t *testing.T) {
	c := &Client{
		projectID: "project-id",
	}

	query := c.Query("q")
	query.JobID = "ajob"
	query.DefaultProjectID = "def-project-id"
	query.DefaultDatasetID = "def-dataset-id"
	// Note: Other configuration fields are tested in other tests above.
	// A lot of that can be consolidated once Client.Copy is gone.

	want := &bq.Job{
		Configuration: &bq.JobConfiguration{
			Query: &bq.JobConfigurationQuery{
				Query: "q",
				DefaultDataset: &bq.DatasetReference{
					ProjectId: "def-project-id",
					DatasetId: "def-dataset-id",
				},
				UseLegacySql:    false,
				ForceSendFields: []string{"UseLegacySql"},
			},
		},
		JobReference: &bq.JobReference{
			JobId:     "ajob",
			ProjectId: "project-id",
		},
	}

	got, err := query.newJob()
	if err != nil {
		t.Fatalf("err calling Query.newJob: %v", err)
	}
	if diff := testutil.Diff(got, want); diff != "" {
		t.Errorf("querying: -got +want:\n%s", diff)
	}
}

func TestQueryLegacySQL(t *testing.T) {
	c := &Client{projectID: "project-id"}
	q := c.Query("q")
	q.UseStandardSQL = true
	q.UseLegacySQL = true
	_, err := q.newJob()
	if err == nil {
		t.Error("UseStandardSQL and UseLegacySQL: got nil, want error")
	}
	q = c.Query("q")
	q.Parameters = []QueryParameter{{Name: "p", Value: 3}}
	q.UseLegacySQL = true
	_, err = q.newJob()
	if err == nil {
		t.Error("Parameters and UseLegacySQL: got nil, want error")
	}
}
