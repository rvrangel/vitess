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

package vindexes

import (
	"context"
	"encoding/json"
	"fmt"

	"vitess.io/vitess/go/sqltypes"
	"vitess.io/vitess/go/vt/key"
	topodatapb "vitess.io/vitess/go/vt/proto/topodata"
	vtgatepb "vitess.io/vitess/go/vt/proto/vtgate"
)

const (
	lookupParamNoVerify  = "no_verify"
	lookupParamWriteOnly = "write_only"
)

var (
	_ SingleColumn    = (*LookupUnique)(nil)
	_ Lookup          = (*LookupUnique)(nil)
	_ LookupPlanable  = (*LookupUnique)(nil)
	_ ParamValidating = (*LookupUnique)(nil)
	_ SingleColumn    = (*LookupNonUnique)(nil)
	_ Lookup          = (*LookupNonUnique)(nil)
	_ LookupPlanable  = (*LookupNonUnique)(nil)
	_ ParamValidating = (*LookupNonUnique)(nil)

	lookupParams = append(
		append(make([]string, 0), lookupCommonParams...),
		lookupParamNoVerify,
		lookupParamWriteOnly,
	)
)

func init() {
	Register("lookup", newLookup)
	Register("lookup_unique", newLookupUnique)
}

// LookupNonUnique defines a vindex that uses a lookup table and create a mapping between from ids and KeyspaceId.
// It's NonUnique and a Lookup.
type LookupNonUnique struct {
	name          string
	writeOnly     bool
	noVerify      bool
	lkp           lookupInternal
	unknownParams []string
}

func (ln *LookupNonUnique) GetCommitOrder() vtgatepb.CommitOrder {
	return vtgatepb.CommitOrder_NORMAL
}

func (ln *LookupNonUnique) AllowBatch() bool {
	return ln.lkp.BatchLookup
}

func (ln *LookupNonUnique) AutoCommitEnabled() bool {
	return ln.lkp.Autocommit
}

// String returns the name of the vindex.
func (ln *LookupNonUnique) String() string {
	return ln.name
}

// Cost returns the cost of this vindex as 20.
func (ln *LookupNonUnique) Cost() int {
	return 20
}

// IsUnique returns false since the Vindex is non unique.
func (ln *LookupNonUnique) IsUnique() bool {
	return false
}

// NeedsVCursor satisfies the Vindex interface.
func (ln *LookupNonUnique) NeedsVCursor() bool {
	return true
}

// Map can map ids to key.ShardDestination objects.
func (ln *LookupNonUnique) Map(ctx context.Context, vcursor VCursor, ids []sqltypes.Value) ([]key.ShardDestination, error) {
	out := make([]key.ShardDestination, 0, len(ids))
	if ln.writeOnly {
		for range ids {
			out = append(out, key.DestinationKeyRange{KeyRange: &topodatapb.KeyRange{}})
		}
		return out, nil
	}

	// if ignore_nulls is set and the query is about single null value, then fallback to all shards
	if len(ids) == 1 && ids[0].IsNull() && ln.lkp.IgnoreNulls {
		for range ids {
			out = append(out, key.DestinationKeyRange{KeyRange: &topodatapb.KeyRange{}})
		}
		return out, nil
	}

	results, err := ln.lkp.Lookup(ctx, vcursor, ids, vtgatepb.CommitOrder_NORMAL)
	if err != nil {
		return nil, err
	}

	return ln.MapResult(ids, results)
}

// MapResult implements the LookupPlanable interface
func (ln *LookupNonUnique) MapResult(ids []sqltypes.Value, results []*sqltypes.Result) ([]key.ShardDestination, error) {
	out := make([]key.ShardDestination, 0, len(ids))
	if ln.writeOnly {
		for range ids {
			out = append(out, key.DestinationKeyRange{KeyRange: &topodatapb.KeyRange{}})
		}
		return out, nil
	}
	for _, result := range results {
		if len(result.Rows) == 0 {
			out = append(out, key.DestinationNone{})
			continue
		}
		ksids := make([][]byte, 0, len(result.Rows))
		for _, row := range result.Rows {
			rowBytes, err := row[0].ToBytes()
			if err != nil {
				return nil, err
			}
			ksids = append(ksids, rowBytes)
		}
		out = append(out, key.DestinationKeyspaceIDs(ksids))
	}
	return out, nil
}

// Verify returns true if ids maps to ksids.
func (ln *LookupNonUnique) Verify(ctx context.Context, vcursor VCursor, ids []sqltypes.Value, ksids [][]byte) ([]bool, error) {
	if ln.writeOnly || ln.noVerify {
		out := make([]bool, len(ids))
		for i := range ids {
			out[i] = true
		}
		return out, nil
	}
	return ln.lkp.Verify(ctx, vcursor, ids, ksidsToValues(ksids))
}

// Create reserves the id by inserting it into the vindex table.
func (ln *LookupNonUnique) Create(ctx context.Context, vcursor VCursor, rowsColValues [][]sqltypes.Value, ksids [][]byte, ignoreMode bool) error {
	return ln.lkp.Create(ctx, vcursor, rowsColValues, ksidsToValues(ksids), ignoreMode)
}

// Delete deletes the entry from the vindex table.
func (ln *LookupNonUnique) Delete(ctx context.Context, vcursor VCursor, rowsColValues [][]sqltypes.Value, ksid []byte) error {
	return ln.lkp.Delete(ctx, vcursor, rowsColValues, sqltypes.MakeTrusted(sqltypes.VarBinary, ksid), vtgatepb.CommitOrder_NORMAL)
}

// Update updates the entry in the vindex table.
func (ln *LookupNonUnique) Update(ctx context.Context, vcursor VCursor, oldValues []sqltypes.Value, ksid []byte, newValues []sqltypes.Value) error {
	return ln.lkp.Update(ctx, vcursor, oldValues, ksid, sqltypes.MakeTrusted(sqltypes.VarBinary, ksid), newValues)
}

// MarshalJSON returns a JSON representation of LookupHash.
func (ln *LookupNonUnique) MarshalJSON() ([]byte, error) {
	return json.Marshal(ln.lkp)
}

// IsBackfilling implements the LookupBackfill interface
func (ln *LookupNonUnique) IsBackfilling() bool {
	return ln.writeOnly
}

// Query implements the LookupPlanable interface
func (ln *LookupNonUnique) Query() (selQuery string, arguments []string) {
	return ln.lkp.query()
}

// UnknownParams implements the ParamValidating interface.
func (ln *LookupNonUnique) UnknownParams() []string {
	return ln.unknownParams
}

// newLookup creates a LookupNonUnique vindex.
// The supplied map has the following required fields:
//
//	table: name of the backing table. It can be qualified by the keyspace.
//	from: list of columns in the table that have the 'from' values of the lookup vindex.
//	to: The 'to' column name of the table.
//
// The following fields are optional:
//
//	autocommit: setting this to "true" will cause inserts to upsert and deletes to be ignored.
//	write_only: in this mode, Map functions return the full keyrange causing a full scatter.
//	no_verify: in this mode, Verify will always succeed.
func newLookup(name string, m map[string]string) (Vindex, error) {
	lookup := &LookupNonUnique{
		name:          name,
		unknownParams: FindUnknownParams(m, lookupParams),
	}

	cc, err := parseCommonConfig(m)
	if err != nil {
		return nil, err
	}
	lookup.writeOnly, err = boolFromMap(m, lookupParamWriteOnly)
	if err != nil {
		return nil, err
	}

	lookup.noVerify, err = boolFromMap(m, lookupParamNoVerify)
	if err != nil {
		return nil, err
	}

	// if autocommit is on for non-unique lookup, upsert should also be on.
	upsert := cc.autocommit || cc.multiShardAutocommit
	if err := lookup.lkp.Init(m, cc.autocommit, upsert, cc.multiShardAutocommit); err != nil {
		return nil, err
	}
	return lookup, nil
}

func ksidsToValues(ksids [][]byte) []sqltypes.Value {
	values := make([]sqltypes.Value, 0, len(ksids))
	for _, ksid := range ksids {
		values = append(values, sqltypes.MakeTrusted(sqltypes.VarBinary, ksid))
	}
	return values
}

// ====================================================================

// LookupUnique defines a vindex that uses a lookup table.
// The table is expected to define the id column as unique. It's
// Unique and a Lookup.
type LookupUnique struct {
	name          string
	writeOnly     bool
	noVerify      bool
	lkp           lookupInternal
	unknownParams []string
}

func (lu *LookupUnique) GetCommitOrder() vtgatepb.CommitOrder {
	return vtgatepb.CommitOrder_NORMAL
}

func (lu *LookupUnique) AllowBatch() bool {
	return lu.lkp.BatchLookup
}

func (lu *LookupUnique) AutoCommitEnabled() bool {
	return lu.lkp.Autocommit
}

// newLookupUnique creates a LookupUnique vindex.
// The supplied map has the following required fields:
//
//	table: name of the backing table. It can be qualified by the keyspace.
//	from: list of columns in the table that have the 'from' values of the lookup vindex.
//	to: The 'to' column name of the table.
//
// The following fields are optional:
//
//	autocommit: setting this to "true" will cause deletes to be ignored.
//	write_only: in this mode, Map functions return the full keyrange causing a full scatter.
func newLookupUnique(name string, m map[string]string) (Vindex, error) {
	lu := &LookupUnique{
		name:          name,
		unknownParams: FindUnknownParams(m, lookupParams),
	}

	cc, err := parseCommonConfig(m)
	if err != nil {
		return nil, err
	}
	lu.writeOnly, err = boolFromMap(m, lookupParamWriteOnly)
	if err != nil {
		return nil, err
	}

	lu.noVerify, err = boolFromMap(m, lookupParamNoVerify)
	if err != nil {
		return nil, err
	}

	// Don't allow upserts for unique vindexes.
	if err := lu.lkp.Init(m, cc.autocommit, false /* upsert */, cc.multiShardAutocommit); err != nil {
		return nil, err
	}
	return lu, nil
}

// String returns the name of the vindex.
func (lu *LookupUnique) String() string {
	return lu.name
}

// Cost returns the cost of this vindex as 10.
func (lu *LookupUnique) Cost() int {
	return 10
}

// IsUnique returns true since the Vindex is unique.
func (lu *LookupUnique) IsUnique() bool {
	return true
}

// NeedsVCursor satisfies the Vindex interface.
func (lu *LookupUnique) NeedsVCursor() bool {
	return true
}

// Map can map ids to key.ShardDestination objects.
func (lu *LookupUnique) Map(ctx context.Context, vcursor VCursor, ids []sqltypes.Value) ([]key.ShardDestination, error) {
	if lu.writeOnly {
		out := make([]key.ShardDestination, 0, len(ids))
		for range ids {
			out = append(out, key.DestinationKeyRange{KeyRange: &topodatapb.KeyRange{}})
		}
		return out, nil
	}
	results, err := lu.lkp.Lookup(ctx, vcursor, ids, vtgatepb.CommitOrder_NORMAL)
	if err != nil {
		return nil, err
	}
	return lu.MapResult(ids, results)
}

func (lu *LookupUnique) MapResult(ids []sqltypes.Value, results []*sqltypes.Result) ([]key.ShardDestination, error) {
	out := make([]key.ShardDestination, 0, len(ids))
	for i, result := range results {
		switch len(result.Rows) {
		case 0:
			out = append(out, key.DestinationNone{})
		case 1:
			rowBytes, err := result.Rows[0][0].ToBytes()
			if err != nil {
				return nil, err
			}
			out = append(out, key.DestinationKeyspaceID(rowBytes))
		default:
			return nil, fmt.Errorf("Lookup.Map: unexpected multiple results from vindex %s: %v", lu.lkp.Table, ids[i])
		}
	}
	return out, nil
}

// Verify returns true if ids maps to ksids.
func (lu *LookupUnique) Verify(ctx context.Context, vcursor VCursor, ids []sqltypes.Value, ksids [][]byte) ([]bool, error) {
	if lu.writeOnly || lu.noVerify {
		out := make([]bool, len(ids))
		for i := range ids {
			out[i] = true
		}
		return out, nil
	}
	return lu.lkp.Verify(ctx, vcursor, ids, ksidsToValues(ksids))
}

// Create reserves the id by inserting it into the vindex table.
func (lu *LookupUnique) Create(ctx context.Context, vcursor VCursor, rowsColValues [][]sqltypes.Value, ksids [][]byte, ignoreMode bool) error {
	return lu.lkp.Create(ctx, vcursor, rowsColValues, ksidsToValues(ksids), ignoreMode)
}

// Update updates the entry in the vindex table.
func (lu *LookupUnique) Update(ctx context.Context, vcursor VCursor, oldValues []sqltypes.Value, ksid []byte, newValues []sqltypes.Value) error {
	return lu.lkp.Update(ctx, vcursor, oldValues, ksid, sqltypes.MakeTrusted(sqltypes.VarBinary, ksid), newValues)
}

// Delete deletes the entry from the vindex table.
func (lu *LookupUnique) Delete(ctx context.Context, vcursor VCursor, rowsColValues [][]sqltypes.Value, ksid []byte) error {
	return lu.lkp.Delete(ctx, vcursor, rowsColValues, sqltypes.MakeTrusted(sqltypes.VarBinary, ksid), vtgatepb.CommitOrder_NORMAL)
}

// MarshalJSON returns a JSON representation of LookupUnique.
func (lu *LookupUnique) MarshalJSON() ([]byte, error) {
	return json.Marshal(lu.lkp)
}

// IsBackfilling implements the LookupBackfill interface
func (lu *LookupUnique) IsBackfilling() bool {
	return lu.writeOnly
}

func (lu *LookupUnique) LookupQuery() (string, error) {
	return lu.lkp.sel, nil
}

func (lu *LookupUnique) Query() (string, []string) {
	return lu.lkp.query()
}

// UnknownParams implements the ParamValidating interface.
func (ln *LookupUnique) UnknownParams() []string {
	return ln.unknownParams
}
