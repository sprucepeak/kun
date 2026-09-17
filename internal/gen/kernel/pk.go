package kernel

// findPrimaryKey E8: 从已解析的 fields 中查找主键信息，被 sqlGenerator 和 sqlParser 共用。
//
// 查找顺序：
//  1. 优先使用 IsPrimaryKey == true 的字段
//  2. 回退到列名为 "id"（大小写不敏感）或 Go 字段名为 "Id" 的字段
//
// 返回值均为零值时表示未找到主键。
func findPrimaryKey(fields []*Field) (name, column, typ string, autoIncr, found bool) {
	// 1. 优先 IsPrimaryKey 标记
	for _, f := range fields {
		if f.IsPrimaryKey {
			return f.Name, f.ColumnName, f.Type,
				containsStr(f.GORMTag, "autoIncrement:true"), true
		}
	}
	// 2. 回退：列名 "id" 或 Go 字段名 "Id"
	for _, f := range fields {
		if eqFold(f.ColumnName, "id") || f.Name == "Id" {
			return f.Name, f.ColumnName, f.Type,
				containsStr(f.GORMTag, "autoIncrement:true"), true
		}
	}
	return "", "", "", false, false
}

// hasSoftDeleteField 检测 fields 中是否包含软删除字段（如 deleted_at、delete_at 或 DeletedAt）
func hasSoftDeleteField(fields []*Field) bool {
	for _, f := range fields {
		if eqFold(f.ColumnName, "deleted_at") || eqFold(f.ColumnName, "delete_at") || eqFold(f.Name, "DeletedAt") {
			return true
		}
	}
	return false
}

func containsStr(s, sub string) bool {
	return len(sub) > 0 && len(s) >= len(sub) && indexOf(s, sub) >= 0
}

func indexOf(s, sub string) int {
	if len(sub) == 0 {
		return 0
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func eqFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}
