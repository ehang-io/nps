// Copyright 2014 beego Author. All Rights Reserved.
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

package migration

import (
	"fmt"

	"github.com/cnlh/nps/vender/github.com/astaxie/beego"
)

// Index struct defines the structure of Index Columns
type Index struct {
	Name string
}

// Unique struct defines a single unique key combination
type Unique struct {
	Definition string
	Columns    []*Column
}

//Column struct defines a single column of a table
type Column struct {
	Name     string
	Inc      string
	Null     string
	Default  string
	Unsign   string
	DataType string
	remove   bool
	Modify   bool
}

// Foreign struct defines a single foreign relationship
type Foreign struct {
	ForeignTable  string
	ForeignColumn string
	OnDelete      string
	OnUpdate      string
	Column
}

// RenameColumn struct allows renaming of columns
type RenameColumn struct {
	OldName     string
	OldNull     string
	OldDefault  string
	OldUnsign   string
	OldDataType string
	NewName     string
	Column
}

// CreateTable creates the table on system
func (m *Migration) CreateTable(tablename, engine, charset string, p ...func()) {
	m.TableName = tablename
	m.Engine = engine
	m.Charset = charset
	m.ModifyType = "create"
}

// AlterTable set the ModifyType to alter
func (m *Migration) AlterTable(tablename string) {
	m.TableName = tablename
	m.ModifyType = "alter"
}

// NewCol creates a new standard column and attaches it to m struct
func (m *Migration) NewCol(name string) *Column {
	col := &Column{Name: name}
	m.AddColumns(col)
	return col
}

//PriCol creates a new primary column and attaches it to m struct
func (m *Migration) PriCol(name string) *Column {
	col := &Column{Name: name}
	m.AddColumns(col)
	m.AddPrimary(col)
	return col
}

//UniCol creates / appends columns to specified unique key and attaches it to m struct
func (m *Migration) UniCol(uni, name string) *Column {
	col := &Column{Name: name}
	m.AddColumns(col)

	uniqueOriginal := &Unique{}

	for _, unique := range m.Uniques {
		if unique.Definition == uni {
			unique.AddColumnsToUnique(col)
			uniqueOriginal = unique
		}
	}
	if uniqueOriginal.Definition == "" {
		unique := &Unique{Definition: uni}
		unique.AddColumnsToUnique(col)
		m.AddUnique(unique)
	}

	return col
}

//ForeignCol creates a new foreign column and returns the instance of column
func (m *Migration) ForeignCol(colname, foreigncol, foreigntable string) (foreign *Foreign) {

	foreign = &Foreign{ForeignColumn: foreigncol, ForeignTable: foreigntable}
	foreign.Name = colname
	m.AddForeign(foreign)
	return foreign
}

//SetOnDelete sets the on delete of foreign
func (foreign *Foreign) SetOnDelete(del string) *Foreign {
	foreign.OnDelete = "ON DELETE" + del
	return foreign
}

//SetOnUpdate sets the on update of foreign
func (foreign *Foreign) SetOnUpdate(update string) *Foreign {
	foreign.OnUpdate = "ON UPDATE" + update
	return foreign
}

//Remove marks the columns to be removed.
//it allows reverse m to create the column.
func (c *Column) Remove() {
	c.remove = true
}

//SetAuto enables auto_increment of column (can be used once)
func (c *Column) SetAuto(inc bool) *Column {
	if inc {
		c.Inc = "auto_increment"
	}
	return c
}

//SetNullable sets the column to be null
func (c *Column) SetNullable(null bool) *Column {
	if null {
		c.Null = ""

	} else {
		c.Null = "NOT NULL"
	}
	return c
}

//SetDefault sets the default value, prepend with "DEFAULT "
func (c *Column) SetDefault(def string) *Column {
	c.Default = "DEFAULT " + def
	return c
}

//SetUnsigned sets the column to be unsigned int
func (c *Column) SetUnsigned(unsign bool) *Column {
	if unsign {
		c.Unsign = "UNSIGNED"
	}
	return c
}

//SetDataType sets the dataType of the column
func (c *Column) SetDataType(dataType string) *Column {
	c.DataType = dataType
	return c
}

//SetOldNullable allows reverting to previous nullable on reverse ms
func (c *RenameColumn) SetOldNullable(null bool) *RenameColumn {
	if null {
		c.OldNull = ""

	} else {
		c.OldNull = "NOT NULL"
	}
	return c
}

//SetOldDefault allows reverting to previous default on reverse ms
func (c *RenameColumn) SetOldDefault(def string) *RenameColumn {
	c.OldDefault = def
	return c
}

//SetOldUnsigned allows reverting to previous unsgined on reverse ms
func (c *RenameColumn) SetOldUnsigned(unsign bool) *RenameColumn {
	if unsign {
		c.OldUnsign = "UNSIGNED"
	}
	return c
}

//SetOldDataType allows reverting to previous datatype on reverse ms
func (c *RenameColumn) SetOldDataType(dataType string) *RenameColumn {
	c.OldDataType = dataType
	return c
}

//SetPrimary adds the columns to the primary key (can only be used any number of times in only one m)
func (c *Column) SetPrimary(m *Migration) *Column {
	m.Primary = append(m.Primary, c)
	return c
}

//AddColumnsToUnique adds the columns to Unique Struct
func (unique *Unique) AddColumnsToUnique(columns ...*Column) *Unique {

	unique.Columns = append(unique.Columns, columns...)

	return unique
}

//AddColumns adds columns to m struct
func (m *Migration) AddColumns(columns ...*Column) *Migration {

	m.Columns = append(m.Columns, columns...)

	return m
}

//AddPrimary adds the column to primary in m struct
func (m *Migration) AddPrimary(primary *Column) *Migration {
	m.Primary = append(m.Primary, primary)
	return m
}

//AddUnique adds the column to unique in m struct
func (m *Migration) AddUnique(unique *Unique) *Migration {
	m.Uniques = append(m.Uniques, unique)
	return m
}

//AddForeign adds the column to foreign in m struct
func (m *Migration) AddForeign(foreign *Foreign) *Migration {
	m.Foreigns = append(m.Foreigns, foreign)
	return m
}

//AddIndex adds the column to index in m struct
func (m *Migration) AddIndex(index *Index) *Migration {
	m.Indexes = append(m.Indexes, index)
	return m
}

//RenameColumn allows renaming of columns
func (m *Migration) RenameColumn(from, to string) *RenameColumn {
	rename := &RenameColumn{OldName: from, NewName: to}
	m.Renames = append(m.Renames, rename)
	return rename
}

//GetSQL returns the generated sql depending on ModifyType
func (m *Migration) GetSQL() (sql string) {
	sql = ""
	switch m.ModifyType {
	case "create":
		{
			sql += fmt.Sprintf("CREATE TABLE `%s` (", m.TableName)
			for index, column := range m.Columns {
				sql += fmt.Sprintf("\n `%s` %s %s %s %s %s", column.Name, column.DataType, column.Unsign, column.Null, column.Inc, column.Default)
				if len(m.Columns) > index+1 {
					sql += ","
				}
			}

			if len(m.Primary) > 0 {
				sql += fmt.Sprintf(",\n PRIMARY KEY( ")
			}
			for index, column := range m.Primary {
				sql += fmt.Sprintf(" `%s`", column.Name)
				if len(m.Primary) > index+1 {
					sql += ","
				}

			}
			if len(m.Primary) > 0 {
				sql += fmt.Sprintf(")")
			}

			for _, unique := range m.Uniques {
				sql += fmt.Sprintf(",\n UNIQUE KEY `%s`( ", unique.Definition)
				for index, column := range unique.Columns {
					sql += fmt.Sprintf(" `%s`", column.Name)
					if len(unique.Columns) > index+1 {
						sql += ","
					}
				}
				sql += fmt.Sprintf(")")
			}
			for _, foreign := range m.Foreigns {
				sql += fmt.Sprintf(",\n `%s` %s %s %s %s %s", foreign.Name, foreign.DataType, foreign.Unsign, foreign.Null, foreign.Inc, foreign.Default)
				sql += fmt.Sprintf(",\n KEY  `%s_%s_foreign`(`%s`),", m.TableName, foreign.Column.Name, foreign.Column.Name)
				sql += fmt.Sprintf("\n CONSTRAINT `%s_%s_foreign` FOREIGN KEY (`%s`) REFERENCES `%s` (`%s`)  %s %s", m.TableName, foreign.Column.Name, foreign.Column.Name, foreign.ForeignTable, foreign.ForeignColumn, foreign.OnDelete, foreign.OnUpdate)

			}
			sql += fmt.Sprintf(")ENGINE=%s DEFAULT CHARSET=%s;", m.Engine, m.Charset)
			break
		}
	case "alter":
		{
			sql += fmt.Sprintf("ALTER TABLE `%s` ", m.TableName)
			for index, column := range m.Columns {
				if !column.remove {
					beego.BeeLogger.Info("col")
					sql += fmt.Sprintf("\n ADD `%s` %s %s %s %s %s", column.Name, column.DataType, column.Unsign, column.Null, column.Inc, column.Default)
				} else {
					sql += fmt.Sprintf("\n DROP COLUMN `%s`", column.Name)
				}

				if len(m.Columns) > index+1 {
					sql += ","
				}
			}
			for index, column := range m.Renames {
				sql += fmt.Sprintf("CHANGE COLUMN `%s` `%s` %s %s %s %s %s", column.OldName, column.NewName, column.DataType, column.Unsign, column.Null, column.Inc, column.Default)
				if len(m.Renames) > index+1 {
					sql += ","
				}
			}

			for index, foreign := range m.Foreigns {
				sql += fmt.Sprintf("ADD `%s` %s %s %s %s %s", foreign.Name, foreign.DataType, foreign.Unsign, foreign.Null, foreign.Inc, foreign.Default)
				sql += fmt.Sprintf(",\n ADD KEY  `%s_%s_foreign`(`%s`)", m.TableName, foreign.Column.Name, foreign.Column.Name)
				sql += fmt.Sprintf(",\n ADD CONSTRAINT  `%s_%s_foreign` FOREIGN KEY (`%s`) REFERENCES `%s` (`%s`)  %s %s", m.TableName, foreign.Column.Name, foreign.Column.Name, foreign.ForeignTable, foreign.ForeignColumn, foreign.OnDelete, foreign.OnUpdate)
				if len(m.Foreigns) > index+1 {
					sql += ","
				}
			}
			sql += ";"

			break
		}
	case "reverse":
		{

			sql += fmt.Sprintf("ALTER TABLE `%s`", m.TableName)
			for index, column := range m.Columns {
				if column.remove {
					sql += fmt.Sprintf("\n ADD `%s` %s %s %s %s %s", column.Name, column.DataType, column.Unsign, column.Null, column.Inc, column.Default)
				} else {
					sql += fmt.Sprintf("\n DROP COLUMN `%s`", column.Name)
				}
				if len(m.Columns) > index+1 {
					sql += ","
				}
			}

			if len(m.Primary) > 0 {
				sql += fmt.Sprintf("\n DROP PRIMARY KEY,")
			}

			for index, unique := range m.Uniques {
				sql += fmt.Sprintf("\n DROP KEY `%s`", unique.Definition)
				if len(m.Uniques) > index+1 {
					sql += ","
				}

			}
			for index, column := range m.Renames {
				sql += fmt.Sprintf("\n CHANGE COLUMN `%s` `%s` %s %s %s %s", column.NewName, column.OldName, column.OldDataType, column.OldUnsign, column.OldNull, column.OldDefault)
				if len(m.Renames) > index+1 {
					sql += ","
				}
			}

			for _, foreign := range m.Foreigns {
				sql += fmt.Sprintf("\n DROP KEY  `%s_%s_foreign`", m.TableName, foreign.Column.Name)
				sql += fmt.Sprintf(",\n DROP FOREIGN KEY  `%s_%s_foreign`", m.TableName, foreign.Column.Name)
				sql += fmt.Sprintf(",\n DROP COLUMN `%s`", foreign.Name)
			}
			sql += ";"
		}
	case "delete":
		{
			sql += fmt.Sprintf("DROP TABLE IF EXISTS `%s`;", m.TableName)
		}
	}

	return
}
