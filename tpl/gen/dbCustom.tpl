package {{.PackageName}}

import (
	"context"
)

//go:generate mockgen -source=./{{.InterfaceName}}.go -destination=../../../test/mocks/repository/db/{{.InterfaceName}}.go -package mock_repo_db -aux_files {{.PackageName}}=./{{.InterfaceName}}_gen.go

var _ {{.StructName}}Db = (*custom{{.StructName}}Db)(nil)

type (
	{{.StructName}}Db interface {
		{{.InterfaceName}}Db

        ListWithTotal(ctx context.Context, args *{{.StructName}}Search) ([]*{{.StructName}}, int64, error)
		ListWithMore(ctx context.Context, args *{{.StructName}}Search) ([]*{{.StructName}}, bool, error)

    	// TODO: add custom functions here and delete this line
	}

	custom{{.StructName}}Db struct {
		*default{{.StructName}}Db
	}

    {{.StructName}}Search struct {
		SearchPage
	}

	// TODO: add struct here and delete this line
)

func New{{.StructName}}Db(c *Conn) {{.StructName}}Db {
	return &custom{{.StructName}}Db{
		default{{.StructName}}Db: new{{.StructName}}Db(c),
	}
}


func (c *custom{{.StructName}}Db) buildListFilter(ctx context.Context, args *{{.StructName}}Search) *DB {
	d := c.WithContext(ctx).Model(c.model)
	// 自定义条件处理
	// TODO: add filter conditions here and delete this line

	return d
}

func (c *custom{{.StructName}}Db) buildListQuery(ctx context.Context, args *{{.StructName}}Search, limit int) *DB {
	filter := c.buildListFilter(ctx, args)
	order := c.HandleRank(
		args.OrderField,
		args.OrderType,
		{{.StructName}}Fields,
		{{if .HasPrimaryKey}}Table{{.StructName}}+".{{.PrimaryKeyColumn}}"{{else}}""{{end}},
	)

	offset, _ := c.HandlePage(args.Page, args.PageSize)

	{{if .HasPrimaryKey -}}
	switch {
	case args.LastId > 0: // 游标分页
		// 游标按主键 {{.PrimaryKeyColumn}} 比较,因此排序也必须按 {{.PrimaryKeyColumn}},否则(如按其它列排序而按主键取游标)
		// 翻页结果会重复或漏行。需要按其它列做游标分页时,应改为 (orderCol, {{.PrimaryKeyColumn}}) 复合游标。
		cursorOrder := Table{{.StructName}} + ".{{.PrimaryKeyColumn}} DESC"
		lastCond := Table{{.StructName}} + ".{{.PrimaryKeyColumn}} < ?"
		if args.OrderType == 1 {
			cursorOrder = Table{{.StructName}} + ".{{.PrimaryKeyColumn}} ASC"
			lastCond = Table{{.StructName}} + ".{{.PrimaryKeyColumn}} > ?"
		}
		return filter.Where(lastCond, args.LastId).Order(cursorOrder).Limit(limit)
	case offset > 100000: // 深分页: SELECT * FROM {{.TableName}} INNER JOIN (SELECT {{.PrimaryKeyColumn}} FROM {{.TableName}} WHERE ... ORDER BY {{.PrimaryKeyColumn}} LIMIT ?,?) AS tmp USING({{.PrimaryKeyColumn}})
		subQuery := filter.Select("{{.PrimaryKeyColumn}}").Offset(offset).Limit(limit)
		if order != "" {
			subQuery = subQuery.Order(order)
		}
		engine := c.WithContext(ctx).Model(c.model)
		if order != "" {
			engine = engine.Order(order)
		}
		return engine.Joins("INNER JOIN (?) AS tmp USING({{.PrimaryKeyColumn}})", subQuery)
	default:
		if order != "" {
			filter = filter.Order(order)
		}
		return filter.Offset(offset).Limit(limit)
	}
	{{- else -}}
	if order != "" {
		filter = filter.Order(order)
	}
	return filter.Offset(offset).Limit(limit)
	{{- end}}
}

func (c *custom{{.StructName}}Db) ListWithTotal(ctx context.Context, args *{{.StructName}}Search) ([]*{{.StructName}}, int64, error) {
	var total int64
	if err := c.buildListFilter(ctx, args).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return nil, 0, nil
	}

	_, limit := c.HandlePage(args.Page, args.PageSize)
	model := c.buildListQuery(ctx, args, limit)

	result := make([]*{{.StructName}}, 0, limit)
	if err := model.Find(&result).Error; err != nil {
		return nil, 0, err
	}
	return result, total, nil
}

func (c *custom{{.StructName}}Db) ListWithMore(ctx context.Context, args *{{.StructName}}Search) ([]*{{.StructName}}, bool, error) {
	_, limit := c.HandlePage(args.Page, args.PageSize)
	want := limit
	limit++

	model := c.buildListQuery(ctx, args, limit)

	result := make([]*{{.StructName}}, 0, limit)
	if err := model.Find(&result).Error; err != nil {
		return nil, false, err
	}

	ln := len(result)
	if ln == 0 {
		return nil, false, nil
	}
	var hasMore bool
	if ln > want {
		result = result[:want]
		hasMore = true
	}

	return result, hasMore, nil
}

// TODO: add your code here and delete this line

