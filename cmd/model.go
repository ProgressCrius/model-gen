package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

var model_str = `
package {{.PackageName}}

import (
	"sync"
	{{if or .HasTime .HasCache}}"time"{{end}}
{{if .HasPrimaryKey}}
	{{if .HasCache}}"context"
	"internal/cache/redis"
	"internal/conf"
	"libs/utils"{{end}}
	"internal/log"
{{end}}
	"libs/types"
	"{{.ModulePath}}/table"
)

var (
	{{lower .StructName}}Pool = sync.Pool{
		New: func() interface{} {
			return &{{.StructName}}{}
		},
	}
)

{{if and .HasCache .HasPrimaryKey}}
var (
	{{lower .StructName}}_cache     = redis.NewClient(conf.App.Mode,"{{lower .StructName}}").Expiration({{.CacheData}})
	{{lower .StructName}}_ids_cache = redis.NewClient(conf.App.Mode, "{{lower .StructName}}_ids").Expiration({{.CacheList}})
)

func init() {
	{{lower .StructName}}_cache.LoaderFunc(func(k interface{}) (interface{}, error) {
		id := utils.Interface2Uint64(k, 0)
		if id < 1 {
			return nil, InvalidKey
		}

		m := New{{.StructName}}()
		db := Db().Table(table.{{.StructName}}.TableName)
		has, e := db.Where(table.{{.StructName}}.PrimaryKey.Eq(),id).
			Get(m)
		if has {
			return m, nil
		}
		if e != nil {
			log.Logs.DBError(db, e)
		}
		return nil, e
	}).DeserializeModel(func() interface{} {
		return New{{.StructName}}()
	})
	//
	{{lower .StructName}}_ids_cache.LoaderFunc(func(k interface{}) (interface{}, error) {
		cond, ok := k.(table.ISqlBuilder)
		if !ok {
			return nil, InvalidKey
		}
		
		db := Db().Table(table.{{.StructName}}.TableName).
			Cols(table.{{.StructName}}.PrimaryKey.Name)
		if s, args := cond.GetWhere(); s != "" {
			db.Where(s, args...)
		}
		if s := cond.GetGroupBy(); s != "" {
			db.GroupBy(s)
		}
		if s := cond.GetHaving(); s != "" {
			db.Having(s)
		}
		if s := cond.GetOrderBy(); s != "" {
			db.OrderBy(s)
		}

		ids := make([]uint64, 0)
		e := db.Limit({{.CacheLimit}}).Find(&ids)
		if e != nil {
			log.Logs.DBError(db, e)
		}
		return ids, e
	}).DeserializeFunc(func(bean interface{}) (interface{}, error) {
		ids := make([]uint64, 0)
		e := utils.JSON.UnmarshalFromString(utils.Interface2String(bean), &ids)
		if e != nil {
			log.Logs.Error(e)
		}
		return ids, e
	})
}
{{end}}

func New{{.StructName}}() *{{.StructName}} {
	return {{lower .StructName}}Pool.Get().(*{{.StructName}})
}

//Free 
func (p *{{.StructName}}) Free() {
	{{range $key, $value := .Columns}}p.{{$key}} = {{getTypeValue $value}}				
	{{end}}
	{{lower .StructName}}Pool.Put(p)
}
{{if .HasPrimaryKey}}
//TableName
func (*{{.StructName}}) TableName() string {
	return table.{{.StructName}}.TableName
}

//Insert 新增一条数据
func (p *{{.StructName}}) Insert(x interface{}, cols ...string) (int64,error) {
	var (
		ok bool
		db *Session
	)
	
	if db, ok = x.(*Session); ok {
		db.Table(table.{{.StructName}}.TableName)
	} else {
		db = Db().Table(table.{{.StructName}}.TableName)
	}

	if len(cols) > 0 {
		db.Cols(cols...)
	}

	i64,e := db.InsertOne(p)
	if e != nil {
		log.Logs.DBError(db, e)
	}
{{if .HasCache}}
	if i64 > 0 {
		p.OnListChange()
	}
{{end}}
	return i64, e
}

//InsertBatch 批量新增数据
func (p *{{.StructName}}) InsertBatch(x interface{}, beans []interface{}, cols ...string) (int64, error) {
	var (
		ok bool
		db *Session
	)
	
	if db, ok = x.(*Session); ok {
		db.Table(table.{{.StructName}}.TableName)
	} else {
		db = Db().Table(table.{{.StructName}}.TableName)
	}

	if len(cols) > 0 {
		db.Cols(cols...)
	}

	i64, e := db.Insert(beans...)
	if e != nil {
		log.Logs.DBError(db, e)
	}
{{if .HasCache}}
	if i64 > 0 {
		p.OnListChange()
	}
{{end}}
	return i64, e
}

//Update 根据主键修改一条数据
func (p *{{.StructName}}) Update(x interface{}, id uint64, bean ...interface{}) (int64,error) {
	var (
		i64 int64
		e error
		ok bool
		db *Session
	)
	
	if db, ok = x.(*Session); ok {
		db.Table(table.{{.StructName}}.TableName)
	} else {
		db = Db().Table(table.{{.StructName}}.TableName)
	}


	db.Where(table.{{.StructName}}.PrimaryKey.Eq(),id)

	if len(bean) == 0 {
		i64,e = db.Update(p)
	} else {
		i64,e = db.Update(bean[0])
	}

	if e != nil {
		log.Logs.DBError(db, e)
	}
{{if .HasCache}}
	if i64 > 0 {
		p.OnChange(id)
	}
{{end}}
	return i64, e
}

//UpdateBatch 根据cond条件批量修改数据
func (p *{{.StructName}}) UpdateBatch(x interface{}, cond table.ISqlBuilder, bean ...interface{}) (int64, error) {
	var (
		i64 int64
		e   error
		ok bool
		db *Session
	)
	
	if db, ok = x.(*Session); ok {
		db.Table(table.{{.StructName}}.TableName)
	} else {
		db = Db().Table(table.{{.StructName}}.TableName)
	}

	if cond != nil {
		if s, args := cond.GetWhere(); s != "" {
			db.Where(s, args...)
		}
	}

	if len(bean) == 0 {
		i64, e = db.Update(p)
	} else {
		i64, e = db.Update(bean[0])
	}

	if e != nil {
		log.Logs.DBError(db, e)
	}
{{if .HasCache}}
	if i64 > 0 {
		p.OnBatchChange(cond)
	}
{{end}}
	return i64, e
}

//Delete 根据主键删除一条数据
func (p *{{.StructName}}) Delete(x interface{}, id uint64) (int64,error) {
	var (
		ok bool
		db *Session
	)
	
	if db, ok = x.(*Session); ok {
		db.Table(table.{{.StructName}}.TableName)
	} else {
		db = Db().Table(table.{{.StructName}}.TableName)
	}

	i64,e := db.Where(table.{{.StructName}}.PrimaryKey.Eq(),id).
		Delete(p)

	if e != nil {
		log.Logs.DBError(db, e)
	}
{{if .HasCache}}
	if i64 > 0 {
		p.OnChange(id)
	}
{{end}}
	return i64, e
}

//DeleteBatch 根据cond条件批量删除数据
func (p *{{.StructName}}) DeleteBatch(x interface{}, cond table.ISqlBuilder) (int64, error) {
	var (
		ok bool
		db *Session
	)
	
	if db, ok = x.(*Session); ok {
		db.Table(table.{{.StructName}}.TableName)
	} else {
		db = Db().Table(table.{{.StructName}}.TableName)
	}

	if cond != nil {
		if s, args := cond.GetWhere(); s != "" {
			db.Where(s, args...)
		}
	}
	i64, e := db.Delete(p)
	if e != nil {
		log.Logs.DBError(db, e)
	}
{{if .HasCache}}
	if i64 > 0 {
		p.OnBatchChange(cond)
	}
{{end}}
	return i64, e
}

//Get 根据主键从Cache中获取一条数据
func (p *{{.StructName}}) Get(x interface{},id uint64) (bool, error) {
{{if .HasCache}}
	cm, e := {{lower .StructName}}_cache.Get(context.TODO(), id)
	if e != nil {
		log.Logs.Error(e)
		return false, e
	}
	if val, ok := cm.(*{{.StructName}}); ok {
		*p = *val
		return ok, nil
	}

	log.Logs.Error(Err_Type)
	return false, e
{{else}}
	return p.GetNoCache(x,id)
{{end}}
}

//GetNoCache 根据主键从数据库中获取一条数据
func (p *{{.StructName}}) GetNoCache(x interface{},id uint64, cols ...table.TableField) (bool, error) {
	var (
		ok bool
		db *Session
	)
	
	if db, ok = x.(*Session); ok {
		db.Table(table.{{.StructName}}.TableName)
	} else {
		db = Db().Table(table.{{.StructName}}.TableName)
	}
	//
	if len(cols) > 0 {
		_cols := make([]string, 0, len(cols))
		for _, col := range cols {
			_cols = append(_cols, col.Name)
		}
		db.Cols(_cols...)
	}

	has, e := db.Where(table.{{.StructName}}.PrimaryKey.Eq(),id).
		Get(p)
	if e != nil {
		log.Logs.DBError(db, e)
	}
	return has,e
}

//IDs 根据cond条件从cache中获取主键列表
func (p *{{.StructName}}) IDs(x interface{}, cond table.ISqlBuilder, size, index int) ([]interface{}, error) {
{{if .HasCache}}
	if size == 0 {
		size = {{.CacheLimit}}
	}

	if index == 0 {
		index = 1
	}
	return {{lower .StructName}}_ids_cache.LGet(context.TODO(), cond, int64(size*(index-1)), int64(size*index))
{{else}}
	return p.IDsNoCache(x,cond,size,index)
{{end}}
}

//IDsNoCache 根据cond条件从数据库中获取主键列表
func (p *{{.StructName}}) IDsNoCache(x interface{}, cond table.ISqlBuilder, size, index int) ([]interface{}, error) {
	var (
		ok bool
		db *Session
	)
	
	if db, ok = x.(*Session); ok {
		db.Table(table.{{.StructName}}.TableName)
	} else {
		db = Db().Table(table.{{.StructName}}.TableName)
	}

	ids := make([]interface{}, 0)

	if cond != nil {
		if s, args := cond.GetWhere(); s != "" {
			db.Where(s, args...)
		}
		if s := cond.GetGroupBy(); s != "" {
			db.GroupBy(s)
		}
		if s := cond.GetHaving(); s != "" {
			db.Having(s)
		}
		if s := cond.GetOrderBy(); s != "" {
			db.OrderBy(s)
		}
	}

	if size > 0 {
		if index == 0 {
			index = 1
		}
		db.Limit(size, size*(index-1))
	}

	e := db.Find(&ids)
	if e != nil {
		log.Logs.DBError(db, e)
	}
	return ids,e
}

//Find 根据cond条件从cache中获取数据列表
func (p *{{.StructName}}) Find(x interface{}, cond table.ISqlBuilder, size, index int) ([]*{{.StructName}}, error) {
{{if .HasCache}}
	ids, e := p.IDs(x,cond,size,index)
	if len(ids) == 0 {
		log.Logs.Error(e)
		return nil, e
	}

	ms, e := {{lower .StructName}}_cache.Gets(context.TODO(), ids...)
	if e != nil {
		log.Logs.Error(e)
		return nil, e
	}
	list := make([]*{{.StructName}}, 0, len(ms))
	for _, m := range ms {
		if mm, ok := m.(*{{.StructName}}); ok {
			list = append(list, mm)
		}
	}
	return list, nil
{{else}}
	return p.FindNoCache(x,cond,size,index)
{{end}}
}

//FindNoCache 根据cond条件从数据库中获取数据列表
func (p *{{.StructName}}) FindNoCache(x interface{}, cond table.ISqlBuilder, size, index int, cols ...table.TableField) ([]*{{.StructName}}, error) {
	var (
		ok bool
		db *Session
	)
	
	if db, ok = x.(*Session); ok {
		db.Table(table.{{.StructName}}.TableName)
	} else {
		db = Db().Table(table.{{.StructName}}.TableName)
	}
	//
	if len(cols) > 0 {
		_cols := make([]string, 0, len(cols))
		for _, col := range cols {
			_cols = append(_cols, col.Name)
		}
		db.Cols(_cols...)
	}

	list := make([]*{{.StructName}}, 0)

	if cond != nil {
		if s, args := cond.GetWhere(); s != "" {
			db.Where(s, args...)
		}
		if s := cond.GetGroupBy(); s != "" {
			db.GroupBy(s)
		}
		if s := cond.GetHaving(); s != "" {
			db.Having(s)
		}
		if s := cond.GetOrderBy(); s != "" {
			db.OrderBy(s)
		}
	}

	if size > 0 {
		if index == 0 {
			index = 1
		}
		db.Limit(size, size*(index-1))
	}

	e := db.Find(&list)
	if e != nil {
		log.Logs.DBError(db, e)
	}
	return list, nil
}

{{if .HasCache}}
//OnChange
func (p *{{.StructName}}) OnChange(id uint64) error {
	return {{lower .StructName}}_cache.Remove(context.TODO(), id)
}

//OnBatchChange
func (p *{{.StructName}}) OnBatchChange(cond table.ISqlBuilder) {
	db := Db().Table(table.{{.StructName}}.TableName)
	if cond != nil {
		if s, args := cond.GetWhere(); s != "" {
			db.Where(s, args...)
		}
	}
	ids := make([]interface{}, 0)
	e := db.Find(&ids)
	if e != nil {
		log.Logs.DBError(db, e)
	}
	if len(ids) > 0 {
		{{lower .StructName}}_cache.Remove(context.TODO(), ids...)
	}
}
//OnListChange
func (p *{{.StructName}}) OnListChange() error {
	return {{lower .StructName}}_ids_cache.Empty(context.TODO())
}

func {{.StructName}}Cache() *redis.RedisBroker {
	return {{lower .StructName}}_cache
}

func {{.StructName}}IDsCache() *redis.RedisBroker {
	return {{lower .StructName}}_ids_cache
}
{{end}}
{{end}}

//ToMap struct转map
func (p *{{.StructName}}) ToMap(cols...table.TableField) map[string]interface{} {
	if len(cols) == 0{
		return map[string]interface{}{
			{{range $key, $value := .Columns}}table.{{$.StructName}}.{{$key}}.Name:p.{{$key}},
			{{end}}
		}
	}

	m := make(map[string]interface{},len(cols))
	for _, col := range cols {
		switch col.Name {
		{{range $key, $value := .Columns}}case table.{{$.StructName}}.{{$key}}.Name:
			m[col.Name] = p.{{$key}}
		{{end}}
		}
	}
	return m
}

//ToJSON struct转json
func (p *{{.StructName}}) ToJSON(cols...table.TableField) types.Smap {
	if len(cols) == 0{
		return types.Smap{
			{{range $key, $value := .Columns}}table.{{$.StructName}}.{{$key}}.Json:p.{{$key}},
			{{end}}
		}
	}

	m := make(types.Smap,len(cols))
	for _, col := range cols {
		switch col.Json {
		{{range $key, $value := .Columns}}case table.{{$.StructName}}.{{$key}}.Json:
			m[col.Json] = p.{{$key}}
		{{end}}
		}
	}
	return m
}

//SliceToJSON slice转json
func (p *{{.StructName}}) SliceToJSON(sls []*{{.StructName}},cols...table.TableField) []types.Smap {
	ms := make([]types.Smap, 0, len(sls))

	if len(cols) == 0 {
		for _, s := range sls {
			ms = append(ms,types.Smap{
				{{range $key, $value := .Columns}}table.{{$.StructName}}.{{$key}}.Json:s.{{$key}},
				{{end}}
			})
		}
		return ms
	}

	funs := make([]func(m types.Smap, s *{{.StructName}}), 0, len(cols))
	for _, col := range cols {
		switch col.Json {
		{{range $key, $value := .Columns}}case table.{{$.StructName}}.{{$key}}.Json:
			funs = append(funs, func(m types.Smap, s *{{$.StructName}}) {
				m[table.{{$.StructName}}.{{$key}}.Json] = s.{{$key}}
			})
		{{end}}
		}
	}
	return p.sliceToJSON(sls, funs)
}

func (p *{{.StructName}}) sliceToJSON(sls []*{{.StructName}}, funs []func(m types.Smap, s *{{.StructName}})) []types.Smap {
	ms := make([]types.Smap, 0, len(sls))
	for _, s := range sls {
		var m = types.Smap{}
		for _, f := range funs {
			f(m, s)
		}
		ms = append(ms, m)
	}
	return ms
}
	`

func (d *TempData) writeToModel(fileName string) error {
	var buf bytes.Buffer
	funcMap := template.FuncMap{
		"lower": strings.ToLower,
		"getTypeValue": func(t []string) interface{} {
			if len(t) < 3 {
				return `""`
			}
			var ret interface{}
			switch t[2] {
			case "string":
				ret = `""`
			case "uint", "uint8", "uint16", "uint32", "uint64", "int", "int8", "int16", "int32", "int64", "float32", "float64":
				ret = 0
			case "time.Time":
				ret = `time.Time{}`
			case "bool":
				ret = `false`
			default:
				ret = 0
			}
			return ret
		},
	}

	err := template.Must(template.New("tableTpl").Funcs(funcMap).Parse(model_str)).Execute(&buf, d)
	if err != nil {
		showError(err)
		return err
	}

	absPath, _ := filepath.Abs(fileName)
	fileName = filepath.Join(filepath.Dir(absPath), "zzz_"+d.StructName+".go")

	////文件已存在
	//_, e := os.Stat(fileName)
	//if e == nil {
	//	return nil
	//}
	var (
		f *os.File
	)

	f, err = os.Create(fileName)

	if err != nil {
		showError(err.Error())
		return err
	}
	defer f.Close()

	_, err = f.Write(buf.Bytes())
	if err != nil {
		showError(err)
		return err
	}

	return nil
}
