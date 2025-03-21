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

package schemadiff

import (
	"vitess.io/vitess/go/mysql/collations"
	"vitess.io/vitess/go/vt/key"
	topodatapb "vitess.io/vitess/go/vt/proto/topodata"
	vschemapb "vitess.io/vitess/go/vt/proto/vschema"
	"vitess.io/vitess/go/vt/sqlparser"
	"vitess.io/vitess/go/vt/vtenv"
	"vitess.io/vitess/go/vt/vtgate/semantics"
	"vitess.io/vitess/go/vt/vtgate/vindexes"
)

// semanticKS is a bogus keyspace, used for consistency purposes. The name is not important
var semanticKS = &vindexes.Keyspace{
	Name:    "ks",
	Sharded: false,
}

var _ semantics.SchemaInformation = (*declarativeSchemaInformation)(nil)

// declarativeSchemaInformation is a utility wrapper around FakeSI, and adds a few utility functions
// to make it more simple and accessible to schemadiff's logic.
type declarativeSchemaInformation struct {
	Tables map[string]*vindexes.BaseTable
	env    *Environment
}

func newDeclarativeSchemaInformation(env *Environment) *declarativeSchemaInformation {
	return &declarativeSchemaInformation{
		Tables: make(map[string]*vindexes.BaseTable),
		env:    env,
	}
}

// FindTableOrVindex implements the SchemaInformation interface
func (si *declarativeSchemaInformation) FindTableOrVindex(tablename sqlparser.TableName) (*vindexes.BaseTable, vindexes.Vindex, string, topodatapb.TabletType, key.ShardDestination, error) {
	table := si.Tables[tablename.Name.String()]
	return table, nil, "", 0, nil, nil
}

func (si *declarativeSchemaInformation) ConnCollation() collations.ID {
	return si.env.DefaultColl
}

func (si *declarativeSchemaInformation) Environment() *vtenv.Environment {
	return si.env.Environment
}

func (si *declarativeSchemaInformation) ForeignKeyMode(keyspace string) (vschemapb.Keyspace_ForeignKeyMode, error) {
	return vschemapb.Keyspace_unmanaged, nil
}

func (si *declarativeSchemaInformation) KeyspaceError(keyspace string) error {
	return nil
}

func (si *declarativeSchemaInformation) GetAggregateUDFs() []string {
	return nil
}

func (si *declarativeSchemaInformation) GetForeignKeyChecksState() *bool {
	return nil
}

// FindMirrorRule implements semantics.SchemaInformation.
func (si *declarativeSchemaInformation) FindMirrorRule(tablename sqlparser.TableName) (*vindexes.MirrorRule, error) {
	return nil, nil
}

// addTable adds a fake table with an empty column list
func (si *declarativeSchemaInformation) addTable(tableName string) {
	tbl := &vindexes.BaseTable{
		Name:                    sqlparser.NewIdentifierCS(tableName),
		Columns:                 []vindexes.Column{},
		ColumnListAuthoritative: true,
		Keyspace:                semanticKS,
	}
	si.Tables[tableName] = tbl
}

// addColumn adds a fake column with no type. It assumes the table already exists
func (si *declarativeSchemaInformation) addColumn(tableName string, columnName string) {
	col := &vindexes.Column{
		Name: sqlparser.NewIdentifierCI(columnName),
	}
	si.Tables[tableName].Columns = append(si.Tables[tableName].Columns, *col)
}
