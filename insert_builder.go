package dbr

import (
	"database/sql"
	"reflect"
)

type PreInsertHook func(*InsertBuilder, interface{}) *InsertBuilder

type InsertBuilder struct {
	runner
	EventReceiver
	Dialect Dialect

	RecordID reflect.Value

	*InsertStmt

	PreInsertHooks []PreInsertHook
}

func (sess *Session) InsertInto(table string) *InsertBuilder {
	return &InsertBuilder{
		runner:        sess,
		EventReceiver: sess,
		Dialect:       sess.Dialect,
		InsertStmt:    InsertInto(table),
	}
}

func (tx *Tx) InsertInto(table string) *InsertBuilder {
	return &InsertBuilder{
		runner:        tx,
		EventReceiver: tx,
		Dialect:       tx.Dialect,
		InsertStmt:    InsertInto(table),
	}
}

func (sess *Session) InsertBySql(query string, value ...interface{}) *InsertBuilder {
	return &InsertBuilder{
		runner:        sess,
		EventReceiver: sess,
		Dialect:       sess.Dialect,
		InsertStmt:    InsertBySql(query, value...),
	}
}

func (tx *Tx) InsertBySql(query string, value ...interface{}) *InsertBuilder {
	return &InsertBuilder{
		runner:        tx,
		EventReceiver: tx,
		Dialect:       tx.Dialect,
		InsertStmt:    InsertBySql(query, value...),
	}
}

func (b *InsertBuilder) ToSql() (string, []interface{}) {
	buf := NewBuffer()
	err := b.Build(b.Dialect, buf)
	if err != nil {
		panic(err)
	}
	return buf.String(), buf.Value()
}

func (b *InsertBuilder) Pair(column string, value interface{}) *InsertBuilder {
	b.Column = append(b.Column, column)
	switch len(b.Value) {
	case 0:
		b.InsertStmt.Values(value)
	case 1:
		b.Value[0] = append(b.Value[0], value)
	default:
		panic("pair only allows one record to insert")
	}
	return b
}

func (b *InsertBuilder) Exec() (sql.Result, error) {
	result, err := exec(b.runner, b.EventReceiver, b, b.Dialect)
	if err != nil {
		return nil, err
	}

	if b.RecordID.IsValid() {
		if id, err := result.LastInsertId(); err == nil {
			b.RecordID.SetInt(id)
		}
	}

	return result, nil
}

func (b *InsertBuilder) Columns(column ...string) *InsertBuilder {
	b.InsertStmt.Columns(column...)
	return b
}

func (b *InsertBuilder) Record(structValue interface{}) *InsertBuilder {
	v := reflect.Indirect(reflect.ValueOf(structValue))
	if v.Kind() == reflect.Struct && v.CanSet() {
		// ID is recommended by golint here
		for _, name := range []string{"Id", "ID"} {
			field := v.FieldByName(name)
			if field.IsValid() && field.Kind() == reflect.Int64 {
				b.RecordID = field
				break
			}
		}
	}

	if len(b.PreInsertHooks) > 0 {
		for _, f := range b.PreInsertHooks {
			f(b, structValue)
		}
	}

	b.InsertStmt.Record(structValue)
	return b
}

func (b *InsertBuilder) Records(structValues interface{}) *InsertBuilder {
	v := reflect.ValueOf(structValues)
	s := reflect.Value{}
	switch v.Kind() {
	case reflect.Ptr:
		s = v.Elem()
		if s.Kind() != reflect.Slice {
			return b
		}
	case reflect.Slice:
		s = v
	default:
		return b
	}

	//t := v.Type().Elem()

	sLen := s.Len()

	for i := 0; i < sLen; i++ {
		structValue := s.Index(i).Addr().Interface()

		if len(b.PreInsertHooks) > 0 {
			for _, f := range b.PreInsertHooks {
				f(b, structValue)
			}
		}

		b.InsertStmt.Record(structValue)
	}

	return b
}

func (b *InsertBuilder) Values(value ...interface{}) *InsertBuilder {
	b.InsertStmt.Values(value...)
	return b
}

func (b *InsertBuilder) SetPreInsertHooks(f ...PreInsertHook) {
	b.PreInsertHooks = append(b.PreInsertHooks, f...)
}

func (b *InsertBuilder) ClearPreInsertHooks() {
	b.PreInsertHooks = []PreInsertHook{}
}
