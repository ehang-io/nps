// Package migration enables you to generate migrations back and forth. It generates both migrations.
//
// //Creates a table
// m.CreateTable("tablename","InnoDB","utf8");
//
// //Alter a table
// m.AlterTable("tablename")
//
// Standard Column Methods
// * SetDataType
// * SetNullable
// * SetDefault
// * SetUnsigned (use only on integer types unless produces error)
//
// //Sets a primary column, multiple calls allowed, standard column methods available
// m.PriCol("id").SetAuto(true).SetNullable(false).SetDataType("INT(10)").SetUnsigned(true)
//
// //UniCol Can be used multiple times, allows standard Column methods. Use same "index" string to add to same index
// m.UniCol("index","column")
//
// //Standard Column Initialisation, can call .Remove() after NewCol("") on alter to remove
// m.NewCol("name").SetDataType("VARCHAR(255) COLLATE utf8_unicode_ci").SetNullable(false)
// m.NewCol("value").SetDataType("DOUBLE(8,2)").SetNullable(false)
//
// //Rename Columns , only use with Alter table, doesn't works with Create, prefix standard column methods with "Old" to
// //create a true reversible migration eg: SetOldDataType("DOUBLE(12,3)")
// m.RenameColumn("from","to")...
//
// //Foreign Columns, single columns are only supported, SetOnDelete & SetOnUpdate are available, call appropriately.
// //Supports standard column methods, automatic reverse.
// m.ForeignCol("local_col","foreign_col","foreign_table")
package migration
